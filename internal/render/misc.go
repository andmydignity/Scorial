package render

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type PageInfo struct {
	URL          string
	ModifiedAt   string
	Title        string
	ImgPath      string
	OverviewText string
}

type RenderConfig struct {
	SiteName    string
	LogoPath    string
	FaviconPath string
}

func loadFromFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func saveToFile(data []byte, saveTo string) error {
	dir := filepath.Dir(saveTo)
	if err := os.MkdirAll(dir, 0o755); err != nil || err == os.ErrExist {
		return err
	}
	return os.WriteFile(saveTo, data, 0o644)
}

func getOverviewText(length int, page []byte) string {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return ""
	}
	var result strings.Builder
	var chars int
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "p" {
			text := getText(n)
			for _, r := range text {
				if chars >= length {
					break
				}
				result.WriteRune(r)
				chars++
			}
		}
		for c := n.FirstChild; c != nil && chars < length; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return result.String()
}

func getText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(getText(c))
	}
	return sb.String()
}

func overviewIMG(page []byte) string {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return ""
	}
	var imgSrc string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" && imgSrc == "" {
			for _, attr := range n.Attr {
				if attr.Key == "src" {
					imgSrc = attr.Val
					break
				}
			}
		}
		for c := n.FirstChild; c != nil && imgSrc == ""; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return imgSrc
}

func GetPages(numberOf int, db *sql.DB) ([]PageInfo, error) {
	pages := []PageInfo{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := db.QueryContext(ctx, "SELECT * FROM pages")
	if err != nil {
		return nil, nil
	}
	i := 0

	for res.Next() {
		if i >= numberOf {
			break
		}
		var page PageInfo
		err = res.Scan(&page.URL, &page.Title, &page.OverviewText, &page.ImgPath, &page.ModifiedAt)
		if err != nil {
			return nil, nil
		}
		page.ModifiedAt = strings.ReplaceAll(strings.ReplaceAll(page.ModifiedAt, "Z", ""), "T", " ")
		pages = append(pages, page)
	}
	return pages, nil
}
