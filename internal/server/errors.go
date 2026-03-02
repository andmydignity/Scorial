package server

import (
	"fmt"
	"net/http"
)

func (cms *CmsStruct) internalError(w http.ResponseWriter, err error) {
	// recoverPanic passes nil since recover() returns an interface
	cms.Logger.Error(err.Error())
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprint(w, "Internal Server Error")
}

func (cms *CmsStruct) tooManyRequests(w http.ResponseWriter) {
	w.WriteHeader(http.StatusTooManyRequests)
	fmt.Fprint(w, "Too many requests.")
}

func (cms *CmsStruct) badRequest(w http.ResponseWriter, warn string) {
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, "Bad request: %v", warn)
}

func (cms *CmsStruct) notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, "Not found")
}
