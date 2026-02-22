package render

import (
	"os"
	"path/filepath"
)

func loadFromFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func saveToFile(data []byte, saveTo string) error {
	dir := filepath.Dir(saveTo)
	if err := os.MkdirAll(dir, 0o755); err != nil || err == os.ErrExist {
		return err
	}
	return os.WriteFile(saveTo, data, 0o644)
}
