package updater

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/performance"
	"github.com/GNURub/node-package-updater/internal/pipeline"
	"github.com/valyala/fasthttp"
)

var (
	// Cliente HTTP reutilizable para mejorar rendimiento
	httpClient = &fasthttp.Client{
		MaxConnsPerHost:               64,
		MaxIdleConnDuration:           30 * time.Second,
		ReadTimeout:                   10 * time.Second,
		WriteTimeout:                  10 * time.Second,
		MaxConnWaitTimeout:            5 * time.Second,
		DisableHeaderNamesNormalizing: true,
	}

	// Optimizador de memoria global
	memOptimizer = performance.NewMemoryOptimizer(50)
)

func FetchNewVersions(deps dependency.Dependencies, flags *cli.Flags, processed chan bool, currentPackage chan string, cache *cache.Cache) {
	// Usar la configuración de CPUs del usuario si está disponible
	numWorkers := flags.CPUs
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU() * 4 // Aumentamos el multiplicador para I/O bound operations
	}

	// Limitar workers si hay pocas dependencias
	if len(deps) < numWorkers {
		numWorkers = len(deps)
	}

	// Pre-agrupar dependencias por registry para optimizar conexiones
	depsByRegistry := groupDependenciesByRegistry(deps, flags)

	// Usar worker pool optimizado para mejor gestión de recursos
	pool := performance.NewWorkerPool(numWorkers)
	pool.Start()
	defer pool.Stop()

	var wg sync.WaitGroup

	// Goroutine separada para cerrar canales
	go func() {
		wg.Wait()
		close(processed)
		close(currentPackage)
	}()

	// Procesar dependencias agrupadas por registry con worker pool
	for _, registryDeps := range depsByRegistry {
		for _, dep := range registryDeps {
			wg.Add(1)

			// Capturar variables para el closure
			currentDep := dep

			pool.Submit(func() {
				defer func() {
					wg.Done()
					memOptimizer.ProcessedPackage()
				}()

				// Timeout más corto para operaciones individuales
				timeoutDuration := time.Duration(flags.Timeout) * time.Second
				if timeoutDuration > 30*time.Second {
					timeoutDuration = 30 * time.Second
				}

				ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
				defer cancel()

				// Non-blocking send para evitar deadlocks
				select {
				case currentPackage <- currentDep.PackageName:
				case <-ctx.Done():
					return
				default:
				}

				currentDep.FetchNewVersion(ctx, flags, cache)

				select {
				case processed <- true:
				case <-ctx.Done():
					return
				default:
				}
			})
		}
	}
	wg.Wait()
}

// FetchNewVersionsBatch procesa dependencias en lotes para mejor rendimiento
func FetchNewVersionsBatch(deps dependency.Dependencies, flags *cli.Flags, processed chan bool, currentPackage chan string, cache *cache.Cache) {
	const batchSize = 20

	// Dividir dependencias en lotes
	batches := make([]dependency.Dependencies, 0, (len(deps)+batchSize-1)/batchSize)
	for i := 0; i < len(deps); i += batchSize {
		end := i + batchSize
		if end > len(deps) {
			end = len(deps)
		}
		batches = append(batches, deps[i:end])
	}

	// Procesar lotes en paralelo
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, runtime.NumCPU())

	for _, batch := range batches {
		semaphore <- struct{}{}
		wg.Add(1)

		go func(batchDeps dependency.Dependencies) {
			defer func() {
				<-semaphore
				wg.Done()
			}()

			FetchNewVersions(batchDeps, flags, processed, currentPackage, cache)
		}(batch)
	}

	wg.Wait()
	close(processed)
	close(currentPackage)
}

// Agrupa dependencias por registry para optimizar conexiones HTTP
func groupDependenciesByRegistry(deps dependency.Dependencies, flags *cli.Flags) map[string]dependency.Dependencies {
	groups := make(map[string]dependency.Dependencies)
	defaultRegistry := flags.Registry
	if defaultRegistry == "" {
		defaultRegistry = "https://registry.npmjs.org/"
	}

	for _, dep := range deps {
		// Por ahora agrupamos por registry por defecto
		// En el futuro se puede extender para detectar scoped packages
		registry := defaultRegistry
		if groups[registry] == nil {
			groups[registry] = make(dependency.Dependencies, 0)
		}
		groups[registry] = append(groups[registry], dep)
	}

	return groups
}

// FetchNewVersionsOptimized usa el pipeline optimizado para mejor rendimiento
func FetchNewVersionsOptimized(deps dependency.Dependencies, flags *cli.Flags, processed chan bool, currentPackage chan string, cache *cache.Cache) {
	// Usar el procesador paralelo optimizado
	processor := pipeline.NewParallelDependencyProcessor(cache, flags)
	defer processor.Cleanup()

	// Agrupar dependencias por registry
	depsByRegistry := groupDependenciesByRegistry(deps, flags)

	// Procesar usando el pipeline optimizado
	results, errors := processor.ProcessByRegistry(depsByRegistry)

	// Enviar notificaciones de progreso
	go func() {
		defer func() {
			close(processed)
			close(currentPackage)
		}()

		for _, dep := range results {
			select {
			case currentPackage <- dep.PackageName:
			default:
			}

			select {
			case processed <- true:
			default:
			}
		}

		// Reportar errores si los hay
		for _, err := range errors {
			_ = err // TODO: manejar errores apropiadamente
		}
	}()
}
