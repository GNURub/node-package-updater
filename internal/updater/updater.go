package updater

import (
	"sync"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/dependency"
)

func FetchNewVersions(deps dependency.Dependencies, flags *cli.Flags, processed chan bool, currentPackage chan string, updateNotification chan bool, cache *cache.Cache) {
	numWorkers := 10
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

				newVersion, err := dep.GetNewVersion(flags, cache)

				if err == nil && newVersion != "" {
					dep.NextVersion = newVersion
					updateNotification <- true
				}

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

func NeedToUpdate(allDeps map[string]dependency.Dependencies) bool {
	for _, deps := range allDeps {
		for _, dep := range deps {
			if dep.HaveToUpdate {
				return true
			}
		}
	}

	return false
}
