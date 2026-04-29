package filesync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andmydignity/Scorial/internal/globals"

	_ "modernc.org/sqlite"
)

var ErrDidntExist = errors.New("didn't exist in the first place")

// dbName= name of the db INSIDE db folder, not the path

func deleteHTML(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func purgeOrphans(db *sql.DB, existingFiles []string, mdDir string) error {
	err, set := purgeOrphanedChecksums(db, existingFiles, mdDir)
	if err != nil {
		return err
	}
	postsDir := filepath.Join(globals.AssetsPath, "posts")
	err = purgeOrphanedHTMLs(postsDir, mdDir, set, db)
	if err != nil {
		return err
	}
	return purgeOrphanPosts(mdDir, db)
}

// Func purgeOrphanedChecksums purges orphaned checksum entries
func purgeOrphanedChecksums(db *sql.DB, fileNames []string, mdDir string) (error, *map[string]struct{}) {
	if mdDir == "" {
		return fmt.Errorf("mdDir cannot be empty"), nil
	}
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
		return err, nil
	}

	var file string
	toDelete := []string{}
	for rows.Next() {
		if err := rows.Scan(&file); err != nil {
			rows.Close()
			return err, nil
		}
		// O(1) lookup
		if _, ok := set[file]; !ok {
			toDelete = append(toDelete, file)
		}
	}
	rows.Close() // Explicitly close rows before executing delete queries

	if err := rows.Err(); err != nil {
		return err, nil
	}

	for _, file := range toDelete {
		if err := deleteChecksum(db, file); err != nil {
			return err, nil
		}
		mdDirAbs, err := filepath.Abs(mdDir)
		if err != nil {
			return err, nil
		}
		filename, _ := strings.CutPrefix(file, mdDirAbs)
		filename, _ = strings.CutSuffix(filename, ".md")
		if err = deleteFromPosts(filename, db); err != nil {
			return err, nil
		}
	}

	return err, &set
}

func purgeOrphanedHTMLs(postsDir, mdDir string, set *map[string]struct{}, db *sql.DB) error {
	err := filepath.WalkDir(postsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // assets/posts/ doesn't exist yet, nothing to purge
			}
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".html.br") {
			// Find the relative path of the HTML file
			relPath, err := filepath.Rel(postsDir, path)
			if err != nil {
				return err
			}

			// Reconstruct what the absolute path to the .md file SHOULD be
			mdRelPath := strings.TrimSuffix(relPath, ".html.br") + ".md"
			expectedMdPath := filepath.Join(mdDir, mdRelPath)

			// If the expected .md file is not in our active set, this HTML file is orphaned
			if _, ok := (*set)[expectedMdPath]; !ok {
				if err := os.Remove(path); err != nil {
					return err
				}
				filename, _ := strings.CutSuffix(mdRelPath, ".md")
				if err = deleteFromPosts(filename, db); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return err
}

func deleteFromPosts(path string, db *sql.DB) error {
	trim := strings.TrimSuffix(path, ".md")

	url := "/posts" + trim
	url = strings.ReplaceAll(url, " ", "%20")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, "DELETE FROM posts WHERE url = ?", url)
	return err
}

func purgeOrphanPosts(mdDir string, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := db.QueryContext(ctx, "SELECT url FROM posts")
	if err != nil {
		return err
	}
	defer res.Close()
	tbd := []string{}

	for res.Next() {
		var url string
		err = res.Scan(&url)
		if err != nil {
			return err
		}
		path, ok := strings.CutPrefix(url, "/posts/")
		if !ok {
			return fmt.Errorf("filesync/misc.go:163 Invalid URL")
		}
		path = filepath.FromSlash(path)
		path = filepath.Join(mdDir, path) + ".md"
		_, err = os.Stat(path)
		if err != nil {
			tbd = append(tbd, url)
		}
	}
	for _, url := range tbd {
		_, err = db.ExecContext(ctx, "DELETE FROM posts WHERE url = ?", url)
		if err != nil {
			return err
		}
	}
	return nil
}
