package main

import (
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/ahmadhassan44/container-orchestrator/internal/worker"
)

func main() {
	// 1. HARDWARE ISOLATION CONFIGURATION
	// This is critical. Since we will pin this container to 2 logical threads (e.g., CPU 1 & 5),
	// we tell Go's runtime to only create 2 OS threads for execution.
	runtime.GOMAXPROCS(2)

	// 2. Identity Setup
	workerID := os.Getenv("WORKER_ID")
	if workerID == "" {
		workerID = "Worker-Unknown"
	}

	log.Printf("Starting %s | Cores: %d | GOMAXPROCS: %d",
		workerID, runtime.NumCPU(), runtime.GOMAXPROCS(0))

	// 3. Handler Setup
	h := &worker.WorkerHandler{WorkerID: workerID}
	http.HandleFunc("/submit", h.StartJob)

	// Health check for the Gateway to ping
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 4. Start Server
	// We listen on 8080. The Docker mapping will expose this to unique ports on the host.
	log.Printf("%s listening on port 8080...", workerID)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
