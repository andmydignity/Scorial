package filesync

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/andmydignity/Scorial/internal/globals"
	"github.com/andmydignity/Scorial/internal/render"

	"github.com/sgtdi/fswatcher"
	_ "modernc.org/sqlite"
)

func FirstSync(rndrConf *render.RenderConfig) error {
	db := rndrConf.DB
	mdDirAbs, err := filepath.Abs(rndrConf.MDDir)
	if err != nil {
		return err
	}

	var entries []string
	err = filepath.WalkDir(mdDirAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		entries = append(entries, path)
		return nil
	})
	if err != nil {
		return err
	}

	existingFiles := []string{}
	for _, file := range entries {
		if _, found := strings.CutSuffix(file, ".md"); !found {
			continue
		}
		checksum, err := checksumCalculate(file)
		if err != nil {
			return err
		}
		same, err := isChecksumSame(db, file, checksum)
		if err != nil {
			return err
		}
		if !same {
			err = appendChecksum(db, file, checksum)
			if err != nil {
				return err
			}
			prefixCut, _ := strings.CutPrefix(file, mdDirAbs)
			extensionSanitized, _ := strings.CutSuffix(prefixCut, ".md")
			err = render.RenderNSave(file, filepath.Join(globals.AssetsPath, "posts", extensionSanitized), rndrConf)
			if err != nil {
				// if errors.Is(err, render.ErrIsDraft) {
				// 	deleteFromPages(extensionSanitized, db)
				// }
				return err
			}
		}
		existingFiles = append(existingFiles, file)
	}
	err = purgeOrphans(db, existingFiles, mdDirAbs)
	if err != nil {
		return err
	}
	return render.RenderSpecials(rndrConf)
}

func Sync(ctx context.Context, logger *slog.Logger, rndrConf *render.RenderConfig) error {
	watcher, err := fswatcher.New(fswatcher.WithPath(rndrConf.MDDir), fswatcher.WithSeverity(fswatcher.SeverityInfo))
	if err != nil {
		return err
	}
	return processSync(ctx, watcher, logger, rndrConf)
}

func processSync(ctx context.Context, watcher fswatcher.Watcher, logger *slog.Logger, rndrConf *render.RenderConfig) error {
	defer watcher.Close()
	var dirs []string
	db := rndrConf.DB
	mdDir := rndrConf.MDDir
	absMdDir, err := filepath.Abs(mdDir)
	if err != nil {
		return err
	}
	err = filepath.WalkDir(mdDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		dirs = append(dirs, abs)
		if err := watcher.AddPath(abs); err != nil {
			logger.Error("Failed to add watcher to subdirectory", "error", err.Error())
		}
		return nil
	})
	if err != nil {
		return err
	}
	go watcher.Watch(ctx)
	var needsSpecialRender bool
	for event := range watcher.Events() {
		types := event.Types
		path := event.Path
		prefixCut, _ := strings.CutPrefix(path, absMdDir)

		// Handle deletions and original globals of renames
		if slices.Contains(types, fswatcher.EventRemove) ||
			(slices.Contains(types, fswatcher.EventRename) && !(slices.Contains(types, fswatcher.EventCreate) || slices.Contains(types, fswatcher.EventMod))) {
			// Time for some editors / Windows to actually create the file. (looking at you, vim)
			time.Sleep(50 * time.Millisecond)
			if _, err := os.Stat(path); err == nil {
				// It still exists, let the create/mod block handle the update
				types = append(types, fswatcher.EventMod)
			} else {
				if _, found := strings.CutSuffix(path, ".md"); !found {
					if !slices.Contains(dirs, path) {
						goto EvaluateSpecials
					}
					watcher.DropPath(path)
					dirs = slices.DeleteFunc(dirs, func(s string) bool {
						return s == path
					})

					targetDir := filepath.Join(globals.AssetsPath, "posts", prefixCut)
					if err := os.RemoveAll(targetDir); err != nil {
						logger.Error("Error while deleting directory recursively.", "error", err.Error())
					}
					goto EvaluateSpecials
				}
				err = deleteChecksum(db, path)
				if err != nil {
					logger.Error("Error while deleting .md file from checksums table.", "error", err.Error())
				}
				suffixCut, _ := strings.CutSuffix(path, ".md")
				extensionSanitized, _ := strings.CutPrefix(suffixCut, absMdDir)
				err = deleteHTML(filepath.Join(globals.AssetsPath, "posts", extensionSanitized+".html.br"))
				if err != nil {
					logger.Error("Couldn't delete HTML file!", "error", err.Error())
				}

				deleteFromCache(filepath.Join(globals.AssetsPath, "posts", extensionSanitized+".html.br"))
				// Standardize slashes for Windows so it macthes the DB
				deleteTerm := prefixCut
				if runtime.GOOS == "windows" {
					deleteTerm = strings.ReplaceAll(deleteTerm, "\\", "/")
				}
				//if slices.Contains(types, fswatcher.EventRename) {
				//	deleteTerm = extensionSanitized
				//}

				if err = deleteFromPosts(deleteTerm, db); err != nil {
					logger.Error("Couldn't delete orphaned page from 'posts' table!", "error", err.Error())
				}
				needsSpecialRender = true

				if err != nil {
					logger.Error("Couldn't render home after markdown deletion.", "error", err.Error())
				}
			}
		}

		// Handle new files/directories and modifications
		if slices.Contains(types, fswatcher.EventCreate) || slices.Contains(types, fswatcher.EventMod) {
			// Give Windows processes a moment to release the lock
			if runtime.GOOS == "windows" {
				time.Sleep(50 * time.Millisecond)
			}
			st, err := os.Stat(path)
			if err != nil {
				if !os.IsNotExist(err) {
					logger.Error("Couldn't get os.Stat!", "error", err.Error())
				}
				goto EvaluateSpecials
			}
			if _, found := strings.CutSuffix(path, ".md"); !found {
				if st.IsDir() {
					if !slices.Contains(dirs, path) {
						if slices.Contains(types, fswatcher.EventRename) || slices.Contains(types, fswatcher.EventCreate) {
							watcher.AddPath(path)
						}
						dirs = append(dirs, path)
						extensionSanitized, _ := strings.CutPrefix(path, absMdDir)
						err = os.MkdirAll(filepath.Join(globals.AssetsPath, "posts", extensionSanitized), 0o755)
						if err != nil {
							logger.Error("Mkdir failed", "error", err.Error())
						}

						// Walk only the newly created directory instead of calling FirstSync on everything
						filepath.WalkDir(path, func(walkPath string, d os.DirEntry, err error) error {
							if err != nil || d.IsDir() {
								return nil
							}
							if strings.HasSuffix(walkPath, ".md") {
								checksum, _ := checksumCalculate(walkPath)
								appendChecksum(db, walkPath, checksum)
								prefixCutWalk, _ := strings.CutPrefix(walkPath, absMdDir)
								extSanitizedWalk, _ := strings.CutSuffix(prefixCutWalk, ".md")
								render.RenderNSave(walkPath, filepath.Join(globals.AssetsPath, "posts", extSanitizedWalk), rndrConf)
								needsSpecialRender = true
							}
							return nil
						})
					}
				}
				goto EvaluateSpecials
			}

			if st.IsDir() {
				if !slices.Contains(dirs, path) {
					dirs = append(dirs, path)
				}
				goto EvaluateSpecials
			}

			checksum, err := checksumCalculate(path)
			if err != nil {
				logger.Error("Couldn't calculate checksum!", "error", err.Error())
			}
			same, err := isChecksumSame(db, path, checksum)
			if err != nil {
				logger.Error("Couldn't compare checksums!", "error", err.Error())
			}
			if same {
				goto EvaluateSpecials
			}

			err = appendChecksum(db, path, checksum)
			if err != nil {
				logger.Error("Couldn't append checksum!", "error", err.Error())
			}
			suffixCut, _ := strings.CutSuffix(path, ".md")
			extensionSanitized, _ := strings.CutPrefix(suffixCut, absMdDir)
			if err := render.RenderNSave(
				path,
				filepath.Join(globals.AssetsPath, "posts", extensionSanitized), rndrConf); err != nil {
				logger.Error("Render error", "error", err)
			}
			needsSpecialRender = true
		}
		// Had to use goto, please do forgive me and my family.
	EvaluateSpecials:
		if needsSpecialRender && len(watcher.Events()) == 0 {
			// Windows being Windows, doesn't reflect the file changes immediately. Fuck Windows.
			if runtime.GOOS == "windows" {
				time.Sleep(50 * time.Millisecond)
			}
			if err := render.RenderSpecials(rndrConf); err != nil {
				logger.Error("Couldn't render specials.", "error", err.Error())
			}
			needsSpecialRender = false
		}
	}
	return nil
}
