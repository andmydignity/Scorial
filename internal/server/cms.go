// Package server contains everything related to HTTP
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	paths "cms/internal"
	"cms/internal/filesync"
	"cms/internal/render"

	"github.com/caddyserver/certmagic"
)

type CmsConfig struct {
	Port      int
	RateLimit struct {
		Rps   float64
		Burst int
	}
	HTTPSMode   bool
	Ratelimit   bool
	CertFile    string
	KeyFile     string
	MDDir       string
	SiteName    string
	LogoPath    string
	FaviconPath string
	Domains     []string
}

type CmsStruct struct {
	Logger *slog.Logger
	Config *CmsConfig
}

func (cms *CmsStruct) Start() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cms.Config.Port),
		Handler:      cms.routes(cms.Config.Ratelimit),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(cms.Logger.Handler(), slog.LevelError),
	}
	shutdownErr := make(chan error)
	checksumDB, err := filesync.OpenDB("database.db")
	if err != nil {
		cms.Logger.Error("Couldn't open checksum database.")
	}
	defer checksumDB.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rdr := &render.RenderConfig{cms.Config.SiteName, cms.Config.LogoPath, cms.Config.FaviconPath}
	err = filesync.FirstSync(cms.Config.MDDir, checksumDB, rdr)
	if err != nil {
		return err
	}
	go filesync.Sync(ctx, checksumDB, cms.Config.MDDir, cms.Logger, rdr)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
		s := <-quit
		cms.Logger.Info("Shutting down.", "signal", s.String())
		cancel()
		ctx, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel2()
		shutdownErr <- srv.Shutdown(ctx)
		checksumDB.Close()
	}()

	cms.Logger.Info(fmt.Sprintf("Starting server at port %d", cms.Config.Port))
	if cms.Config.HTTPSMode {
		if cms.Config.Domains != nil && (cms.Config.CertFile == "" || cms.Config.KeyFile == "") {
			certConf := certmagic.NewDefault()
			tlsConfig := certConf.TLSConfig()
			srv.TLSConfig = tlsConfig
			return srv.ListenAndServeTLS("", "")
		} else if cms.Config.KeyFile == "" || cms.Config.CertFile == "" {
			cms.Logger.Warn("HTTPS mode is on but no cert files are supplied. Using self-signed certs, which browsers will complain about.")
			_, _, err := certSetup()
			if err != nil {
				cms.Logger.Error("Error whilst generating self-signed certs", "error", err.Error())
				os.Exit(2)
			}
			cms.Config.CertFile = filepath.Join(paths.CertsPath, "selfsigned.pem")
			cms.Config.KeyFile = filepath.Join(paths.CertsPath, "selfsigned-key.pem")
		} else {
			_, doesKeyexist := os.Stat(cms.Config.KeyFile)
			_, doesCertExist := os.Stat(cms.Config.CertFile)
			if doesCertExist != nil || doesKeyexist != nil {
				cms.Logger.Warn(" Supplied cert/key files are inaccesible. Using self-signed certs, which browsers will complain about.")
				_, _, err := certSetup()
				if err != nil {
					cms.Logger.Error("Error whilst generating self-signed certs", "error", err.Error())
					os.Exit(2)
				}
				cms.Config.CertFile = filepath.Join(paths.CertsPath, "selfsigned.pem")
				cms.Config.KeyFile = filepath.Join(paths.CertsPath, "selfsigned-key.pem")
			}
		}
		return srv.ListenAndServeTLS(cms.Config.CertFile, cms.Config.KeyFile)
	} else {
		return srv.ListenAndServe()
	}
}
