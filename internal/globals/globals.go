// Package globals contains global values (mostly constants) that i cannot be bothered to pass around 50 billion function calls like its a fucking hot potato.
package globals

import (
	"os"
	"path/filepath"
)

var (
	AssetsPath, CertsPath, DBPath, BinaryPath string
	LRUCacheSize                              int
	HomePageCache, AtomCache                  []byte
	HomePageChecksumCache, AtomChecksumCache  string
)

func getBinaryDir() (string, error) {
	exPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(exPath)
	if err != nil {
		return "", err
	}
	return filepath.Dir(resolvedPath), nil
}

func SetPaths() error {
	binDir, err := getBinaryDir()
	if err != nil {
		return err
	}
	AssetsPath = filepath.Join(binDir, "assets")
	CertsPath = filepath.Join(binDir, "certs")
	DBPath = filepath.Join(binDir, "databases")
	BinaryPath = binDir
	return nil
}
