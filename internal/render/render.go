// Package render is about generating full.html.br pages from templates and parsing markdown to html.
package render

import (
	"bytes"
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
	page, title, category, err := parseMdToHTML(loadFrom)
	if err != nil {
		if errors.Is(err, ErrIsDraft) {
			// Don't do anything for a draft files.
			return nil
		}
	}
	overviewText := getOverviewText(117, page) + "..."
	overviewImg := overviewIMG(page)
	rel, err := filepath.Rel(filepath.Join(globals.AssetsPath, "pages"), saveTo)
	if err != nil {
		return err
	}
	url, _ := strings.CutSuffix(rel, ".html.br")
	url = "/pages/" + url
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

	data := DataStruct{title, customCSS, customJS, template.HTML(page), rndrConf.SiteName, time.Now().Year(), rndrConf.FaviconPath, rndrConf.LogoPath}
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
	_, err = db.Exec(
		"INSERT INTO pages (url, title, overview, overviewImg, category) VALUES (?, ?, ?, ?, ?) ON CONFLICT(url) DO UPDATE SET title=excluded.title, overview=excluded.overview, overviewImg=excluded.overviewImg",
		url, title, overviewText, overviewImg, category,
	)
	if err != nil {
		return err
	}
	return RenderSpecials(&data, rndrConf.CardsInHomePage, db)
}

func RenderSpecials(conf *DataStruct, card int, db *sql.DB) error {
	return renderHome(conf, card, db)
}

func renderHome(conf *DataStruct, card int, db *sql.DB) error {
	type homeDataStruct struct {
		Title       string
		Style       string
		Script      string
		SiteName    string
		Year        int
		LatestPages []PageInfo
		AllPages    []PageInfo
		LogoPath    string
		FaviconPath string
	}
	latestPages, err := GetPages(card, db)
	if err != nil {
		return err
	}
	allPages, err := GetPages(2147483647, db)
	if err != nil {
		return err
	}
	homeData := homeDataStruct{conf.Title, conf.Style, conf.Script, conf.SiteName, time.Now().Year(), latestPages, allPages, conf.LogoPath, conf.FaviconPath}
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
