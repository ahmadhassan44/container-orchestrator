package worker

import (
	"math"
	"sync"
	"time"
)

// GenerateCPULoad creates CPU load at specified percentage for specified duration
// cpuPercent: target CPU utilization (0-100)
// durationSeconds: how long to sustain the load
// threads: number of goroutines to use (from GOMAXPROCS)
func GenerateCPULoad(cpuPercent float64, durationSeconds float64, threads int) float64 {
	var wg sync.WaitGroup
	wg.Add(threads)

	// Calculate work/sleep ratio to achieve target CPU percentage
	// cpuPercent of 50 means: work 50ms, sleep 50ms in each 100ms window
	workRatio := cpuPercent / 100.0

	// Use 10ms as the base time quantum for work/sleep cycles
	quantumMs := 10 * time.Millisecond
	workTime := time.Duration(float64(quantumMs) * workRatio)
	sleepTime := quantumMs - workTime

	endTime := time.Now().Add(time.Duration(durationSeconds * float64(time.Second)))

	// Track total operations performed (for result)
	var totalOps uint64
	var mu sync.Mutex

	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()

			var localOps uint64

			for time.Now().Before(endTime) {
				// Work phase: perform CPU-intensive math operations
				workStart := time.Now()
				for time.Since(workStart) < workTime {
					// CPU-intensive floating point operations
					_ = math.Sqrt(math.Pow(float64(localOps), 2) + math.Pow(3.14159, 2))
					_ = math.Sin(float64(localOps)) * math.Cos(float64(localOps))
					localOps++
				}

				// Sleep phase: reduce CPU usage
				if sleepTime > 0 {
					time.Sleep(sleepTime)
				}
			}

			mu.Lock()
			totalOps += localOps
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Return total operations performed as a metric
	return float64(totalOps)
}
