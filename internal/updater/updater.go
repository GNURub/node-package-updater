package updater

import (
	"sync"

	"github.com/GNURub/node-package-updater/internal/cli"
	"github.com/GNURub/node-package-updater/internal/dependency"
)

func FetchNewVersions(deps dependency.Dependencies, flags *cli.Flags, processed chan bool, currentPackage chan string, updateNotification chan bool) {
	numWorkers := 8
	jobs := make(chan *dependency.Dependency, len(deps))
	var wg sync.WaitGroup

	// Start worker pool
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for dep := range jobs {
				currentPackage <- dep.PackageName

				newVersion, err := dep.GetNewVersion(flags)

				if err == nil && newVersion != "" {
					dep.NextVersion = newVersion
					updateNotification <- true
				}

				processed <- true
			}
		}()
	}

	// Send jobs to workers
	for _, dep := range deps {
		jobs <- dep
	}
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()
}
