package content

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"cms/internal/render"

	"github.com/fsnotify/fsnotify"
	_ "modernc.org/sqlite"
)

const dbLocation = "databases"

var ErrDidntExist = errors.New("didn't exist in the first place")

func OpenDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", filepath.Join(dbLocation, "checksum.db"))
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS checksums (filename TEXT PRIMARY KEY,hash TEXT NOT NULL)`)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func purgeNonExistent(db *sql.DB, fileNames []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build O(1) lookup set
	set := make(map[string]struct{}, len(fileNames))
	for _, f := range fileNames {
		set[f] = struct{}{}
	}

	rows, err := db.QueryContext(ctx, "SELECT filename FROM checksums")
	if err != nil {
		return err
	}
	defer rows.Close()

	var file string
	for rows.Next() {
		if err := rows.Scan(&file); err != nil {
			return err
		}

		// O(1) lookup instead of slices.Contains (O(n))
		if _, ok := set[file]; !ok {
			if err := deleteChecksum(db, file); err != nil {
				return err
			}
			// TODO: Implement HTML file deletion.
		}
	}

	return rows.Err()
}

func FirstSync(mdDir string, db *sql.DB) error {
	var entries []os.DirEntry
	err := filepath.WalkDir(mdDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		entries = append(entries, d)
		return nil
	})
	files := []string{}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Shouldn't be possible but still check.
		if entry.IsDir() {
			continue
		}
		file := entry.Name()

		checksum, err := checksumCalculate(filepath.Join(mdDir, file))
		if err != nil {
			return err
		}
		err = appendChecksum(db, file, checksum)
		if err != nil {
			return err
		}
		files = append(files, file)
		fullpath := filepath.Join(mdDir, file)
		extensionSanitized, _ := strings.CutSuffix(file, ".md")
		err = render.SaveMdtoHTML(fullpath, filepath.Join("assets", "pages", extensionSanitized))
		if err != nil {
			return err
		}
	}

	err = purgeNonExistent(db, files)
	if err != nil {
		return err
	}
	return nil
}

// TODO: Since this is called as a go routine, i cannot get errors as function returns. Slapped some loggers for now, but gotta find a better way.
func Sync(db *sql.DB, mdDir string, logger *slog.Logger) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error("Sync go func error", "error", err)
		return err
	}
	defer watcher.Close()

	go func() {
		for event := range watcher.Events {
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				st, err := os.Stat(event.Name)
				if err == nil {
					if st.IsDir() && !slices.Contains(watcher.WatchList(), event.Name) {
						watcher.Add(event.Name)
					}
					continue
				}

				checksum, err := checksumCalculate(event.Name)
				if err != nil {
					logger.Error("Sync go func error", "error", err)
					return
				}
				if err := appendChecksum(db, event.Name, checksum); err != nil {
					logger.Error("Sync go func error", "error", err)
					return
				}
				extensionSanitized, _ := strings.CutSuffix(event.Name, ".md")
				if err := render.SaveMdtoHTML(event.Name, filepath.Join("assets", "pages", extensionSanitized)); err != nil {
					logger.Error("Sync go func error", "error", err)
					return
				}
			} else if event.Has(fsnotify.Remove) {
				if err := deleteChecksum(db, event.Name); err != nil {
					logger.Error("Sync go func error", "error", err)
					return
				}
				// TODO:Implement HTML file deletion
			}
		}
	}()

	if err := watcher.Add(mdDir); err != nil {
		logger.Error("Sync go func error", "error", err)
		return err
	}
	err = filepath.WalkDir(mdDir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		logger.Error("Sync go func error", "error", err)
		return err
	}
	return err
}
