package render

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/andmydignity/Scorial/internal/globals"
)

func renderAtom(conf *RenderConfig) error {
	db := conf.DB
	type PostInfoWithContent struct {
		URL          string
		ModifiedAt   string
		CreatedAt    string
		Title        string
		ImgPath      string
		OverviewText string
		Category     string
		Content      string
	}
	type FeedData struct {
		FeedTitle     string
		FeedSubtitle  string
		SiteURL       string
		AuthorName    string
		LastBuildDate string
		Pages         []PostInfoWithContent
	}
	contentPages, err := GetPosts(conf.PagesInAtomFeed, db)
	var pages []PostInfoWithContent
	if conf.MainContentInAtomFeed {
		for _, x := range contentPages {
			pagePath := filepath.Join(globals.AssetsPath, "posts", strings.TrimPrefix(strings.ReplaceAll(x.URL, "%20", " "), "/posts")+".html.br")
			raw, err := loadFromFile(pagePath)
			if err != nil {
				return err
			}
			unzipped, err := brotliUncompress(raw)
			if err != nil {
				return err
			}
			htmlStr := string(unzipped)

			// 1. Extract <style> tags
			styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
			styles := strings.Join(styleRe.FindAllString(htmlStr, -1), "\n")

			// 2. Extract contents of <main id="main-content">
			contentRe := regexp.MustCompile(`(?is)<main[^>]*>(.*?)</main>`)
			match := contentRe.FindStringSubmatch(htmlStr)
			mainContent := ""
			if len(match) > 1 {
				mainContent = match[1]
			} else {
				// Fallback to the whole string if the tag is not found
				mainContent = htmlStr
			}

			// 3. Combine and remove any literal "]]>" that breaks CDATA
			finalContent := styles + "\n" + mainContent
			finalContent = strings.ReplaceAll(finalContent, "]]>", "") // Strip CDATA closures
			wContent := PostInfoWithContent{x.URL, x.ModifiedAt, x.CreatedAt, x.Title, x.ImgPath, x.OverviewText, x.Category, finalContent}
			pages = append(pages, wContent)
		}
	} else {
		for _, x := range contentPages {
			pages = append(pages, PostInfoWithContent{x.URL, x.ModifiedAt, x.CreatedAt, x.Title, x.ImgPath, x.OverviewText, x.Category, ""})
		}
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
