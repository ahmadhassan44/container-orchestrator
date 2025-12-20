package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
)

func main() {
	// Hard constraint: Maximize usage of the 2 assigned logical threads
	runtime.GOMAXPROCS(2)

	workerID := os.Getenv("WORKER_ID")
	if workerID == "" {
		workerID = "UNKNOWN"
	}

	log.Printf("Starting Worker %s on allocated Core...", workerID)

	http.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Worker %s: Ready to process", workerID)
	})

	// Listen on port 8080 (Docker will map this internally)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
