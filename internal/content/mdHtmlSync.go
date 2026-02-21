package content

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var ErrDidntExist = errors.New("didn't exist in the first place")

func openDB(mdDir string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", filepath.Join(mdDir, "checksum.db"))
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS checksums (filename TEXT PRIMARY KEY,hash TEXT NOT NULL)`)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func checksumCalculate(pathTo string) (string, error) {
	file, err := os.Open(pathTo)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	cheksum := hash.Sum(nil)
	return hex.EncodeToString(cheksum), nil
}

func appendChecksum(db *sql.DB, mdFile, checksum string) error {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `
		INSERT INTO checksums (filename, hash)
		VALUES (?, ?)
		ON CONFLICT(filename) DO UPDATE SET hash = excluded.hash`, mdFile, checksum)
	return err
}

func compareChecksum(db *sql.DB, mdFile, checksum string) (bool, error) {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	row := db.QueryRowContext(ctx, "SELECT 1 FROM checksums WHERE filename=? AND hash=?", mdFile, checksum)
	var exist int
	err = row.Scan(&exist)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrDidntExist
		}
		return false, err
	}
	return true, nil
}

func deleteChecksum(db *sql.DB, mdFile string) error {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := db.ExecContext(ctx, "DELETE FROM checksums WHERE filename=?", mdFile)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); affected != 1 || err != nil {
		if err != nil {
			return err
		}
		return ErrDidntExist
	}
	return nil
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

// TODO:Add actually creating the files.
func Sync(mdDir string) error {
	db, err := openDB(mdDir)
	if err != nil {
		return err
	}
	defer db.Close()
	var entries []os.DirEntry
	err = filepath.WalkDir(mdDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		entries = append(entries, d)
		return nil
	})
	files := []string{}
	mutexFiles := sync.Mutex{}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
		}
		file := entry.Name()
		if file == "checksum.db" {
			continue
		}
		checksum, err := checksumCalculate(filepath.Join(mdDir, file))
		if err != nil {
			return err
		}
		err = appendChecksum(db, file, checksum)
		if err != nil {
			return err
		}
		files = append(files, file)
	}

	err = purgeNonExistent(db, files)
	if err != nil {
		return err
	}
	return nil
}
