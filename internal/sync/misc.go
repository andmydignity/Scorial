// Package content
package sync

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	paths "cms/internal"
)

var dbLocation = paths.DBPath

var ErrDidntExist = errors.New("didn't exist in the first place")

// dbName= name of the db INSIDE db folder, not the path
func OpenDB(dbName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", filepath.Join(dbLocation, dbName))
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS checksums (filename TEXT PRIMARY KEY,hash TEXT NOT NULL)`)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func deleteHTML(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func purgeNonExistent(db *sql.DB, fileNames []string, mdDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build O(1) lookup set for existing markdown files
	set := make(map[string]struct{}, len(fileNames))
	for _, f := range fileNames {
		set[f] = struct{}{}
	}

	// 1. Purge deleted files from the Database
	rows, err := db.QueryContext(ctx, "SELECT filename FROM checksums")
	if err != nil {
		return err
	}

	var file string
	toDelete := []string{}
	for rows.Next() {
		if err := rows.Scan(&file); err != nil {
			rows.Close()
			return err
		}
		// O(1) lookup
		if _, ok := set[file]; !ok {
			toDelete = append(toDelete, file)
		}
	}
	rows.Close() // Explicitly close rows before executing delete queries

	if err := rows.Err(); err != nil {
		return err
	}

	for _, file := range toDelete {
		if err := deleteChecksum(db, file); err != nil {
			return err
		}
	}

	// 2. Purge orphaned HTML files from assets/pages/
	pagesDir := filepath.Join(paths.AssetsPath, "pages")
	err = filepath.WalkDir(pagesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // assets/pages/ doesn't exist yet, nothing to purge
			}
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".html") {
			// Find the relative path of the HTML file
			relPath, err := filepath.Rel(pagesDir, path)
			if err != nil {
				return err
			}

			// Reconstruct what the absolute path to the .md file SHOULD be
			mdRelPath := strings.TrimSuffix(relPath, ".html") + ".md"
			expectedMdPath := filepath.Join(mdDir, mdRelPath)

			// If the expected .md file is not in our active set, this HTML file is orphaned
			if _, ok := set[expectedMdPath]; !ok {
				if err := os.Remove(path); err != nil {
					return err
				}
			}
		}
		return nil
	})

	return err
}
