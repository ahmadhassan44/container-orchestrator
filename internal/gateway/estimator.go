package gateway

import (
	"github.com/ahmadhassan44/container-orchestrator/pkg/protocol"
)

// CPUEstimator calculates expected CPU usage for different operations
type CPUEstimator struct {
	// No calibration needed - we use client-specified CPU load directly
}

func NewCPUEstimator() *CPUEstimator {
	return &CPUEstimator{}
}

// EstimateCPUUsage returns expected CPU percentage (0-100) for a given request
// Now directly uses the client-specified cpu_load value
func (e *CPUEstimator) EstimateCPUUsage(req *protocol.ComputeRequest) float64 {
	// Validate CPU load is within bounds
	if req.CPULoad < 0 {
		return 0.0
	}
	if req.CPULoad > 100 {
		return 100.0
	}
	return req.CPULoad
}

// EstimateJobDuration returns expected execution time in seconds
func (e *CPUEstimator) EstimateJobDuration(req *protocol.ComputeRequest) float64 {
	if req.LoadTime < 0 {
		return 0.0
	}
	return req.LoadTime
}
