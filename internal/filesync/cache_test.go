package filesync

import (
	"os"
	"testing"

	"github.com/andmydignity/Scorial/internal/globals"
)

func TestAppendToCacheAndFromCache(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "cachetest")
	globals.LRUCacheSize = 200
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	content := []byte("testdata")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	data, err := AppendToCache(tmpfile.Name())
	if err != nil {
		t.Fatalf("AppendToCache error: %v", err)
	}
	if string(data) != "testdata" {
		t.Errorf("expected 'testdata', got %s", string(data))
	}

	cached := FromCache(tmpfile.Name())
	if string(cached) != "testdata" {
		t.Errorf("FromCache expected 'testdata', got %s", string(cached))
	}
}

func TestCacheEviction(t *testing.T) {
	// Set cacheSize to a small number for testing
	globals.LRUCacheSize = 2
	pageCache = map[string]page{}
	pageList = []string{}

	files := []string{}
	for i := 0; i < 3; i++ {
		tmpfile, _ := os.CreateTemp("", "cachetest")
		defer os.Remove(tmpfile.Name())
		tmpfile.Write([]byte{byte(i)})
		tmpfile.Close()
		files = append(files, tmpfile.Name())
		AppendToCache(tmpfile.Name())
	}
	if len(pageCache) != 2 {
		t.Errorf("expected cache size 2, got %d", len(pageCache))
	}
}
