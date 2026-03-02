package server

import (
	"net/http"
	"path/filepath"

	paths "cms/internal"

	"github.com/julienschmidt/httprouter"
)

func (cms *CmsStruct) routes() http.Handler {
	router := httprouter.New()
	router.GET("/", cms.homeHandler)
	router.GET("/pages/*name", cms.pageHandler)
	router.ServeFiles("/assets/style/*filepath", http.Dir(filepath.Join(paths.AssetsPath, "style")))
	router.ServeFiles("/assets/media/*filepath", http.Dir(filepath.Join(paths.AssetsPath, "media")))

	return cms.uncaughtErrorMiddleware(cms.rateLimitMiddleware(router))
}
