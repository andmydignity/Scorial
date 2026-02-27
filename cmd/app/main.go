package main

import (
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"

	paths "cms/internal"
	"cms/internal/server"
)

func main() {
	paths.SetPaths()
	port := flag.Int("p", 8080, "Port to listen on")
	replenishment := flag.Float64("rps", 10.0, "Rate limit replenishment rate (+1 token every second 1/rps)")
	burst := flag.Int("burst", 20, "Rate limit max token per client.")
	https := flag.Bool("httpsOn", true, "Use HTTPS. (-httpsOn=false to disable)")
	certPath := flag.String("certPath", "", "Path to certificate file.")
	keyPath := flag.String("keyFile", "", "Path to key file")
	mdPath := flag.String("mdPath", "mdFiles", "Path to the folder containing md files")
	siteName := flag.String("siteName", "MySite", "Site name to display")
	flag.Parse()
	cmsConfig := server.CmsConfig{*port, struct {
		Rps   float64
		Burst int
	}{*replenishment, *burst}, *https, *certPath, *keyPath, *mdPath, *siteName}
	cms := server.CmsStruct{slog.New(slog.NewTextHandler(os.Stdout, nil)), &cmsConfig}
	err := cms.Start()
	if !errors.Is(err, http.ErrServerClosed) && err != nil {
		cms.Logger.Error("Error while closing the server.", "error", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
