package main

import (
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	paths "cms/internal"
	"cms/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	paths.SetPaths()
	port := flag.Int("p", 0, "Port to listen on")
	replenishment := flag.Float64("rps", 10.0, "Rate limit replenishment rate (+1 token every second 1/rps)")
	burst := flag.Int("burst", 20, "Rate limit max token per client.")
	https := flag.Bool("httpsOn", true, "Use HTTPS. (-httpsOn=false to disable)")
	certPath := flag.String("certPath", "", "Path to certificate file.")
	keyPath := flag.String("keyFile", "", "Path to key file")
	mdPath := flag.String("mdPath", "", "Path to the folder containing md files")
	siteName := flag.String("siteName", "MySite", "Site name to display")
	flag.Parse()
	if *mdPath == "" || *port == 0 {
		// TODO: Add the remaining ones later on
		slog.Default().Info("mdPath and/or port is not supplied from flags, loading configuration from .env")
		err := godotenv.Load(filepath.Join(paths.BinaryPath, ".env"))
		if err != nil {
			slog.Default().Error("Error while accessing .env file", "error", err.Error())
			os.Exit(3)
		}
		*port, err = strconv.Atoi(os.Getenv("port"))
		if err != nil {
			slog.Default().Error("Invalid port value", "error", err.Error())
			os.Exit(4)
		}
		*mdPath = os.Getenv("mdPath")
	}

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
