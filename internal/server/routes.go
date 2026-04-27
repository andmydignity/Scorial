package server

import (
	"net/http"
	"path/filepath"

	"cms/internal/globals"

	"github.com/julienschmidt/httprouter"
)

func (cms *CmsStruct) routes(ratelimitMode bool) http.Handler {
	router := httprouter.New()
	router.GET("/", cms.headersMiddleware(cms.homeHandler))
	router.GET("/pages/*name", cms.headersMiddleware(cms.pageHandler))
	router.GET("/atom.xml", cms.headersMiddleware(cms.atomHandler))
	router.ServeFiles("/assets/style/*filepath", http.Dir(filepath.Join(globals.AssetsPath, "style")))
	router.ServeFiles("/assets/media/*filepath", http.Dir(filepath.Join(globals.AssetsPath, "media")))
	if !ratelimitMode {
		return cms.uncaughtErrorMiddleware(router)
	}
	return cms.uncaughtErrorMiddleware(cms.rateLimitMiddleware(router))
}
