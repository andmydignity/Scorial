package filesync

import (
	"os"
	"sync"
)

var (
	pageCache  = map[string][]byte{}
	mutexCache = sync.RWMutex{}
	cacheSize  = 100
	pageList   = []string{}
)

func FromCache(path string) []byte {
	mutexCache.RLock()
	val := pageCache[path]
	mutexCache.RUnlock()
	return val
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
	pageCache[path] = data
	mutexCache.Unlock()
	pageList = append(pageList, path)
	purgeCache()
	return data, nil
}

func purgeCache() {
	mutexCache.Lock()
	for len(pageList) > cacheSize {
		delete(pageCache, pageList[0])
		pageList = pageList[1:]
	}
	mutexCache.Unlock()
}
