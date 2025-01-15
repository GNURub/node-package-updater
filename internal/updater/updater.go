package updater

import (
	"sync"

	"github.com/GNURub/node-package-updater/internal/cache"
	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/dependency"
)

func FetchNewVersions(deps dependency.Dependencies, flags *cli.Flags, processed chan bool, currentPackage chan string, updateNotification chan bool, cache *cache.Cache) {
	numWorkers := 8
	jobs := make(chan *dependency.Dependency, len(deps))
	var wg sync.WaitGroup

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
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

	for _, dep := range deps {
		jobs <- dep
	}
	close(jobs)

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
