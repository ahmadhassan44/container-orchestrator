package gateway

import (
	"math"

	"github.com/ahmadhassan44/container-orchestrator/pkg/protocol"
)

// CPUEstimator calculates expected CPU usage for different operations
type CPUEstimator struct {
	// Calibration constants based on empirical testing
	// These define how many iterations per second a single core can handle
	monteCarloOpsPerSecond float64
}

func NewCPUEstimator() *CPUEstimator {
	return &CPUEstimator{
		// Benchmark: a modern CPU core can handle ~50M Monte Carlo iterations/sec
		// This is conservative and should be calibrated to actual hardware
		monteCarloOpsPerSecond: 50_000_000,
	}
}

// EstimateCPUUsage returns expected CPU percentage (0-100) for a given request
// This assumes the worker has 2 threads (1 physical core with hyperthreading)
func (e *CPUEstimator) EstimateCPUUsage(req *protocol.ComputeRequest) float64 {
	switch req.Operation {
	case "monte_carlo_pi":
		return e.estimateMonteCarlo(req.Data.Iterations)
	default:
		// Unknown operation: assume conservative 50% usage
		return 50.0
	}
}

func (e *CPUEstimator) estimateMonteCarlo(iterations int64) float64 {
	if iterations <= 0 {
		return 0.0
	}

	// Calculate expected execution time
	expectedSeconds := float64(iterations) / e.monteCarloOpsPerSecond

	// Each worker has 2 threads, so it can utilize up to 200% of a single core
	// We model CPU usage based on expected duration and thread utilization
	// For simplicity:
	// - Small jobs (< 1 sec): low CPU burst
	// - Medium jobs (1-5 sec): moderate CPU
	// - Large jobs (> 5 sec): high sustained CPU

	var cpuPercent float64
	switch {
	case expectedSeconds < 1.0:
		// Quick burst: 10-30% average
		cpuPercent = 10.0 + (expectedSeconds * 20.0)
	case expectedSeconds < 5.0:
		// Medium load: 30-60%
		cpuPercent = 30.0 + ((expectedSeconds - 1.0) * 7.5)
	default:
		// Heavy load: 60-95%
		cpuPercent = 60.0 + math.Min((expectedSeconds-5.0)*5.0, 35.0)
	}

	return math.Min(cpuPercent, 100.0)
}

// EstimateJobDuration returns expected execution time in seconds
func (e *CPUEstimator) EstimateJobDuration(req *protocol.ComputeRequest) float64 {
	switch req.Operation {
	case "monte_carlo_pi":
		return float64(req.Data.Iterations) / e.monteCarloOpsPerSecond
	default:
		return 1.0
	}
}
