// Package server contains everything related to HTTP
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cms/internal/sync"
)

type CmsConfig struct {
	Port      int
	RateLimit struct {
		Rps   float64
		Burst int
	}
	HTTPSMode bool
	CertFile  string
	KeyFile   string
	MDDir     string
}

type CmsStruct struct {
	Logger *slog.Logger
	Config *CmsConfig
}

func (cms *CmsStruct) Start() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cms.Config.Port),
		Handler:      cms.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(cms.Logger.Handler(), slog.LevelError),
	}
	shutdownErr := make(chan error)
	checksumDB, err := sync.OpenDB()
	if err != nil {
		cms.Logger.Error("Couldn't open checksum database.")
	}
	defer checksumDB.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = sync.FirstSync(cms.Config.MDDir, checksumDB)
	if err != nil {
		return err
	}
	go sync.Sync(ctx, checksumDB, cms.Config.MDDir, cms.Logger)
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
		if cms.Config.KeyFile == "" || cms.Config.CertFile == "" {
			cms.Logger.Warn("HTTPS mode is on but no cert files are supplied. Using self-signed certs, which browsers will complain about.")
			_, _, err := certSetup()
			if err != nil {
				cms.Logger.Error("Error whilst generating self-signed certs", "error", err.Error())
				os.Exit(2)
			}
			cms.Config.CertFile = "certs/selfsigned.pem"
			cms.Config.KeyFile = "certs/selfsigned-key.pem"
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
				cms.Config.CertFile = "certs/selfsigned.pem"
				cms.Config.KeyFile = "certs/selfsigned-key.pem"

			}
		}
		return srv.ListenAndServeTLS(cms.Config.CertFile, cms.Config.KeyFile)
	} else {
		return srv.ListenAndServe()
	}
}
