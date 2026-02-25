package render

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/adrg/frontmatter"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

var extensions = parser.CommonExtensions | parser.AutoHeadingIDs | parser.Footnotes | parser.SuperSubscript | parser.NoEmptyLineBeforeBlock | parser.DefinitionLists

type meta struct {
	layout   string `yaml:"layout"`
	title    string `yaml:"title"`
	category string `yaml:"category"`
}

var ErrFaultyUTF8 = errors.New("file has invalid utf8")

func parseMdToHTML(loadFrom string) ([]byte, string, error) {
	md, err := loadFromFile(loadFrom)
	if err != nil {
		return nil, "", err
	}
	m := meta{}
	body, err := frontmatter.Parse(bytes.NewReader(md), &m)
	if err != nil {
		return nil, "", err
	}
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(body)
	htmlFlags := html.CommonFlags | html.HrefTargetBlank | html.LazyLoadImages | html.TOC | html.FootnoteReturnLinks
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)
	rendered := markdown.Render(doc, renderer)
	if m.title != "" {
		return rendered, m.title, nil
	}
	title, err := parseTitleFromMd(body)
	if errors.Is(err, ErrFaultyUTF8) {
		return rendered, "", ErrFaultyUTF8
	}
	if err != nil {
		fileName, _ := strings.CutSuffix(filepath.Base(loadFrom), ".md")
		return rendered, fileName, nil
	}
	return rendered, title, nil
}

func parseTitleFromMd(data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", ErrFaultyUTF8
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(line[1:]), nil
		}
	}
	return "", fmt.Errorf("no title")
}
