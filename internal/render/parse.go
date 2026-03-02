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
	Layout   string `yaml:"layout"`
	Title    string `yaml:"title"`
	Category string `yaml:"category"`
}

var ErrFaultyUTF8 = errors.New("file has invalid utf8")

func parseMdToHTML(loadFrom string) ([]byte, string, error) {
	raw, err := loadFromFile(loadFrom)
	if err != nil {
		return nil, "", err
	}

	m := ameta{}
	body, err := frontmatter.Parse(bytes.NewReader(raw), &m)
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	if err := mdParser.Convert(body, &buf); err != nil {
		return nil, "", err
	}
	rendered := buf.Bytes()

	if m.Title != "" {
		return rendered, m.Title, nil
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
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[1:]), nil
		}
	}
	return "", fmt.Errorf("no title")
}
