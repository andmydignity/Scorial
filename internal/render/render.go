// Package render is about generating full.html.br pages from templates and parsing markdown to html.
package render

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cms/internal/globals"
)

// assets/templates

func RenderTemplates(base string, data any, tmpls []string) ([]byte, error) {
	tmpl, err := template.ParseFiles(tmpls...)
	if err != nil {
		return nil, err
	}
	getCommonTemplates(tmpl)
	var buffer bytes.Buffer
	err = tmpl.ExecuteTemplate(&buffer, base, data)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

type DataStruct struct {
	Title       string
	Style       string
	Script      string
	Content     template.HTML
	SiteName    string
	Year        int
	FaviconPath string
	LogoPath    string
}

func RenderNSave(loadFrom, saveTo string, rndrConf *RenderConfig, db *sql.DB) error {
	page, info, err := parseMdToHTML(loadFrom)
	if err != nil {
		if errors.Is(err, ErrIsDraft) {
			// Don't do anything for a draft files.
			return nil
		}
	}
	overviewText := getOverviewText(rndrConf.OverviewCharCount, page) + "..."
	overviewImg := overviewIMG(page)
	rel, err := filepath.Rel(filepath.Join(globals.AssetsPath, "pages"), saveTo)
	if err != nil {
		return err
	}
	URL, _ := strings.CutSuffix(rel, ".html.br")
	URL = "/pages/" + URL
	fileName, _ := strings.CutSuffix(filepath.Base(loadFrom), ".md")
	entries, err := os.ReadDir(filepath.Join(globals.AssetsPath, "templates"))
	if err != nil {
		return err
	}
	templates := []string{}
	for _, e := range entries {
		_, has := strings.CutSuffix(e.Name(), ".tmpl")
		if has && !e.IsDir() {
			templates = append(templates, filepath.Join(globals.AssetsPath, "templates", e.Name()))
		}
	}
	_, err = os.Stat(filepath.Join(globals.AssetsPath, "sytle", fmt.Sprintf("%v.css", fileName)))
	customCSS := ""
	if err == nil {
		customCSS = fileName
	}
	_, err = os.Stat(filepath.Join(globals.AssetsPath, "sytle", fmt.Sprintf("%v.js", fileName)))
	customJS := ""
	if err == nil {
		customJS = fileName
	}

	data := DataStruct{info.title, customCSS, customJS, template.HTML(page), rndrConf.SiteName, time.Now().Year(), rndrConf.FaviconPath, rndrConf.LogoPath}
	// You pass base just by name, for some reason
	full, err := RenderTemplates("base.tmpl", &data, templates[:])
	if err != nil {
		return err
	}
	zipped, err := brotliData(full)
	if err != nil {
		return err
	}
	if _, found := strings.CutSuffix(saveTo, ".html.br"); !found {
		err = saveToFile(zipped, fmt.Sprintf("%v.html.br", saveTo))
	} else {
		err = saveToFile(zipped, saveTo)
	}
	if err != nil {
		return err
	}
	URL = strings.ReplaceAll(URL, " ", "%20")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `
    INSERT INTO pages (url, title, overview, overviewImg, category, createdAt, modifiedAt) 
    VALUES (?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
    ON CONFLICT(url) DO UPDATE SET 
        title=excluded.title, 
        overview=excluded.overview, 
        overviewImg=excluded.overviewImg,
        category=excluded.category,
        modifiedAt=CURRENT_TIMESTAMP
`,
		URL, info.title, overviewText, overviewImg, info.category, info.date,
	)
	if err != nil {
		return err
	}
	return RenderSpecials(rndrConf, db)
}

func RenderSpecials(conf *RenderConfig, db *sql.DB) error {
	err := renderHome(conf, db)
	if err != nil {
		return err
	}
	return renderAtom(conf, db)
}

func renderHome(conf *RenderConfig, db *sql.DB) error {
	type homeDataStruct struct {
		Title string
		// Style       string
		// Script      string
		SiteName        string
		SiteDescription string
		Year            int
		LatestPages     []PageInfo
		AllPages        []PageInfo
		LogoPath        string
		FaviconPath     string
	}
	latestPages, err := GetPages(conf.CardsInHomePage, db)
	if err != nil {
		return err
	}
	allPages, err := GetPages(2147483647, db)
	if err != nil {
		return err
	}
	homeData := homeDataStruct{conf.SiteName, conf.SiteName, conf.SiteDescription, time.Now().Year(), latestPages, allPages, conf.LogoPath, conf.FaviconPath}
	entries, err := os.ReadDir(filepath.Join(globals.AssetsPath, "homePage"))
	if err != nil {
		return err
	}
	templates := []string{}
	for _, e := range entries {
		_, has := strings.CutSuffix(e.Name(), ".tmpl")
		if has && !e.IsDir() {
			templates = append(templates, filepath.Join(globals.AssetsPath, "homePage", e.Name()))
		}
	}

	home, err := RenderTemplates("base.tmpl", homeData, templates)
	if err != nil {
		return err
	}
	zipped, err := brotliData(home)
	if err != nil {
		return err
	}
	globals.HomePageCache = zipped
	err = saveToFile(zipped, filepath.Join(globals.AssetsPath, "homePage", "home.html.br"))
	if err != nil {
		return err
	}
	checksum, err := checksumCalculate(filepath.Join(globals.AssetsPath, "homePage", "home.html.br"))
	if err != nil {
		return err
	}
	globals.HomePageChecksumCache = checksum
	return nil
}
