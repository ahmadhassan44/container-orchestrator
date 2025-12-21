package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ahmadhassan44/container-orchestrator/pkg/config"
	"github.com/ahmadhassan44/container-orchestrator/pkg/protocol"
)

// ============================================================================
// JOB QUEUING FEATURE - Can be enabled/disabled by setting ENABLE_JOB_QUEUE
// ============================================================================
const (
	ENABLE_JOB_QUEUE = true  // Set to false to disable job queuing
	MAX_QUEUE_SIZE   = 100   // Maximum number of queued jobs
	QUEUE_TIMEOUT    = 30    // Seconds to wait in queue before giving up
)

// QueuedJob represents a job waiting to be scheduled
type QueuedJob struct {
	request     *protocol.ComputeRequest
	responseCh  chan *protocol.JobResponse
	errorCh     chan error
	enqueuedAt  time.Time
	estimatedCPU float64
}

// Scheduler handles intelligent job routing and load balancing
type Scheduler struct {
	orchestrator *Orchestrator
	estimator    *CPUEstimator
	config       *config.Config
	httpClient   *http.Client
	scheduleMux  sync.Mutex // Prevents race conditions in concurrent scheduling
	
	// Job Queue (can be disabled by setting ENABLE_JOB_QUEUE = false)
	jobQueue     chan *QueuedJob
	queueWorkerStop chan struct{}
}

func NewScheduler(orch *Orchestrator, cfg *config.Config) *Scheduler {
	s := &Scheduler{
		orchestrator: orch,
		estimator:    NewCPUEstimator(),
		config:       cfg,
		httpClient:   &http.Client{}, // Timeout set per request
	}
	
	// Initialize job queue if enabled
	if ENABLE_JOB_QUEUE {
		s.jobQueue = make(chan *QueuedJob, MAX_QUEUE_SIZE)
		s.queueWorkerStop = make(chan struct{})
		go s.processJobQueue()
		log.Printf("[Scheduler] Job queuing ENABLED (max queue size: %d, timeout: %ds)", 
			MAX_QUEUE_SIZE, QUEUE_TIMEOUT)
	}
	
	return s
}

// ScheduleJob finds the best worker for a job or spawns a new one if needed
func (s *Scheduler) ScheduleJob(req *protocol.ComputeRequest) (*protocol.JobResponse, error) {
	estimatedCPU := s.estimator.EstimateCPUUsage(req)
	loadTime := s.estimator.EstimateJobDuration(req)

	log.Printf("[Scheduler] Job request: cpu_load=%.1f%%, load_time=%.1fs",
		estimatedCPU, loadTime)

	// ========================================================================
	// JOB QUEUING: If enabled, try to queue job when all workers are busy
	// ========================================================================
	if ENABLE_JOB_QUEUE {
		return s.scheduleJobWithQueue(req, estimatedCPU, loadTime)
	}
	
	// Original scheduling logic (without queuing)
	return s.scheduleJobDirect(req, estimatedCPU, loadTime)
}

// scheduleJobDirect handles immediate scheduling without queuing
func (s *Scheduler) scheduleJobDirect(req *protocol.ComputeRequest, estimatedCPU, loadTime float64) (*protocol.JobResponse, error) {
	// Lock to prevent race conditions when multiple jobs arrive simultaneously
	s.scheduleMux.Lock()

	// Try to find a suitable existing worker
	worker := s.findSuitableWorker(estimatedCPU)

	if worker == nil {
		// No suitable worker found, try to spawn a new one
		log.Printf("[Scheduler] No suitable worker found, attempting to spawn new worker")

		coreID, err := s.orchestrator.GetNextAvailableCore()
		if err != nil {
			s.scheduleMux.Unlock()
			return nil, fmt.Errorf("cannot spawn worker: %w", err)
		}

		if _, err := s.orchestrator.StartWorker(coreID); err != nil {
			s.scheduleMux.Unlock()
			return nil, fmt.Errorf("failed to start worker on core %d: %w", coreID, err)
		}

		// Wait briefly for worker to initialize
		time.Sleep(2 * time.Second)

		worker, _ = s.orchestrator.GetWorkerByCore(coreID)
		if worker == nil {
			s.scheduleMux.Unlock()
			return nil, fmt.Errorf("worker spawned but not found in state")
		}
	}

	// Update projected CPU usage BEFORE releasing lock
	s.orchestrator.UpdateWorkerCPU(worker.CoreID, worker.CurrentCPU+estimatedCPU)

	// Release lock - worker is now reserved for this job
	s.scheduleMux.Unlock()

	log.Printf("[Scheduler] Routing job to Worker-Core-%d (port %d, current_cpu=%.1f%%)",
		worker.CoreID, worker.HostPort, worker.CurrentCPU)

	// Execute job on selected worker
	response, err := s.executeJobOnWorker(worker, req)
	if err != nil {
		// Restore CPU usage on failure (ensure it doesn't go negative)
		newCPU := worker.CurrentCPU - estimatedCPU
		if newCPU < 0 {
			newCPU = 0
		}
		s.orchestrator.UpdateWorkerCPU(worker.CoreID, newCPU)
		return nil, err
	}

	// After job completion, decay CPU usage (job is done)
	// The UpdateWorkerCPU function will ensure it doesn't go below 0
	s.orchestrator.UpdateWorkerCPU(worker.CoreID, worker.CurrentCPU-estimatedCPU)

	// Check if we need to proactively spawn another worker
	s.checkProactiveSpawn()

	return response, nil
}

// ============================================================================
// JOB QUEUING IMPLEMENTATION - Can be removed if ENABLE_JOB_QUEUE = false
// ============================================================================

// scheduleJobWithQueue attempts immediate scheduling, or queues if all workers busy
func (s *Scheduler) scheduleJobWithQueue(req *protocol.ComputeRequest, estimatedCPU, loadTime float64) (*protocol.JobResponse, error) {
	// Try immediate scheduling first
	s.scheduleMux.Lock()
	worker := s.findSuitableWorker(estimatedCPU)
	
	if worker == nil {
		// Try to spawn a new worker
		coreID, err := s.orchestrator.GetNextAvailableCore()
		if err == nil {
			// Can spawn a worker
			if _, startErr := s.orchestrator.StartWorker(coreID); startErr == nil {
				time.Sleep(2 * time.Second)
				worker, _ = s.orchestrator.GetWorkerByCore(coreID)
			}
		}
	}
	
	if worker != nil {
		// Found a worker - schedule immediately
		s.orchestrator.UpdateWorkerCPU(worker.CoreID, worker.CurrentCPU+estimatedCPU)
		s.scheduleMux.Unlock()
		
		log.Printf("[Scheduler] Routing job to Worker-Core-%d (port %d, current_cpu=%.1f%%)",
			worker.CoreID, worker.HostPort, worker.CurrentCPU)
		
		response, err := s.executeJobOnWorker(worker, req)
		s.orchestrator.UpdateWorkerCPU(worker.CoreID, worker.CurrentCPU-estimatedCPU)
		s.checkProactiveSpawn()
		return response, err
	}
	
	// No worker available - queue the job
	s.scheduleMux.Unlock()
	log.Printf("[Scheduler] All workers busy, queueing job (cpu_load=%.1f%%)", estimatedCPU)
	
	queuedJob := &QueuedJob{
		request:      req,
		responseCh:   make(chan *protocol.JobResponse, 1),
		errorCh:      make(chan error, 1),
		enqueuedAt:   time.Now(),
		estimatedCPU: estimatedCPU,
	}
	
	select {
	case s.jobQueue <- queuedJob:
		// Job queued successfully, wait for response
		select {
		case response := <-queuedJob.responseCh:
			return response, nil
		case err := <-queuedJob.errorCh:
			return nil, err
		case <-time.After(time.Duration(QUEUE_TIMEOUT) * time.Second):
			return nil, fmt.Errorf("job timed out in queue after %ds", QUEUE_TIMEOUT)
		}
	default:
		return nil, fmt.Errorf("job queue full (max size: %d), cannot accept job", MAX_QUEUE_SIZE)
	}
}

// processJobQueue continuously processes queued jobs
func (s *Scheduler) processJobQueue() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	log.Printf("[Scheduler] Queue processor started")
	
	for {
		select {
		case <-s.queueWorkerStop:
			log.Printf("[Scheduler] Queue processor stopping")
			return
			
		case <-ticker.C:
			// Try to process pending jobs
			s.tryProcessQueue()
		}
	}
}

// tryProcessQueue attempts to assign queued jobs to available workers
func (s *Scheduler) tryProcessQueue() {
	// Process multiple jobs if multiple workers are available
	for {
		// Non-blocking check if queue has jobs
		select {
		case queuedJob := <-s.jobQueue:
			// Check if job has timed out
			if time.Since(queuedJob.enqueuedAt) > time.Duration(QUEUE_TIMEOUT)*time.Second {
				log.Printf("[Scheduler] Queue job timed out, discarding")
				queuedJob.errorCh <- fmt.Errorf("job expired in queue")
				continue // Try next job in queue
			}
			
			// Try to schedule the queued job
			s.scheduleMux.Lock()
			worker := s.findSuitableWorker(queuedJob.estimatedCPU)
			
			if worker != nil {
				// Worker available - schedule it
				s.orchestrator.UpdateWorkerCPU(worker.CoreID, worker.CurrentCPU+queuedJob.estimatedCPU)
				s.scheduleMux.Unlock()
				
				waitTime := time.Since(queuedJob.enqueuedAt)
				log.Printf("[Scheduler] Dequeued job (waited %.1fs) → Worker-Core-%d", 
					waitTime.Seconds(), worker.CoreID)
				
				// Execute job asynchronously so we can process more queue items
				go func(w *WorkerInfo, job *QueuedJob) {
					response, err := s.executeJobOnWorker(w, job.request)
					s.orchestrator.UpdateWorkerCPU(w.CoreID, w.CurrentCPU-job.estimatedCPU)
					
					if err != nil {
						job.errorCh <- err
					} else {
						job.responseCh <- response
					}
					
					s.checkProactiveSpawn()
				}(worker, queuedJob)
				
				// Continue to next queued job immediately
				continue
			} else {
			// Still no worker available - put job back and stop processing this tick
			s.scheduleMux.Unlock()
			
			// Try to put job back at front of queue
			go func(job *QueuedJob) {
				select {
				case s.jobQueue <- job:
					// Requeued successfully
				case <-time.After(1 * time.Second):
					// Couldn't requeue - fail the job
					job.errorCh <- fmt.Errorf("failed to requeue job")
				}
			}(queuedJob)
			
			return // Stop processing this tick
		}
		
		default:
			// No more jobs in queue
			return
		}
	}
}

// StopQueueProcessor stops the queue processing goroutine (call on shutdown)
func (s *Scheduler) StopQueueProcessor() {
	if ENABLE_JOB_QUEUE {
		close(s.queueWorkerStop)
		log.Printf("[Scheduler] Queue processor stopped")
	}
}

// GetQueueStatus returns current queue statistics
func (s *Scheduler) GetQueueStatus() map[string]interface{} {
	if !ENABLE_JOB_QUEUE {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	
	return map[string]interface{}{
		"enabled":    true,
		"queue_size": len(s.jobQueue),
		"max_size":   MAX_QUEUE_SIZE,
		"timeout":    QUEUE_TIMEOUT,
	}
}

// ============================================================================
// END OF JOB QUEUING IMPLEMENTATION
// ============================================================================

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

	// Set dynamic timeout: job duration + 10 second buffer for overhead
	jobTimeout := time.Duration(req.LoadTime)*time.Second + 10*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
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
