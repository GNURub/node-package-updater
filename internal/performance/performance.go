package performance

import (
	"runtime"
	"sync"
	"time"

	"github.com/valyala/fastjson"
)

// JSONParser pool para reutilizar parsers
var (
	jsonParserPool = sync.Pool{
		New: func() interface{} {
			return &fastjson.Parser{}
		},
	}

	// Buffer pool para operaciones de JSON
	jsonBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 1024)
		},
	}
)

// FastJSONParser optimiza el parsing de JSON usando fastjson
type FastJSONParser struct {
	parser *fastjson.Parser
}

// NewFastJSONParser crea un nuevo parser optimizado
func NewFastJSONParser() *FastJSONParser {
	return &FastJSONParser{
		parser: jsonParserPool.Get().(*fastjson.Parser),
	}
}

// Release devuelve el parser al pool
func (p *FastJSONParser) Release() {
	jsonParserPool.Put(p.parser)
}

// ParseNPMResponse parsea la respuesta del registry de manera optimizada
func (p *FastJSONParser) ParseNPMResponse(data []byte) (*NPMResponseData, error) {
	v, err := p.parser.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	result := &NPMResponseData{
		DistTags: make(map[string]string),
		Versions: make(map[string]VersionData),
	}

	// Parse dist-tags
	if distTags := v.Get("dist-tags"); distTags != nil {
		distTags.GetObject().Visit(func(key []byte, v *fastjson.Value) {
			result.DistTags[string(key)] = string(v.GetStringBytes())
		})
	}

	// Parse versions
	if versions := v.Get("versions"); versions != nil {
		versions.GetObject().Visit(func(key []byte, v *fastjson.Value) {
			versionKey := string(key)
			versionData := VersionData{}

			if dist := v.Get("dist"); dist != nil {
				if unpackedSize := dist.Get("unpackedSize"); unpackedSize != nil {
					versionData.UnpackedSize = uint64(unpackedSize.GetUint64())
				}
			}

			if deprecated := v.Get("deprecated"); deprecated != nil {
				versionData.Deprecated = string(deprecated.GetStringBytes())
			}

			result.Versions[versionKey] = versionData
		})
	}

	return result, nil
}

// NPMResponseData estructura optimizada para datos del registry
type NPMResponseData struct {
	DistTags map[string]string      `json:"dist-tags"`
	Versions map[string]VersionData `json:"versions"`
}

// VersionData estructura optimizada para datos de versión
type VersionData struct {
	UnpackedSize uint64 `json:"unpackedSize"`
	Deprecated   string `json:"deprecated"`
}

// MemoryOptimizer gestiona la memoria de manera eficiente
type MemoryOptimizer struct {
	gcThreshold int
	processed   int
	mu          sync.Mutex
}

// NewMemoryOptimizer crea un nuevo optimizador de memoria
func NewMemoryOptimizer(threshold int) *MemoryOptimizer {
	return &MemoryOptimizer{
		gcThreshold: threshold,
	}
}

// ProcessedPackage incrementa el contador y ejecuta GC si es necesario
func (m *MemoryOptimizer) ProcessedPackage() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processed++
	if m.processed%m.gcThreshold == 0 {
		runtime.GC()
		runtime.Gosched() // Permitir que otras goroutines ejecuten
	}
}

// HTTPResponseOptimizer optimiza el manejo de respuestas HTTP
type HTTPResponseOptimizer struct {
	maxBodySize int64
}

// NewHTTPResponseOptimizer crea un nuevo optimizador de respuestas HTTP
func NewHTTPResponseOptimizer(maxBodySize int64) *HTTPResponseOptimizer {
	return &HTTPResponseOptimizer{
		maxBodySize: maxBodySize,
	}
}

// ShouldDecompress determina si una respuesta debe descomprimirse
func (h *HTTPResponseOptimizer) ShouldDecompress(contentLength int64, contentEncoding string) bool {
	if contentLength > h.maxBodySize {
		return false
	}
	return contentEncoding == "gzip" || contentEncoding == "deflate"
}

// ConcurrencyOptimizer gestiona la concurrencia de manera inteligente
type ConcurrencyOptimizer struct {
	maxWorkers     int
	currentWorkers int
	mu             sync.RWMutex
	avgDuration    time.Duration
	samples        []time.Duration
	sampleSize     int
}

// NewConcurrencyOptimizer crea un nuevo optimizador de concurrencia
func NewConcurrencyOptimizer(maxWorkers, sampleSize int) *ConcurrencyOptimizer {
	return &ConcurrencyOptimizer{
		maxWorkers: maxWorkers,
		sampleSize: sampleSize,
		samples:    make([]time.Duration, 0, sampleSize),
	}
}

// RecordDuration registra la duración de una operación
func (c *ConcurrencyOptimizer) RecordDuration(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.samples = append(c.samples, duration)
	if len(c.samples) > c.sampleSize {
		c.samples = c.samples[1:]
	}

	// Calcular promedio
	var total time.Duration
	for _, d := range c.samples {
		total += d
	}
	c.avgDuration = total / time.Duration(len(c.samples))
}

// OptimalWorkerCount calcula el número óptimo de workers
func (c *ConcurrencyOptimizer) OptimalWorkerCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.samples) < 5 {
		return c.maxWorkers / 2
	}

	// Si las operaciones son rápidas (< 100ms), usar más workers
	if c.avgDuration < 100*time.Millisecond {
		return c.maxWorkers
	}

	// Si son lentas (> 1s), usar menos workers
	if c.avgDuration > time.Second {
		return c.maxWorkers / 4
	}

	return c.maxWorkers / 2
}

// WorkerPool maneja un pool de workers optimizado
type WorkerPool struct {
	workers   int
	jobQueue  chan func()
	wg        sync.WaitGroup
	done      chan struct{}
	optimizer *ConcurrencyOptimizer
}

// NewWorkerPool crea un nuevo pool de workers
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		workers:   workers,
		jobQueue:  make(chan func(), workers*2),
		done:      make(chan struct{}),
		optimizer: NewConcurrencyOptimizer(workers, 50),
	}
}

// Start inicia el pool de workers
func (p *WorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// Stop detiene el pool de workers
func (p *WorkerPool) Stop() {
	close(p.done)
	p.wg.Wait()
}

// Submit envía una tarea al pool
func (p *WorkerPool) Submit(job func()) {
	select {
	case p.jobQueue <- job:
	case <-p.done:
		return
	}
}

// worker ejecuta tareas del pool
func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for {
		select {
		case job := <-p.jobQueue:
			start := time.Now()
			job()
			p.optimizer.RecordDuration(time.Since(start))
		case <-p.done:
			return
		}
	}
}

// GetOptimalWorkerCount retorna el número óptimo de workers basado en métricas
func (p *WorkerPool) GetOptimalWorkerCount() int {
	return p.optimizer.OptimalWorkerCount()
}
