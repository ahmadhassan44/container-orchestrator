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
	// 1. Parse the realistic business request
	var req protocol.ComputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 2. Validate "Business Rules"
	if req.Data.Iterations <= 0 {
		http.Error(w, "Iterations must be positive", http.StatusBadRequest)
		return
	}

	log.Printf("[%s] Starting Simulation: %s with N=%d",
		h.WorkerID, req.Operation, req.Data.Iterations)

	// 3. Execute Real Work
	startTime := time.Now()

	// Dynamically use all assigned threads (e.g., 2)
	numThreads := runtime.GOMAXPROCS(0)

	// The CPU load is now governed entirely by 'req.Data.Iterations'
	result := PerformMonteCarlo(req.Data.Iterations, numThreads)

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
