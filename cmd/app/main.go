package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/andmydignity/Scorial/internal/globals"
	"github.com/andmydignity/Scorial/internal/server"

	"gopkg.in/yaml.v2"
)

type config struct {
	Port                  int      `yaml:"port"`
	CertPath              string   `yaml:"certPath"`
	KeyPath               string   `yaml:"keyPath"`
	CardsInHome           int      `yaml:"cardsInHome"`
	MdPath                string   `yaml:"mdPath"`
	SiteName              string   `yaml:"siteName"`
	LogoPath              string   `yaml:"logoPath"`
	FaviconPath           string   `yaml:"faviconPath"`
	HTTPSMode             bool     `yaml:"httpsMode"`
	Ratelimit             bool     `yaml:"ratelimit"`
	Replenishment         float64  `yaml:"replenishment"`
	Burst                 int      `yaml:"burst"`
	Domains               []string `yaml:"domains"`
	LRUSize               int      `yaml:"lruSize"`
	SiteDescription       string   `yaml:"description"`
	OverviewCharCount     int      `yaml:"overviewCharCount"`
	PagesInAtomFeed       int      `yaml:"postsInAtomFeed"`
	MainContentInAtomFeed bool     `yaml:"mainContentInAtomFeed"`
}

func OpenDB(dbName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", filepath.Join(globals.DBPath, dbName))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL;"); err != nil {
		return nil, err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous=NORMAL;"); err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS checksums (filename TEXT PRIMARY KEY CHECK (filename LIKE '%.md'),hash TEXT NOT NULL)`)
	if err != nil {
		return nil, err
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS posts (url TEXT PRIMARY KEY,title TEXT NOT NULL, overview TEXT, overviewImg TEXT , category TEXT ,modifiedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, createdAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP )`)
	return db, err
}

func exit(code int) {
	if runtime.GOOS == "windows" {
		fmt.Println("Press Enter to exit.")
		fmt.Scanln()
	}
	os.Exit(code)
}

func main() {
	globals.SetPaths()
	deamon := flag.Bool("d", false, "Run as a background daemon")
	flag.Parse()
	if *deamon {
		exe, err := os.Executable()
		if err == nil {
			var args []string
			for _, arg := range os.Args[1:] {
				if arg != "-d" && arg != "--d" {
					args = append(args, arg)
				}
			}
			cmd := exec.Command(exe, args...)
			cmd.Start()
			fmt.Println("Scorial is running in the background.")
			os.Exit(0)
		}
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	file, err := os.Open(filepath.Join(globals.BinaryPath, "config.yaml"))
	if err != nil {
		logger.Error("Couldn't access config.yaml file!", "error", err.Error())
		exit(2)
	}
	var cfg config
	decoder := yaml.NewDecoder(file)
	if err = decoder.Decode(&cfg); err != nil {
		logger.Error("Error while parsing YAML config!", "error", err.Error())
		exit(3)
	}
	if cfg.LRUSize <= 0 {
		logger.Error("Max LRU Cache size has to be more than 0.")
		exit(4)
	}
	if cfg.CardsInHome <= 0 {
		logger.Error("Cards in home must be over 0.")
		exit(4)
	}
	if cfg.OverviewCharCount < 0 {
		logger.Error("OverviewCharCount cannot be smaller than 0.")
	}
	db, err := OpenDB("database.db")
	if err != nil {
		logger.Error("Couldn't open DB! Error:" + err.Error())
		exit(3)
	}
	var siteUrl string
	if cfg.Domains == nil {
		siteUrl = fmt.Sprintf("localhost:%v", cfg.Port)
	} else {
		siteUrl = cfg.Domains[0]
	}
	cmsConfig := server.CmsConfig{cfg.Port, cfg.CardsInHome, struct {
		Rps   float64
		Burst int
	}{cfg.Replenishment, cfg.Burst}, cfg.HTTPSMode, cfg.Ratelimit, cfg.CertPath, cfg.KeyPath, cfg.MdPath, cfg.SiteName, cfg.LogoPath, siteUrl, cfg.SiteDescription, cfg.FaviconPath, cfg.Domains, cfg.OverviewCharCount, cfg.PagesInAtomFeed, cfg.MainContentInAtomFeed}
	globals.LRUCacheSize = cfg.LRUSize
	cms := server.CmsStruct{logger, &cmsConfig, db}
	err = cms.Start()
	if !errors.Is(err, http.ErrServerClosed) && err != nil {
		cms.Logger.Error("Error while closing the server.", "error", err.Error())
		exit(1)
	}
	exit(0)
}
