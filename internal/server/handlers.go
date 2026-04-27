package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"cms/internal/filesync"
	"cms/internal/globals"

	"github.com/julienschmidt/httprouter"
)

func (cms *CmsStruct) homeHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if len(globals.HomePageCache) != 0 {
		eTag := fmt.Sprintf(`"%s"`, globals.HomePageChecksumCache)
		if r.Header.Get("If-None-Match") == eTag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("ETag", eTag)
		w.Write(globals.HomePageCache)
	} else {
		home, err := os.ReadFile(filepath.Join(globals.AssetsPath, "homePage", "home.html.br"))
		if err != nil {
			cms.internalError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(home)
	}
}

func (cms *CmsStruct) atomHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if len(globals.AtomCache) != 0 {
		eTag := fmt.Sprintf(`"%s"`, globals.AtomChecksumCache)
		if r.Header.Get("If-None-Match") == eTag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Header().Set("ETag", eTag)
		w.Write(globals.AtomCache)
	} else {
		home, err := os.ReadFile(filepath.Join(globals.AssetsPath, "atom", "atom.xml.br"))
		if err != nil {
			cms.internalError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write(home)
	}
}

func (cms *CmsStruct) pageHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := strings.TrimPrefix(ps.ByName("name"), "/")
	if name == "" {
		cms.badRequest(w, "Page name empty.")
		return
	}
	path := filepath.Join(globals.AssetsPath, "pages", name+".html.br")
	if page := filesync.FromCache(path); page != nil {
		checksum := filesync.ChecksumFromCache(path)
		eTag := fmt.Sprintf(`"%s"`, checksum)
		if r.Header.Get("If-None-Match") == eTag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("ETag", eTag)
		w.Write(page)
		return
	}
	if _, err := os.Stat(path); err != nil {
		cms.notFound(w)
		return
	}
	_, err := filesync.AppendToCache(path)
	if err != nil {
		cms.internalError(w, err)
		return
	}
	if page := filesync.FromCache(path); page != nil {
		checksum := filesync.ChecksumFromCache(path)
		w.Header().Set("ETag", checksum)
		w.Header().Set("Content-Type", "text/html")
		w.Write(page)
		return
	}

	cms.internalError(w, fmt.Errorf("Page should have been added to the cache but it isn't? Error: %v", err))
}
