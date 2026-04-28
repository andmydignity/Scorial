package render

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andmydignity/Scorial/internal/globals"

	"github.com/andybalholm/brotli"
	"golang.org/x/net/html"
)

type PageInfo struct {
	URL          string
	ModifiedAt   string
	CreatedAt    string
	Title        string
	ImgPath      string
	OverviewText string
	Category     string
}

type RenderConfig struct {
	DB                    *sql.DB
	SiteName              string
	LogoPath              string
	FaviconPath           string
	SiteURL               string
	SiteDescription       string
	CardsInHomePage       int
	OverviewCharCount     int
	MDDir                 string
	PagesInAtomFeed       int
	MainContentInAtomFeed bool
}

func checksumCalculate(pathTo string) (string, error) {
	file, err := os.Open(pathTo)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	cheksum := hash.Sum(nil)
	return hex.EncodeToString(cheksum), nil
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

	query := "SELECT url, title, overview, overviewImg, category ,modifiedAt, createdAt FROM pages ORDER BY createdAt DESC LIMIT ?"
	res, err := db.QueryContext(ctx, query, numberOf)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	for res.Next() {
		var page PageInfo
		err = res.Scan(&page.URL, &page.Title, &page.OverviewText, &page.ImgPath, &page.Category, &page.ModifiedAt, &page.CreatedAt)
		if err != nil {
			return nil, err
		}

		// page.ModifiedAt = strings.ReplaceAll(strings.ReplaceAll(page.ModifiedAt, "Z", ""), "T", " ")
		// page.CreatedAt = strings.ReplaceAll(strings.ReplaceAll(page.CreatedAt, "Z", ""), "T", " ")

		pages = append(pages, page)
	}

	// Always check for errors that might have occurred during iteration
	if err = res.Err(); err != nil {
		return nil, err
	}

	return pages, nil
}

func brotliData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	bw := brotli.NewWriterLevel(&b, brotli.BestCompression)
	_, err := bw.Write(data)
	if err != nil {
		return nil, err
	}
	if err = bw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func brotliUncompress(data []byte) ([]byte, error) {
	br := brotli.NewReader(bytes.NewReader(data))
	return io.ReadAll(br)
}

func getCommonTemplates(ts *template.Template) error {
	entries, err := os.ReadDir(filepath.Join(globals.AssetsPath, "commonTemplates"))
	if err != nil {
		return err
	}
	templates := []string{}
	for _, e := range entries {
		_, has := strings.CutSuffix(e.Name(), ".tmpl")
		if has && !e.IsDir() {
			templates = append(templates, filepath.Join(globals.AssetsPath, "commonTemplates", e.Name()))
		}
	}
	_, err = ts.ParseFiles(templates...)
	return err
}
