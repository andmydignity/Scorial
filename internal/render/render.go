// Package render is about generating full html pages from templates and parsing markdown to html.
package render

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	paths "cms/internal"
)

// assets/templates

func RenderTemplates(base string, data any, tmpls []string) ([]byte, error) {
	tmpl, err := template.ParseFiles(tmpls...)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	err = tmpl.ExecuteTemplate(&buffer, base, data)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

type dataStruct struct {
	Title       string
	Content     template.HTML
	Style       string
	Script      string
	SiteName    string
	Year        int
	FaviconPath string
}

type homeDataStruct struct {
	Title       string
	Style       string
	Script      string
	SiteName    string
	Year        int
	Pages       []PageInfo
	LogoPath    string
	FaviconPath string
}

func SaveMdtoHTML(loadFrom, saveTo string, rndrConf *RenderConfig, db *sql.DB) error {
	page, title, err := parseMdToHTML(loadFrom)
	if err != nil {
		return err
	}
	overviewText := getOverviewText(117, page) + "..."
	overviewImg := overviewIMG(page)
	url, _ := strings.CutSuffix(filepath.Base(saveTo), ".html")
	url = "/pages/" + url
	fileName, _ := strings.CutSuffix(filepath.Base(loadFrom), ".md")
	entries, err := os.ReadDir(filepath.Join(paths.AssetsPath, "templates"))
	if err != nil {
		return err
	}
	templates := []string{}
	for _, e := range entries {
		_, has := strings.CutSuffix(e.Name(), ".tmpl")
		if has && !e.IsDir() {
			templates = append(templates, filepath.Join(paths.AssetsPath, "templates", e.Name()))
		}
	}
	_, err = os.Stat(filepath.Join(paths.AssetsPath, "sytle", fmt.Sprintf("%v.css", fileName)))
	customCSS := ""
	if err == nil {
		customCSS = fileName
	}
	_, err = os.Stat(filepath.Join(paths.AssetsPath, "sytle", fmt.Sprintf("%v.js", fileName)))
	customJS := ""
	if err == nil {
		customJS = fileName
	}

	data := dataStruct{title, template.HTML(page), customCSS, customJS, rndrConf.SiteName, time.Now().Year(), rndrConf.FaviconPath}
	// You pass base just by name, for some reason
	full, err := RenderTemplates("base.tmpl", &data, templates[:])
	if err != nil {
		return err
	}
	if _, found := strings.CutSuffix(saveTo, ".html"); !found {
		err = saveToFile(full, fmt.Sprintf("%v.html", saveTo))
	} else {
		err = saveToFile(full, saveTo)
	}
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"INSERT INTO pages (url, title, overview, overviewImg) VALUES (?, ?, ?, ?) ON CONFLICT(url) DO UPDATE SET title=excluded.title, overview=excluded.overview, overviewImg=excluded.overviewImg",
		url, title, overviewText, overviewImg,
	)
	if err != nil {
		return err
	}
	pages, err := getPages(25, db)
	if err != nil {
		return err
	}
	homeConf := homeDataStruct{title, fileName, fileName, rndrConf.SiteName, time.Now().Year(), pages, rndrConf.LogoPath, rndrConf.FaviconPath}
	return RenderHome(&homeConf)
}

func RenderHome(conf *homeDataStruct) error {
	entries, err := os.ReadDir(filepath.Join(paths.AssetsPath, "homePage", "templates"))
	if err != nil {
		return err
	}
	templates := []string{}
	for _, e := range entries {
		_, has := strings.CutSuffix(e.Name(), ".tmpl")
		if has && !e.IsDir() {
			templates = append(templates, filepath.Join(paths.AssetsPath, "homePage", "templates", e.Name()))
		}
	}
	home, err := RenderTemplates("base.tmpl", conf, templates)
	if err != nil {
		return err
	}
	return saveToFile(home, filepath.Join(paths.AssetsPath, "homePage", "home.html"))
}
