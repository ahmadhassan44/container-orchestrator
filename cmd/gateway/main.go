package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	log.Println("Starting Orchestrator Gateway on Core 0 (Management Zone)...")

	// TODO: Initialize Docker Client
	// TODO: Initialize Scheduler Map

	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Gateway: Submit endpoint")
	})

	// Listen on port 8000
	log.Fatal(http.ListenAndServe(":8000", nil))
}
