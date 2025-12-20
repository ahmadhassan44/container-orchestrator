package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ahmadhassan44/container-orchestrator/pkg/config"
	"github.com/ahmadhassan44/container-orchestrator/pkg/protocol"
)

// Scheduler handles intelligent job routing and load balancing
type Scheduler struct {
	orchestrator *Orchestrator
	estimator    *CPUEstimator
	config       *config.Config
	httpClient   *http.Client
}

func NewScheduler(orch *Orchestrator, cfg *config.Config) *Scheduler {
	return &Scheduler{
		orchestrator: orch,
		estimator:    NewCPUEstimator(),
		config:       cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ScheduleJob finds the best worker for a job or spawns a new one if needed
func (s *Scheduler) ScheduleJob(req *protocol.ComputeRequest) (*protocol.JobResponse, error) {
	estimatedCPU := s.estimator.EstimateCPUUsage(req)
	loadTime := s.estimator.EstimateJobDuration(req)

	log.Printf("[Scheduler] Job request: cpu_load=%.1f%%, load_time=%.1fs",
		estimatedCPU, loadTime)

	// Try to find a suitable existing worker
	worker := s.findSuitableWorker(estimatedCPU)

	if worker == nil {
		// No suitable worker found, try to spawn a new one
		log.Printf("[Scheduler] No suitable worker found, attempting to spawn new worker")

		coreID, err := s.orchestrator.GetNextAvailableCore()
		if err != nil {
			return nil, fmt.Errorf("cannot spawn worker: %w", err)
		}

		if _, err := s.orchestrator.StartWorker(coreID); err != nil {
			return nil, fmt.Errorf("failed to start worker on core %d: %w", coreID, err)
		}

		// Wait briefly for worker to initialize
		time.Sleep(2 * time.Second)

		worker, _ = s.orchestrator.GetWorkerByCore(coreID)
		if worker == nil {
			return nil, fmt.Errorf("worker spawned but not found in state")
		}
	}

	// Update projected CPU usage
	s.orchestrator.UpdateWorkerCPU(worker.CoreID, worker.CurrentCPU+estimatedCPU)

	log.Printf("[Scheduler] Routing job to Worker-Core-%d (port %d, current_cpu=%.1f%%)",
		worker.CoreID, worker.HostPort, worker.CurrentCPU)

	// Execute job on selected worker
	response, err := s.executeJobOnWorker(worker, req)
	if err != nil {
		// Restore CPU usage on failure
		s.orchestrator.UpdateWorkerCPU(worker.CoreID, worker.CurrentCPU-estimatedCPU)
		return nil, err
	}

	// After job completion, decay CPU usage (job is done)
	s.orchestrator.UpdateWorkerCPU(worker.CoreID, worker.CurrentCPU-estimatedCPU)

	// Check if we need to proactively spawn another worker
	s.checkProactiveSpawn()

	return response, nil
}

// findSuitableWorker locates a worker that can handle the estimated CPU load
func (s *Scheduler) findSuitableWorker(estimatedCPU float64) *WorkerInfo {
	workers := s.orchestrator.GetAllWorkers()

	if len(workers) == 0 {
		return nil
	}

	// Strategy: Find worker with lowest current CPU that can accommodate the request
	var bestWorker *WorkerInfo
	var lowestCPU float64 = 101.0 // Start above 100%

	for _, worker := range workers {
		projectedCPU := worker.CurrentCPU + estimatedCPU

		// Check if this worker can handle the load without exceeding threshold
		if projectedCPU <= s.config.MaxCPUThreshold {
			if worker.CurrentCPU < lowestCPU {
				lowestCPU = worker.CurrentCPU
				bestWorker = worker
			}
		}
	}

	return bestWorker
}

// executeJobOnWorker sends the job request to a specific worker via HTTP
func (s *Scheduler) executeJobOnWorker(worker *WorkerInfo, req *protocol.ComputeRequest) (*protocol.JobResponse, error) {
	url := fmt.Sprintf("http://localhost:%d/submit", worker.HostPort)

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("worker communication failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("worker returned status %d", resp.StatusCode)
	}

	var jobResp protocol.JobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[Scheduler] Job completed: job_id=%s, worker=%s, result=%.6f, duration=%s",
		jobResp.JobID, jobResp.WorkerID, jobResp.Result, jobResp.TimeTaken)

	return &jobResp, nil
}

// checkProactiveSpawn spawns a new worker if all active workers are near threshold
func (s *Scheduler) checkProactiveSpawn() {
	workers := s.orchestrator.GetAllWorkers()

	if len(workers) == 0 {
		return
	}

	// Check if all workers are above pre-spawn threshold
	allBusy := true
	for _, worker := range workers {
		if worker.CurrentCPU < s.config.PreSpawnThreshold {
			allBusy = false
			break
		}
	}

	if !allBusy {
		return
	}

	// All workers are busy, try to spawn another
	coreID, err := s.orchestrator.GetNextAvailableCore()
	if err != nil {
		log.Printf("[Scheduler] Proactive spawn skipped: %v", err)
		return
	}

	log.Printf("[Scheduler] All workers above %.0f%% threshold, proactively spawning worker on Core %d",
		s.config.PreSpawnThreshold, coreID)

	if _, err := s.orchestrator.StartWorker(coreID); err != nil {
		log.Printf("[Scheduler] Proactive spawn failed: %v", err)
	}
}

// GetWorkerStatus returns current status of all workers (for status endpoint)
func (s *Scheduler) GetWorkerStatus() []map[string]interface{} {
	workers := s.orchestrator.GetAllWorkers()

	status := make([]map[string]interface{}, 0, len(workers))
	for _, worker := range workers {
		status = append(status, map[string]interface{}{
			"core_id":      worker.CoreID,
			"container_id": worker.ContainerID[:12],
			"host_port":    worker.HostPort,
			"cpu_usage":    fmt.Sprintf("%.1f%%", worker.CurrentCPU),
			"is_healthy":   worker.IsHealthy,
		})
	}

	return status
}
