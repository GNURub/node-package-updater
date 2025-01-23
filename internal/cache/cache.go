package cache

import (
	"errors"
	"os"
	"path"
	"sync"

	"git.mills.io/prologic/bitcask"
)

const CACHE_APP_DIR = ".npu-cache"

type Cache struct {
	db       *bitcask.Bitcask
	memCache sync.Map
}

func NewCache() (*Cache, error) {
	cacheDir := path.Join(os.TempDir(), CACHE_APP_DIR)
	ensureDir(cacheDir)

	maxDatafileSize := 1024 * 1024 * 800
	maxKeySize := 1024 * 1024 * 5
	maxValueSize := 1024 * 1024 * 15
	db, err := bitcask.Open(
		cacheDir,
		bitcask.WithMaxDatafileSize(maxDatafileSize),
		bitcask.WithMaxKeySize(uint32(maxKeySize)),
		bitcask.WithMaxValueSize(uint64(maxValueSize)),
	)
	if err != nil {
		return nil, err
	}

	return &Cache{db: db}, nil
}

func (cache *Cache) Clean() error {
	cache.memCache = sync.Map{}
	return cache.db.DeleteAll()
}

func (cache *Cache) Close() error {
	return cache.db.Close()
}

func (cache *Cache) Has(key string) bool {
	_, found := cache.memCache.Load(key)
	if found {
		return true
	}
	return cache.db.Has([]byte(key))
}

func (cache *Cache) Get(key string) ([]byte, error) {
	if value, found := cache.memCache.Load(key); found {
		return value.([]byte), nil
	}

	if !cache.db.Has([]byte(key)) {
		return nil, errors.New("key not found")
	}

	data, err := cache.db.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	cache.memCache.Store(key, data)

	return data, nil
}

func (cache *Cache) Set(key string, data []byte) error {
	cache.memCache.Store(key, data)

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
