package filesync

import (
	"os"
	"sync"

	"github.com/andmydignity/Scorial/internal/globals"
)

type page struct {
	Data     []byte
	Checksum string
}

var (
	pageCache  = map[string]page{}
	mutexCache = sync.RWMutex{}
	pageList   = []string{}
)

func FromCache(path string) []byte {
	mutexCache.RLock()
	val := pageCache[path]
	mutexCache.RUnlock()
	return val.Data
}

func ChecksumFromCache(path string) string {
	mutexCache.RLock()
	val := pageCache[path]
	mutexCache.RUnlock()
	return val.Checksum
}

func deleteFromCache(path string) {
	mutexCache.Lock()
	delete(pageCache, path)
	mutexCache.Unlock()
}

func AppendToCache(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	mutexCache.Lock()
	checksum, err := checksumCalculate(path)
	if err != nil {
		return nil, err
	}
	pageCache[path] = page{data, checksum}
	pageList = append(pageList, path)
	mutexCache.Unlock()
	purgeCache()
	return data, nil
}

func purgeCache() {
	mutexCache.Lock()
	for len(pageList) > globals.LRUCacheSize {
		delete(pageCache, pageList[0])
		pageList = pageList[1:]
	}
	mutexCache.Unlock()
}
