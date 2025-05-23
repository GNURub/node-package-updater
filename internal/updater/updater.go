package updater

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/dependency"
)

func FetchNewVersions(deps dependency.Dependencies, flags *cli.Flags, processed chan bool, currentPackage chan string, cache *cache.Cache) {
	numCPUs := runtime.NumCPU()
	numWorkers := numCPUs * 2
	if len(deps) < numWorkers {
		numWorkers = len(deps)
	}

	sem := make(chan struct{}, numWorkers)
	var wg sync.WaitGroup

	go func() {
		wg.Wait()
		close(processed)
		close(currentPackage)
	}()

	for _, dep := range deps {
		sem <- struct{}{}
		wg.Add(1)

		go func(dep *dependency.Dependency) {
			defer func() {
				<-sem
				wg.Done()
			}()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(flags.Timeout))
			defer cancel()

			select {
			case currentPackage <- dep.PackageName:
			default:
			}
			dep.FetchNewVersion(ctx, flags, cache)
			select {
			case processed <- true:
			default:
			}
		}(dep)
	}
	wg.Wait()
}
