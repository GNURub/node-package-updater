package cache

import (
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path"
	"time"

	"git.mills.io/prologic/bitcask"
)

const CACHE_APP_DIR = ".npu-cache"

type Cache struct {
	db *bitcask.Bitcask
}

func NewCache() (*Cache, error) {
	cacheDir := path.Join(os.TempDir(), CACHE_APP_DIR)
	ensureDir(cacheDir)
	db, err := bitcask.Open(cacheDir)
	if err != nil {
		return nil, err
	}

	return &Cache{db: db}, nil
}

func (cache *Cache) Close() error {
	return cache.db.Close()
}

func (cache *Cache) getKeyHash(url string) string {
	keyHash := fnv.New64()
	keyHash.Write([]byte(url))
	return fmt.Sprintf("%v", keyHash.Sum64())
}

func (cache *Cache) Get(url string) (data []byte, err error) {
	key := cache.getKeyHash(url)

	if !cache.db.Has([]byte(key)) {
		return nil, errors.New("key not found")
	}

	data, err = cache.db.Get([]byte(key))

	if err != nil {
		return
	}

	return
}

func (cache *Cache) Set(url string, data []byte) error {
	key := cache.getKeyHash(url)
	return cache.db.PutWithTTL([]byte(key), data, 30*time.Second)
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
