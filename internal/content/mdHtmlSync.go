package content

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
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
		if file == "." {
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
	err = purgeNonExistent(db, fullpaths)
	if err != nil {
		return err
	}
	return nil
}

func Sync(ctx context.Context, db *sql.DB, mdDir string, logger *slog.Logger) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Add existing directories
	err = filepath.WalkDir(mdDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if err := watcher.Add(path); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Main event loop (blocks until ctx cancelled)
	for {
		select {
		case <-ctx.Done():
			logger.Info("Sync shutting down")
			return nil

		case err := <-watcher.Errors:
			logger.Error("Watcher error", "error", err)

		case event := <-watcher.Events:
			if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {

				st, err := os.Stat(event.Name)
				if err == nil && st.IsDir() {
					// Add new directories dynamically
					if err := watcher.Add(event.Name); err != nil {
						logger.Error("Failed to add new dir", "error", err)
					}
					continue
				}

				fullpath := event.Name

				checksum, err := checksumCalculate(fullpath)
				if err != nil {
					logger.Error("Checksum error", "error", err)
					continue
				}

				if err := appendChecksum(db, fullpath, checksum); err != nil {
					logger.Error("DB error", "error", err)
					continue
				}

				suffixCut, _ := strings.CutSuffix(fullpath, ".md")
				extensionSanitized, _ := strings.CutPrefix(suffixCut, mdDir)

				if err := render.SaveMdtoHTML(
					fullpath,
					filepath.Join("assets", "pages", extensionSanitized),
				); err != nil {
					logger.Error("Render error", "error", err)
					continue
				}

			} else if event.Op&fsnotify.Remove != 0 {
				if err := deleteChecksum(db, event.Name); err != nil {
					logger.Error("Delete checksum error", "error", err)
				}
			}
		}
	}
}
