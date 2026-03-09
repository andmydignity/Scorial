package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	paths "cms/internal"
	"cms/internal/server"

	"gopkg.in/yaml.v2"
)

type config struct {
	Port          int      `yaml:"port"`
	CertPath      string   `yaml:"certPath"`
	KeyPath       string   `yaml:"keyPath"`
	MdPath        string   `yaml:"mdPath"`
	SiteName      string   `yaml:"siteName"`
	LogoPath      string   `yaml:"logoPath"`
	FaviconPath   string   `yaml:"faviconPath"`
	HTTPSMode     bool     `yaml:"httpsMode"`
	Ratelimit     bool     `yaml:"ratelimit"`
	Replenishment float64  `yaml:"replenishment"`
	Burst         int      `yaml:"burst"`
	Domains       []string `yaml:"domains"`
}

func main() {
	paths.SetPaths()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	file, err := os.Open(filepath.Join(paths.BinaryPath, "config.yaml"))
	if err != nil {
		logger.Error("Couldn't access config.yaml file!", "error", err.Error())
		os.Exit(2)
	}
	var cfg config
	decoder := yaml.NewDecoder(file)
	if err = decoder.Decode(&cfg); err != nil {
		logger.Error("Error while parsing YAML config!", "error", err.Error())
		os.Exit(3)
	}

	cmsConfig := server.CmsConfig{cfg.Port, struct {
		Rps   float64
		Burst int
	}{cfg.Replenishment, cfg.Burst}, cfg.HTTPSMode, cfg.Ratelimit, cfg.CertPath, cfg.KeyPath, cfg.MdPath, cfg.SiteName, cfg.LogoPath, cfg.FaviconPath, cfg.Domains}
	cms := server.CmsStruct{logger, &cmsConfig}
	err = cms.Start()
	if !errors.Is(err, http.ErrServerClosed) && err != nil {
		cms.Logger.Error("Error while closing the server.", "error", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
