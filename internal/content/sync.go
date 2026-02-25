package content

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

// a
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
			appendChecksum(db, mdDirAbs, checksum)
		} else if err != nil {
			return err
		}
		if !same {
			err = appendChecksum(db, file, checksum)
			if err != nil {
				return err
			}
			extensionSanitized, _ := strings.CutSuffix(filepath.Base(file), ".md")
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
	logger.Info("Start dirs", "dirs", dirs)
	go watcher.Watch(ctx)
	for event := range watcher.Events() {
		logger.Info("Current dirs", "dirs", dirs)
		types := event.Types
		path := event.Path
		logger.Info(path, "types", event.Types)
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		if slices.Contains(types, fswatcher.EventRemove) {
			path := event.Path

			if _, found := strings.CutSuffix(path, ".md"); !found {
				if !slices.Contains(dirs, path) {
					continue
				}
				suffixCut, _ := strings.CutPrefix(path, absMdDir)
				targetDir := filepath.Join("assets", "pages", suffixCut)
				if err := os.RemoveAll(targetDir); err != nil {
					logger.Error("Error while deleting directory recursively.", "error", err.Error())
				}
				continue

			}

			suffixCut, _ := strings.CutSuffix(path, ".md")
			extensionSanitized, _ := strings.CutPrefix(suffixCut, absMdDir)
			if slices.Contains(dirs, path) {

				// Also removes directories.
				err = deleteHTML(filepath.Join("assets", "pages", extensionSanitized))
				if err != nil {
					logger.Error("Error while deleting directory.")
				}
				continue
			}
			err = deleteHTML(filepath.Join("assets", "pages", extensionSanitized+".html"))
			if err != nil {
				logger.Error("Couldn't delete HTML file!", "error", err.Error())
			}

		}
		if slices.Contains(types, fswatcher.EventRename) {
			path := event.Path

			if _, found := strings.CutSuffix(path, ".md"); !found {
				if !slices.Contains(dirs, path) {
					continue
				}
				suffixCut, _ := strings.CutPrefix(path, absMdDir)
				targetDir := filepath.Join("assets", "pages", suffixCut)
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
			// my brain is fried, this should work tho for now.
			FirstSync(mdDir, db)
		}

		if slices.Contains(types, fswatcher.EventCreate) || slices.Contains(types, fswatcher.EventMod) {
			path := event.Path

			st, err := os.Stat(path)
			if err != nil {
				logger.Error("Couldn't get os.Stat!", "error", err.Error())
				continue
			}
			if _, found := strings.CutSuffix(path, ".md"); !found {
				if st.IsDir() {
					if !slices.Contains(dirs, path) {
						dirs = append(dirs, path)
						extensionSanitized, _ := strings.CutPrefix(path, absMdDir)
						err = os.Mkdir(filepath.Join("assets", "pages", extensionSanitized), 0o644)
						if err != nil {
							logger.Error("Mkdir failed", "error", err.Error())
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
