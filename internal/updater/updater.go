package updater

import (
	"sync"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/dependency"
)

func FetchNewVersions(deps dependency.Dependencies, flags *cli.Flags, processed chan bool, currentPackage chan string, cache *cache.Cache) {
	numWorkers := 20
	if len(deps) < numWorkers {
		numWorkers = len(deps)
	}

	jobs := make(chan *dependency.Dependency, len(deps))
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			for dep := range jobs {
				currentPackage <- dep.PackageName

				dep.FetchNewVersion(flags, cache)

				processed <- true
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, dep := range deps {
			jobs <- dep
		}
	}()

	wg.Wait()
}
