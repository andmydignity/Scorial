package filesync

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"cms/internal/globals"
	"cms/internal/render"

	"github.com/sgtdi/fswatcher"
	_ "modernc.org/sqlite"
)

func FirstSync(mdDir string, db *sql.DB, rndrConf *render.RenderConfig) error {
	mdDirAbs, err := filepath.Abs(mdDir)
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
			err = render.RenderNSave(file, filepath.Join(globals.AssetsPath, "pages", extensionSanitized), rndrConf, db)
			if err != nil {
				return err
			}
		}
		existingFiles = append(existingFiles, file)
	}
	err = purgeOrphans(db, existingFiles, mdDirAbs)
	if err != nil {
		return err
	}
	return nil
}

func Sync(ctx context.Context, db *sql.DB, mdDir string, logger *slog.Logger, rndrConf *render.RenderConfig) error {
	watcher, err := fswatcher.New(fswatcher.WithPath(mdDir), fswatcher.WithSeverity(fswatcher.SeverityInfo))
	if err != nil {
		return err
	}
	return processSync(ctx, watcher, db, mdDir, logger, rndrConf)
}

func processSync(ctx context.Context, watcher fswatcher.Watcher, db *sql.DB, mdDir string, logger *slog.Logger, rndrConf *render.RenderConfig) error {
	defer watcher.Close()
	var dirs []string
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
		return nil
	})
	if err != nil {
		return err
	}
	go watcher.Watch(ctx)
	for event := range watcher.Events() {
		types := event.Types
		path := event.Path
		prefixCut, _ := strings.CutPrefix(path, absMdDir)

		// Handle deletions and original globals of renames
		if slices.Contains(types, fswatcher.EventRemove) ||
			(slices.Contains(types, fswatcher.EventRename) && !(slices.Contains(types, fswatcher.EventCreate) || slices.Contains(types, fswatcher.EventMod))) {

			if _, found := strings.CutSuffix(path, ".md"); !found {
				if !slices.Contains(dirs, path) {
					continue
				}
				watcher.DropPath(path)
				dirs = slices.DeleteFunc(dirs, func(s string) bool {
					return s == path
				})

				targetDir := filepath.Join(globals.AssetsPath, "pages", prefixCut)
				if err := os.RemoveAll(targetDir); err != nil {
					logger.Error("Error while deleting directory recursively.", "error", err.Error())
				}
				continue
			}
			err = deleteChecksum(db, path)
			if err != nil {
				logger.Error("Error while deleting .md file from checksums table.", "error", err.Error())
			}
			suffixCut, _ := strings.CutSuffix(path, ".md")
			extensionSanitized, _ := strings.CutPrefix(suffixCut, absMdDir)
			err = deleteHTML(filepath.Join(globals.AssetsPath, "pages", extensionSanitized+".html.br"))
			if err != nil {
				logger.Error("Couldn't delete HTML file!", "error", err.Error())
			}

			deleteFromCache(filepath.Join(globals.AssetsPath, "pages", extensionSanitized+".html.br"))
			// Use prefixCut for EventRemove and extensionSanitized for EventRename as per your original logic
			deleteTerm := prefixCut
			//if slices.Contains(types, fswatcher.EventRename) {
			//	deleteTerm = extensionSanitized
			//}

			if err = deleteFromPages(deleteTerm, db); err != nil {
				logger.Error("Couldn't delete orphaned page from 'pages' table!", "error", err.Error())
			}

			err = render.RenderSpecials(&render.DataStruct{rndrConf.SiteName, "", "", "", rndrConf.SiteName, time.Now().Year(), rndrConf.FaviconPath, rndrConf.LogoPath}, rndrConf.CardsInHomePage, db)
			if err != nil {
				logger.Error("Couldn't render home after markdown deletion.", "error", err.Error())
			}
		}

		// Handle new files/directories and modifications
		if slices.Contains(types, fswatcher.EventCreate) || slices.Contains(types, fswatcher.EventMod) {
			st, err := os.Stat(path)
			if err != nil {
				if !os.IsNotExist(err) {
					logger.Error("Couldn't get os.Stat!", "error", err.Error())
				}
				continue
			}
			if _, found := strings.CutSuffix(path, ".md"); !found {
				if st.IsDir() {
					if !slices.Contains(dirs, path) {
						if slices.Contains(types, fswatcher.EventRename) || slices.Contains(types, fswatcher.EventCreate) {
							watcher.AddPath(path)
						}
						dirs = append(dirs, path)
						extensionSanitized, _ := strings.CutPrefix(path, absMdDir)
						err = os.MkdirAll(filepath.Join(globals.AssetsPath, "pages", extensionSanitized), 0o755)
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
								render.RenderNSave(walkPath, filepath.Join(globals.AssetsPath, "pages", extSanitizedWalk), rndrConf, db)
							}
							return nil
						})
					}
				}
				continue
			}

			if st.IsDir() {
				if !slices.Contains(dirs, path) {
					dirs = append(dirs, path)
				}
				continue
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
				continue
			}

			err = appendChecksum(db, path, checksum)
			if err != nil {
				logger.Error("Couldn't append checksum!", "error", err.Error())
			}
			suffixCut, _ := strings.CutSuffix(path, ".md")
			extensionSanitized, _ := strings.CutPrefix(suffixCut, absMdDir)
			if err := render.RenderNSave(
				path,
				filepath.Join(globals.AssetsPath, "pages", extensionSanitized), rndrConf, db); err != nil {
				logger.Error("Render error", "error", err)
			}
		}
	}
	return nil
}
