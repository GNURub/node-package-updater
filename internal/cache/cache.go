package cache

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"git.mills.io/prologic/bitcask"
)

const CACHE_APP_DIR = ".npu-cache"

type Cache struct {
	db           *bitcask.Bitcask
	memCache     sync.Map
	lastCleanup  time.Time
	cleanupMutex sync.RWMutex
}

// CacheOptions configuración para optimizaciones del cache
type CacheOptions struct {
	EnableCompression bool
	CompressionLevel  int
	MemCacheSize      int
	TTL               time.Duration
}

// BatchOperation representa una operación batch
type BatchOperation struct {
	Key  string
	Data []byte
}

var (
	// Pool de buffers para compresión
	bufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 1024))
		},
	}

	// Pool de compresores gzip (mantener solo el que funciona bien)
	gzipWriterPool = sync.Pool{
		New: func() interface{} {
			return gzip.NewWriter(io.Discard)
		},
	}
)

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

	return &Cache{
		db:          db,
		lastCleanup: time.Now(),
	}, nil
}

func (cache *Cache) Clean() error {
	cache.cleanupMutex.Lock()
	defer cache.cleanupMutex.Unlock()

	cache.memCache = sync.Map{}
	cache.lastCleanup = time.Now()
	return cache.db.DeleteAll()
}

func (cache *Cache) Close() error {
	return cache.db.Close()
}

// AutoCleanup limpia automáticamente la memoria cache si ha pasado mucho tiempo
func (cache *Cache) AutoCleanup() {
	cache.cleanupMutex.RLock()
	shouldClean := time.Since(cache.lastCleanup) > 30*time.Minute
	cache.cleanupMutex.RUnlock()

	if shouldClean {
		cache.cleanupMutex.Lock()
		// Double-check pattern
		if time.Since(cache.lastCleanup) > 30*time.Minute {
			cache.memCache = sync.Map{}
			cache.lastCleanup = time.Now()
		}
		cache.cleanupMutex.Unlock()
	}
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

	// Verificar si los datos están comprimidos de forma simple
	if len(data) > 0 && data[0] == 1 {
		if decompressedData, err := cache.tryDecompress(data); err == nil {
			data = decompressedData
		}
	}

	cache.memCache.Store(key, data)
	return data, nil
}

func (cache *Cache) Set(key string, data []byte) error {
	// Comprimir datos grandes para ahorrar espacio (simplificado)
	compressedData := data
	if len(data) > 1024 { // Solo comprimir datos grandes
		if compressed, err := cache.compressIfBeneficial(data); err == nil {
			compressedData = compressed
		}
	}

	cache.memCache.Store(key, data) // Almacenar descomprimido en memoria
	return cache.db.Put([]byte(key), compressedData)
}

// SetBatch realiza múltiples operaciones de escritura en batch
func (cache *Cache) SetBatch(operations []BatchOperation) error {
	errChan := make(chan error, len(operations))
	semaphore := make(chan struct{}, 10) // Limitar concurrencia

	var wg sync.WaitGroup
	for _, op := range operations {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(operation BatchOperation) {
			defer func() {
				<-semaphore
				wg.Done()
			}()

			if err := cache.Set(operation.Key, operation.Data); err != nil {
				select {
				case errChan <- err:
				default:
				}
			}
		}(op)
	}

	wg.Wait()
	close(errChan)

	// Retornar el primer error encontrado
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// compressIfBeneficial comprime datos si vale la pena
func (cache *Cache) compressIfBeneficial(data []byte) ([]byte, error) {
	// Solo comprimir si los datos son lo suficientemente grandes
	if len(data) < 256 {
		return data, nil
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	gzWriter := gzipWriterPool.Get().(*gzip.Writer)
	defer gzipWriterPool.Put(gzWriter)

	gzWriter.Reset(buf)

	if _, err := gzWriter.Write(data); err != nil {
		return data, err
	}

	if err := gzWriter.Close(); err != nil {
		return data, err
	}

	compressed := buf.Bytes()

	// Solo usar compresión si ahorra al menos 20% de espacio
	if len(compressed) < len(data)*4/5 {
		// Agregar marcador de compresión
		result := make([]byte, len(compressed)+1)
		result[0] = 1 // Marcador de compresión
		copy(result[1:], compressed)
		return result, nil
	}

	return data, nil
}

// tryDecompress intenta descomprimir datos si tienen el marcador
func (cache *Cache) tryDecompress(data []byte) ([]byte, error) {
	if len(data) == 0 || data[0] != 1 {
		return data, errors.New("not compressed")
	}

	compressedData := data[1:] // Remover marcador

	// Crear un nuevo reader en lugar de usar el pool para evitar problemas de concurrencia
	reader := bytes.NewReader(compressedData)
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if _, err := io.Copy(buf, gzReader); err != nil {
		return nil, err
	}

	// Hacer una copia de los datos para evitar problemas con el buffer pool
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return result, nil
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
