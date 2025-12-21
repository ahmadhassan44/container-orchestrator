package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ahmadhassan44/container-orchestrator/pkg/protocol"
)

// Server handles HTTP requests from clients
type Server struct {
	scheduler *Scheduler
	port      int
}

func NewServer(sched *Scheduler, port int) *Server {
	return &Server{
		scheduler: sched,
		port:      port,
	}
}

// Start begins listening for HTTP requests
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/submit", s.handleSubmit)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/queue", s.handleQueueStatus) // New endpoint for queue status

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("[Gateway] HTTP server listening on %s", addr)

	return http.ListenAndServe(addr, s.loggingMiddleware(mux))
}

// handleSubmit accepts job requests from clients
func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.ComputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.CPULoad <= 0 || req.CPULoad > 100 {
		http.Error(w, "cpu_load must be between 0 and 100", http.StatusBadRequest)
		return
	}
	if req.LoadTime <= 0 {
		http.Error(w, "load_time must be positive", http.StatusBadRequest)
		return
	}

	// Schedule and execute job
	response, err := s.scheduler.ScheduleJob(&req)
	if err != nil {
		log.Printf("[Gateway] Job scheduling failed: %v", err)
		http.Error(w, fmt.Sprintf("Job failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealth provides a simple health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleStatus returns current system status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	workers := s.scheduler.GetWorkerStatus()
	queueStatus := s.scheduler.GetQueueStatus()

	status := map[string]interface{}{
		"status":       "running",
		"worker_count": len(workers),
		"workers":      workers,
		"queue":        queueStatus, // Include queue status
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleQueueStatus returns detailed queue information
func (s *Server) handleQueueStatus(w http.ResponseWriter, r *http.Request) {
	queueStatus := s.scheduler.GetQueueStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(queueStatus)
}

// loggingMiddleware logs all incoming HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[Gateway] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
