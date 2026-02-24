package content

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"cms/internal/render"

	"github.com/sgtdi/fswatcher"
	_ "modernc.org/sqlite"
)

func FirstSync(mdDir string, db *sql.DB) error {
	var entries []string
	err := filepath.WalkDir(mdDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(mdDir, path)
		if err != nil {
			return err
		}
		entries = append(entries, rel)
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
		fullpath := filepath.Join(mdDir, file)
		checksum, err := checksumCalculate(fullpath)
		if err != nil {
			return err
		}
		err = appendChecksum(db, fullpath, checksum)
		if err != nil {
			return err
		}

		extensionSanitized, _ := strings.CutSuffix(file, ".md")
		err = render.SaveMdtoHTML(fullpath, filepath.Join("assets", "pages", extensionSanitized))
		if err != nil {
			return err
		}
		fullpaths = append(fullpaths, fullpath)
	}
	err = purgeNonExistent(db, fullpaths, mdDir)
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
	go watcher.Watch(ctx)
	for event := range watcher.Events() {
		types := event.Types
		if slices.Contains(types, fswatcher.EventRemove) {
			path := event.Path
			if _, found := strings.CutSuffix(path, ".md"); !found {
				continue
			}
			path = filepath.Clean(path)
			parts := strings.Split(path, string(filepath.Separator))
			idx := -1
			for i, p := range parts {
				if p == mdDir {
					idx = i
					break
				}
			}
			if idx == -1 {
				// relativezation,damn...
				logger.Error("Error in the relativinization of the absolute path", "error", err.Error())
			}
			path = filepath.Join(parts[idx:]...)

			suffixCut, _ := strings.CutSuffix(path, ".md")
			extensionSanitized, _ := strings.CutPrefix(suffixCut, mdDir)
			err = deleteHTML(filepath.Join("assets", "pages", extensionSanitized+".html"))
			if err != nil {
				logger.Error("Couldn't delete HTML file!", "error", err.Error())
			}

		}
		if slices.Contains(types, fswatcher.EventRename) {
			path := event.Path
			if _, found := strings.CutSuffix(path, ".md"); !found {
				continue
			}

			path = filepath.Clean(path)
			parts := strings.Split(path, string(filepath.Separator))
			idx := -1
			for i, p := range parts {
				if p == mdDir {
					idx = i
					break
				}
			}
			if idx == -1 {
				logger.Error("Error in the relativezation of the absolute path", "error", err.Error())
			}
			path = filepath.Join(parts[idx:]...)
			suffixCut, _ := strings.CutSuffix(path, ".md")
			extensionSanitized, _ := strings.CutPrefix(suffixCut, mdDir)
			err = deleteHTML(filepath.Join("assets", "pages", extensionSanitized+".html"))
			if err != nil {
				logger.Error("Couldn't delete HTML file!", "error", err.Error())
			}
			// my brain is fried, this should work tho for now.
			FirstSync(mdDir, db)
		}

		if slices.Contains(types, fswatcher.EventCreate) || slices.Contains(types, fswatcher.EventMod) {
			path := event.Path
			if _, found := strings.CutSuffix(path, ".md"); !found {
				continue
			}
			path = filepath.Clean(path)
			parts := strings.Split(path, string(filepath.Separator))
			idx := -1
			for i, p := range parts {
				if p == mdDir {
					idx = i
					break
				}
			}
			if idx == -1 {
				logger.Error("Error in the relativezation of the absolute path", "error", err.Error())
			}
			path = filepath.Join(parts[idx:]...)
			st, err := os.Stat(path)
			if err != nil {
				logger.Error("File error", "error", err.Error())
			}
			if st.IsDir() {
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
			extensionSanitized, _ := strings.CutPrefix(suffixCut, mdDir)
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
