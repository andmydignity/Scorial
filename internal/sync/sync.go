package sync

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"cms/internal/render"

	"github.com/sgtdi/fswatcher"
	_ "modernc.org/sqlite"
)

// TODO: When a folder is renamed, a new folder in pages should generate.

func FirstSync(mdDir string, db *sql.DB) error {
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

	fullpaths := []string{}
	for _, file := range entries {
		if _, found := strings.CutSuffix(file, ".md"); !found {
			continue
		}
		checksum, err := checksumCalculate(file)
		if err != nil {
			return err
		}
		same, err := compareChecksum(db, file, checksum)
		if errors.Is(err, ErrDidntExist) {
			appendChecksum(db, file, checksum)
			prefixCut, _ := strings.CutPrefix(file, mdDirAbs)
			extensionSanitized, _ := strings.CutSuffix(prefixCut, ".md")
			err = render.SaveMdtoHTML(file, filepath.Join("assets", "pages", extensionSanitized))
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if !same {
			err = appendChecksum(db, file, checksum)
			if err != nil {
				return err
			}
			prefixCut, _ := strings.CutPrefix(file, mdDirAbs)
			extensionSanitized, _ := strings.CutSuffix(prefixCut, ".md")
			err = render.SaveMdtoHTML(file, filepath.Join("assets", "pages", extensionSanitized))
			if err != nil {
				return err
			}
		}
		fullpaths = append(fullpaths, file)
	}
	err = purgeNonExistent(db, fullpaths, mdDirAbs)
	if err != nil {
		return err
	}
	return nil
}

// TODO: Don't use FirstSync in Sync, use custom written stuff instead. Don't add overhead.

// Sync implements a filewatcher to the mdDir.
func Sync(ctx context.Context, db *sql.DB, mdDir string, logger *slog.Logger) error {
	watcher, err := fswatcher.New(fswatcher.WithPath(mdDir), fswatcher.WithSeverity(fswatcher.SeverityInfo))
	if err != nil {
		return err
	}
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
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		path := event.Path
		if slices.Contains(types, fswatcher.EventRemove) {
			if _, found := strings.CutSuffix(path, ".md"); !found {
				if !slices.Contains(dirs, path) {
					continue
				}
				watcher.DropPath(path)
				dirs = slices.DeleteFunc(dirs, func(s string) bool {
					return s == path
				})
				prefixCut, _ := strings.CutPrefix(path, absMdDir)
				targetDir := filepath.Join("assets", "pages", prefixCut)
				if err := os.RemoveAll(targetDir); err != nil {
					logger.Error("Error while deleting directory recursively.", "error", err.Error())
				}
				continue

			}

			suffixCut, _ := strings.CutSuffix(path, ".md")
			extensionSanitized, _ := strings.CutPrefix(suffixCut, absMdDir)
			err = deleteHTML(filepath.Join("assets", "pages", extensionSanitized+".html"))
			if err != nil {
				logger.Error("Couldn't delete HTML file!", "error", err.Error())
			}

		}
		// Could apply De Morgen, but short circutting gets removed.
		if slices.Contains(types, fswatcher.EventRename) && !(slices.Contains(types, fswatcher.EventCreate) || slices.Contains(types, fswatcher.EventMod)) {

			if _, found := strings.CutSuffix(path, ".md"); !found {
				if !slices.Contains(dirs, path) {
					continue
				}
				prefixCut, _ := strings.CutPrefix(path, absMdDir)
				targetDir := filepath.Join("assets", "pages", prefixCut)
				if err := os.RemoveAll(targetDir); err != nil {
					logger.Error("Error while deleting directory recursively.", "error", err.Error())
				} else {
					err = FirstSync(mdDir, db)
					if err != nil {
						logger.Error("Sync->FirstSync error.", "error", err.Error())
					}
				}
				continue
			}
			suffixCut, _ := strings.CutSuffix(path, ".md")
			extensionSanitized, _ := strings.CutPrefix(suffixCut, absMdDir)
			err = deleteHTML(filepath.Join("assets", "pages", extensionSanitized+".html"))
			if err != nil {
				logger.Error("Couldn't delete HTML file!", "error", err.Error())
			}
			// my brain is fried, this should work tho for now.
			FirstSync(mdDir, db)
			if err != nil {
				logger.Error("Sync->FirstSync error.", "error", err.Error())
			}
		}

		if slices.Contains(types, fswatcher.EventCreate) || slices.Contains(types, fswatcher.EventMod) {
			st, err := os.Stat(path)
			if err != nil {
				logger.Error("Couldn't get os.Stat!", "error", err.Error())
				continue
			}
			if _, found := strings.CutSuffix(path, ".md"); !found {
				if st.IsDir() {
					if !slices.Contains(dirs, path) {
						if slices.Contains(types, fswatcher.EventRename) {
							watcher.AddPath(path)
						}
						dirs = append(dirs, path)
						extensionSanitized, _ := strings.CutPrefix(path, absMdDir)
						err = os.Mkdir(filepath.Join("assets", "pages", extensionSanitized), 0o755)
						if err != nil {
							logger.Error("Mkdir failed", "error", err.Error())
						}
						err = FirstSync(mdDir, db)
						if err != nil {
							logger.Error("Sync->FirstSync error.", "error", err.Error())
						}
					}
				}
				continue
			}

			if st.IsDir() {
				dirs = append(dirs, path)
				continue
			}
			checksum, err := checksumCalculate(path)
			if err != nil {
				logger.Error("Couldn't calculate checksum!", "error", err.Error())
			}
			err = appendChecksum(db, path, checksum)
			if err != nil {
				logger.Error("Couldn't append checksum!", "error", err.Error())
			}
			suffixCut, _ := strings.CutSuffix(path, ".md")
			extensionSanitized, _ := strings.CutPrefix(suffixCut, absMdDir)
			if err := render.SaveMdtoHTML(
				path,
				filepath.Join("assets", "pages", extensionSanitized),
			); err != nil {
				logger.Error("Render error", "error", err)
			}
		}
	}
	return nil
}
