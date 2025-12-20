package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ahmadhassan44/container-orchestrator/internal/gateway"
)

func main() {
	fmt.Println("Starting Orchestrator Gateway...")

	ctx := context.Background()

	// 1. Initialize
	orch, err := gateway.NewOrchestrator(ctx)
	if err != nil {
		log.Fatalf("Failed to init: %v", err)
	}
	orch.CheckConnectivity()

	// 2. SMOKE TEST: Spawn on Core 1
	fmt.Println("------------------------------------------------")
	fmt.Println("TEST: Attempting to claim Execution Zone A (Core 1)...")

	containerID, err := orch.StartWorker(1)
	if err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	fmt.Printf("SUCCESS! Worker running. Container ID: %s\n", containerID[:12])
	fmt.Println("------------------------------------------------")
	fmt.Println("The container will remain active for 60 seconds so you can inspect it.")
	fmt.Println("Run this in another terminal to verify isolation:")
	fmt.Printf(">> docker inspect %s | grep Cpuset\n", containerID[:12])

	// Keep process alive so we don't exit immediately
	time.Sleep(60 * time.Second)
}
