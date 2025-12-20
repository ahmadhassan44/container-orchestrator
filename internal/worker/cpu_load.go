package worker

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

func PerformMonteCarlo(iterations int64, threads int) float64 {
	var wg sync.WaitGroup
	wg.Add(threads)

	chunkSize := iterations / int64(threads)

	// Create a channel to collect results from goroutines
	results := make(chan int64, threads)

	// We generate a base seed once
	baseSeed := time.Now().UnixNano()

	for i := range threads {
		// Capture 'i' for the closure
		threadIndex := int64(i)

		go func() {
			defer wg.Done()

			// FIX: Unique Seed per Goroutine
			// We add the threadIndex to ensure that even if two threads start
			// at the exact same nanosecond, they have different seeds.
			// This guarantees statistically independent random sequences.
			currentSeed := baseSeed + threadIndex
			r := rand.New(rand.NewSource(currentSeed))

			var pointsInsideCircle int64 = 0

			for j := int64(0); j < chunkSize; j++ {
				x := r.Float64()
				y := r.Float64()

				// HEAVY MATH: CPU-bound floating point operations
				distance := math.Sqrt(math.Pow(x, 2) + math.Pow(y, 2))

				if distance <= 1.0 {
					pointsInsideCircle++
				}
			}
			results <- pointsInsideCircle
		}()
	}

	wg.Wait()
	close(results)

	var totalInside int64 = 0
	for count := range results {
		totalInside += count
	}

	// Return the estimated value of Pi
	return 4.0 * float64(totalInside) / float64(iterations)
}
