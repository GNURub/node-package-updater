package cache

import (
	"errors"
	"os"
	"path"

	"git.mills.io/prologic/bitcask"
)

const CACHE_APP_DIR = ".npu-cache"

type Cache struct {
	db *bitcask.Bitcask
}

func NewCache() (*Cache, error) {
	cacheDir := path.Join(os.TempDir(), CACHE_APP_DIR)
	ensureDir(cacheDir)

	// 300MB
	maxDatafileSize := 1024 * 1024 * 300
	db, err := bitcask.Open(cacheDir, bitcask.WithMaxDatafileSize(maxDatafileSize))
	if err != nil {
		return nil, err
	}

	return &Cache{db}, nil
}

func (cache *Cache) Close() error {
	return cache.db.Close()
}

func (cache *Cache) Has(key string) bool {
	return cache.db.Has([]byte(key))
}

func (cache *Cache) Get(key string) ([]byte, error) {
	if !cache.db.Has([]byte(key)) {
		return nil, errors.New("key not found")
	}

	return cache.db.Get([]byte(key))
}

func (cache *Cache) Set(key string, data []byte) error {
	return cache.db.Put([]byte(key), data)
}

func ensureDir(dirName string) error {
	if _, err := os.Stat(dirName); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(dirName, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return nil
}
