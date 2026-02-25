// Package content
package content

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	toDelete := []string{}
	// When the rows.Next is open, you cannot write to DB
	for rows.Next() {

		if err := rows.Scan(&file); err != nil {
			return err
		}

		// O(1) lookup instead of slices.Contains (O(n))
		if _, ok := set[file]; !ok {
			toDelete = append(toDelete, file)
		}
	}
	for _, file := range toDelete {
		if err := deleteChecksum(db, file); err != nil {
			return err
		}

		suffixCut, _ := strings.CutSuffix(file, ".md")
		extensionSanitized, _ := strings.CutPrefix(suffixCut, mdDir)
		err = deleteHTML(filepath.Join("assets", "pages", extensionSanitized+".html"))
		if err != nil {
			return err
		}
	}

	return rows.Err()
}
