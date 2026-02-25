// Package render is about generating full html pages from templates and parsing markdown to html.
package render

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
)

// assets/templates
var (
	templates     = [...]string{"base.tmpl", "footer.tmpl", "navbar.tmpl", "sidebar.tmpl"}
	templateFiles = []string{}
)

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
	Title   string
	Content template.HTML
	Style   string
	Script  string
}

func SaveMdtoHTML(loadFrom, saveTo string) error {
	page, title, err := parseMdToHTML(loadFrom)
	if err != nil {
		return err
	}
	fileName, _ := strings.CutSuffix(filepath.Base(loadFrom), ".md")
	data := dataStruct{title, template.HTML(page), fileName, fileName}
	if len(templateFiles) == 0 {
		for _, fileName := range templates {
			templateFiles = append(templateFiles, filepath.Join("assets", "templates", fileName))
		}
	}
	// You pass base just by name, for some reason
	full, err := RenderTemplates("base.tmpl", &data, templateFiles[:])
	if err != nil {
		return err
	}
	if _, found := strings.CutSuffix(saveTo, ".html"); !found {
		return saveToFile(full, fmt.Sprintf("%v.html", saveTo))
	}
	return saveToFile(full, saveTo)
}
