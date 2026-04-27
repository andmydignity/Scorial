package render

import (
	"bytes"
	"database/sql"
	"path/filepath"
	"text/template"
	_ "text/template"
	"time"

	"cms/internal/globals"
)

func renderAtom(conf *RenderConfig, db *sql.DB) error {
	type FeedData struct {
		FeedTitle     string
		FeedSubtitle  string
		SiteURL       string
		AuthorName    string
		LastBuildDate string
		Pages         []PageInfo
	}
	pages, err := GetPages(2147483647, db)
	if err != nil {
		return err
	}
	feed := FeedData{conf.SiteName, conf.SiteDescription, conf.SiteURL, conf.SiteName, time.Now().Format(time.RFC3339), pages}

	ts, err := template.ParseFiles(filepath.Join(globals.AssetsPath, "atom", "atom.tmpl"))
	if err != nil {
		return err
	}
	var out bytes.Buffer
	err = ts.ExecuteTemplate(&out, "atom.tmpl", &feed)
	if err != nil {
		return err
	}
	zipped, err := brotliData(out.Bytes())
	if err != nil {
		return err
	}
	err = saveToFile(zipped, filepath.Join(globals.AssetsPath, "atom", "atom.xml.br"))
	if err != nil {
		return err
	}
	checksum, err := checksumCalculate(filepath.Join(globals.AssetsPath, "atom", "atom.xml.br"))
	if err != nil {
		return err
	}
	globals.AtomCache = zipped
	globals.AtomChecksumCache = checksum
	return nil
}
