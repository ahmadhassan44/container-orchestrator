package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/ahmadhassan44/container-orchestrator/pkg/protocol"
)

type WorkerHandler struct {
	WorkerID string
}

func (h *WorkerHandler) StartJob(w http.ResponseWriter, r *http.Request) {
	// 1. Parse the CPU load request
	var req protocol.ComputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 2. Validate request parameters
	if req.CPULoad <= 0 || req.CPULoad > 100 {
		http.Error(w, "cpu_load must be between 0 and 100", http.StatusBadRequest)
		return
	}
	if req.LoadTime <= 0 {
		http.Error(w, "load_time must be positive", http.StatusBadRequest)
		return
	}

	log.Printf("[%s] Starting CPU Load: %.1f%% for %.1fs",
		h.WorkerID, req.CPULoad, req.LoadTime)

	// 3. Execute CPU load simulation
	startTime := time.Now()

	// Dynamically use all assigned threads (e.g., 2)
	numThreads := runtime.GOMAXPROCS(0)

	// Generate CPU load that matches the requested percentage and duration
	result := GenerateCPULoad(req.CPULoad, req.LoadTime, numThreads)

	duration := time.Since(startTime)

	// 4. Return the Scientific Result
	resp := protocol.JobResponse{
		JobID:     fmt.Sprintf("JOB-%d", time.Now().Unix()),
		WorkerID:  h.WorkerID,
		Result:    result,
		TimeTaken: duration.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	log.Printf("[%s] Job Finished in %s. Result: %f", h.WorkerID, duration, result)
}
