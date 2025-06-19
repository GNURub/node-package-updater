package pipeline

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/dependency"
)

// DependencyPipeline optimiza el procesamiento de dependencias
type DependencyPipeline struct {
	workers   int
	batchSize int
	cache     *cache.Cache
	flags     *cli.Flags

	// Métricas de rendimiento
	processedCount int64
	errorCount     int64
	totalDuration  time.Duration

	// Canales para comunicación
	input  chan *dependency.Dependency
	output chan *dependency.Dependency
	errors chan error
	done   chan struct{}

	// Control de flujo
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	// Pools para reutilización
	ctxPool sync.Pool
}

// NewDependencyPipeline crea un nuevo pipeline optimizado
func NewDependencyPipeline(workers, batchSize int, cache *cache.Cache, flags *cli.Flags) *DependencyPipeline {
	ctx, cancel := context.WithCancel(context.Background())

	return &DependencyPipeline{
		workers:   workers,
		batchSize: batchSize,
		cache:     cache,
		flags:     flags,
		input:     make(chan *dependency.Dependency, workers*2),
		output:    make(chan *dependency.Dependency, workers*2),
		errors:    make(chan error, workers),
		done:      make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,
		ctxPool: sync.Pool{
			New: func() interface{} {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				return &ctxWithCancel{ctx: ctx, cancel: cancel}
			},
		},
	}
}

type ctxWithCancel struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Start inicia el pipeline
func (p *DependencyPipeline) Start() {
	// Iniciar workers
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	// Iniciar collector de resultados
	go p.resultCollector()
}

// Stop detiene el pipeline
func (p *DependencyPipeline) Stop() {
	close(p.input)
	p.wg.Wait()
	close(p.output)
	close(p.errors)
	p.cancel()
}

// ProcessDependencies procesa una lista de dependencias
func (p *DependencyPipeline) ProcessDependencies(deps dependency.Dependencies) (<-chan *dependency.Dependency, <-chan error) {
	go func() {
		defer close(p.input)

		// Enviar dependencias al pipeline en lotes
		batches := p.createBatches(deps)
		for _, batch := range batches {
			for _, dep := range batch {
				select {
				case p.input <- dep:
				case <-p.ctx.Done():
					return
				}
			}
		}
	}()

	return p.output, p.errors
}

// worker procesa dependencias individualmente
func (p *DependencyPipeline) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case dep, ok := <-p.input:
			if !ok {
				return
			}

			start := time.Now()
			if err := p.processDependency(dep); err != nil {
				atomic.AddInt64(&p.errorCount, 1)
				select {
				case p.errors <- err:
				case <-p.ctx.Done():
					return
				default:
				}
			} else {
				atomic.AddInt64(&p.processedCount, 1)
				select {
				case p.output <- dep:
				case <-p.ctx.Done():
					return
				}
			}

			// Actualizar métricas
			atomic.AddInt64((*int64)(&p.totalDuration), int64(time.Since(start)))

		case <-p.ctx.Done():
			return
		}
	}
}

// processDependency procesa una dependencia individual
func (p *DependencyPipeline) processDependency(dep *dependency.Dependency) error {
	// Obtener contexto del pool
	ctxItem := p.ctxPool.Get().(*ctxWithCancel)
	defer func() {
		ctxItem.cancel()
		ctxItem = &ctxWithCancel{}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		ctxItem.ctx = ctx
		ctxItem.cancel = cancel
		p.ctxPool.Put(ctxItem)
	}()

	return dep.FetchNewVersion(ctxItem.ctx, p.flags, p.cache)
}

// resultCollector recolecta resultados en segundo plano
func (p *DependencyPipeline) resultCollector() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Ejecutar limpieza periódica del cache
			if p.cache != nil {
				p.cache.AutoCleanup()
			}
		case <-p.ctx.Done():
			return
		}
	}
}

// createBatches divide las dependencias en lotes optimizados
func (p *DependencyPipeline) createBatches(deps dependency.Dependencies) []dependency.Dependencies {
	if len(deps) <= p.batchSize {
		return []dependency.Dependencies{deps}
	}

	numBatches := (len(deps) + p.batchSize - 1) / p.batchSize
	batches := make([]dependency.Dependencies, numBatches)

	for i := 0; i < numBatches; i++ {
		start := i * p.batchSize
		end := start + p.batchSize
		if end > len(deps) {
			end = len(deps)
		}
		batches[i] = deps[start:end]
	}

	return batches
}

// GetMetrics retorna métricas de rendimiento
func (p *DependencyPipeline) GetMetrics() PipelineMetrics {
	processed := atomic.LoadInt64(&p.processedCount)
	errors := atomic.LoadInt64(&p.errorCount)
	duration := time.Duration(atomic.LoadInt64((*int64)(&p.totalDuration)))

	var avgDuration time.Duration
	if processed > 0 {
		avgDuration = duration / time.Duration(processed)
	}

	return PipelineMetrics{
		ProcessedCount:      processed,
		ErrorCount:          errors,
		TotalDuration:       duration,
		AverageDuration:     avgDuration,
		ThroughputPerSecond: float64(processed) / duration.Seconds(),
	}
}

// PipelineMetrics contiene métricas de rendimiento del pipeline
type PipelineMetrics struct {
	ProcessedCount      int64
	ErrorCount          int64
	TotalDuration       time.Duration
	AverageDuration     time.Duration
	ThroughputPerSecond float64
}

// ParallelDependencyProcessor optimiza el procesamiento paralelo
type ParallelDependencyProcessor struct {
	pipeline *DependencyPipeline
	registry map[string]*DependencyPipeline // Pipeline por registry
	mu       sync.RWMutex
}

// NewParallelDependencyProcessor crea un procesador paralelo optimizado
func NewParallelDependencyProcessor(cache *cache.Cache, flags *cli.Flags) *ParallelDependencyProcessor {
	workers := flags.CPUs
	if workers <= 0 {
		workers = 16
	}

	return &ParallelDependencyProcessor{
		pipeline: NewDependencyPipeline(workers, 20, cache, flags),
		registry: make(map[string]*DependencyPipeline),
	}
}

// ProcessByRegistry procesa dependencias agrupadas por registry
func (p *ParallelDependencyProcessor) ProcessByRegistry(depsByRegistry map[string]dependency.Dependencies) (dependency.Dependencies, []error) {
	var allResults dependency.Dependencies
	var allErrors []error
	var mu sync.Mutex
	var wg sync.WaitGroup

	for registry, deps := range depsByRegistry {
		wg.Add(1)

		go func(reg string, dependencies dependency.Dependencies) {
			defer wg.Done()

			pipeline := p.getPipelineForRegistry(reg)
			pipeline.Start()
			defer pipeline.Stop()

			resultChan, errorChan := pipeline.ProcessDependencies(dependencies)

			// Recopilar resultados
			var results dependency.Dependencies
			var errors []error

			done := make(chan struct{})
			go func() {
				defer close(done)
				for result := range resultChan {
					results = append(results, result)
				}
			}()

			// Recopilar errores
			go func() {
				for err := range errorChan {
					errors = append(errors, err)
				}
			}()

			<-done

			// Agregar a resultados globales
			mu.Lock()
			allResults = append(allResults, results...)
			allErrors = append(allErrors, errors...)
			mu.Unlock()

		}(registry, deps)
	}

	wg.Wait()
	return allResults, allErrors
}

// getPipelineForRegistry obtiene o crea un pipeline para un registry específico
func (p *ParallelDependencyProcessor) getPipelineForRegistry(registry string) *DependencyPipeline {
	p.mu.RLock()
	if pipeline, exists := p.registry[registry]; exists {
		p.mu.RUnlock()
		return pipeline
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check pattern
	if pipeline, exists := p.registry[registry]; exists {
		return pipeline
	}

	// Crear nuevo pipeline para este registry
	workers := p.pipeline.workers / 2
	if workers < 4 {
		workers = 4
	}

	pipeline := NewDependencyPipeline(workers, 10, p.pipeline.cache, p.pipeline.flags)
	p.registry[registry] = pipeline

	return pipeline
}

// Cleanup limpia recursos del procesador
func (p *ParallelDependencyProcessor) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, pipeline := range p.registry {
		pipeline.Stop()
	}

	if p.pipeline != nil {
		p.pipeline.Stop()
	}
}
