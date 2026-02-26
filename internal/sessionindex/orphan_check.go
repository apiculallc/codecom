package sessionindex

import (
	"os"
	"runtime"
	"sync"
)

func DefaultOrphanWorkers() int {
	w := runtime.NumCPU() * 2
	if w > 32 {
		return 32
	}
	if w < 1 {
		return 1
	}
	return w
}

func ApplyOrphanStatus(records []SessionRecord, workers int) {
	if len(records) == 0 {
		return
	}
	if workers <= 0 {
		workers = DefaultOrphanWorkers()
	}

	jobs := make(chan int)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				cwd := records[idx].EffectiveCWD()
				if cwd == "" {
					records[idx].Orphan = true
					continue
				}
				if _, err := os.Stat(cwd); err != nil {
					records[idx].Orphan = true
				}
			}
		}()
	}

	for i := range records {
		records[i].Orphan = false
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}
