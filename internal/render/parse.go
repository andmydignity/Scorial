package render

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/adrg/frontmatter"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	highlighting "github.com/yuin/goldmark-highlighting"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/toc"
)

var mdParser = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM, // Shorthand for Table, Strikethrough, TaskList, Linkify
		extension.Footnote,
		extension.DefinitionList,
		extension.Typographer,
		extension.CJK,
		meta.Meta,
		emoji.Emoji,
		highlighting.NewHighlighting(
			highlighting.WithGuessLanguage(true),
		),
		&toc.Extender{},
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
		parser.WithAttribute(),
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
		html.WithXHTML(),
	),
)

type ameta struct {
	Layout   string    `yaml:"layout"`
	Title    string    `yaml:"title"`
	Category string    `yaml:"category"`
	Tags     []string  `yaml:"tags"`
	Draft    bool      `yaml:"draft"`
	Date     time.Time `yaml:"date"`
}

type mdinfo struct {
	title    string
	category string
	date     string
}

var (
	ErrFaultyUTF8 = errors.New("file has invalid utf8")
	ErrIsDraft    = errors.New(".md file is a draft.")
)

func parseMdToHTML(loadFrom string) (data []byte, mdInfo mdinfo, err error) {
	raw, err := loadFromFile(loadFrom)
	if err != nil {
		return nil, mdinfo{}, err
	}

	m := ameta{}
	info := mdinfo{}
	body, err := frontmatter.Parse(bytes.NewReader(raw), &m)
	if err != nil {
		return nil, mdinfo{}, err
	}
	tempDate := time.Time{}
	if m.Date.IsZero() {
		stat, err := os.Stat(loadFrom)
		if err != nil {
			tempDate = time.Now()
		}
		tempDate = stat.ModTime()
	} else {
		tempDate = m.Date
	}
	info.date = tempDate.UTC().Format(time.RFC3339)
	if m.Draft {
		return nil, mdinfo{}, ErrIsDraft
	}
	var category string
	if m.Category == "" {
		if len(m.Tags) == 0 || m.Tags == nil {
			category = ""
		} else {
			category = m.Tags[0]
		}
	} else {
		category = m.Category
	}
	info.category = category
	var buf bytes.Buffer
	if err := mdParser.Convert(body, &buf); err != nil {
		return nil, mdinfo{}, err
	}
	rendered := buf.Bytes()

	if m.Title != "" {
		info.title = m.Title
		return rendered, info, nil
	}
	title, err := parseTitleFromMd(body)
	if errors.Is(err, ErrFaultyUTF8) {
		return nil, mdinfo{}, ErrFaultyUTF8
	}
	if err != nil {
		fileName, _ := strings.CutSuffix(filepath.Base(loadFrom), ".md")
		info.title = fileName
		return rendered, info, nil
	}
	info.title = title
	return rendered, info, nil
}

func parseTitleFromMd(data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", ErrFaultyUTF8
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[1:]), nil
		}
	}
	return "", fmt.Errorf("no title")
}
