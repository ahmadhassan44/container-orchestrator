package main

import (
	"context"
	"log"
	"time"

	"github.com/ahmadhassan44/container-orchestrator/internal/gateway"
	"github.com/ahmadhassan44/container-orchestrator/pkg/config"
)

func main() {
	log.Println("Starting Container Orchestrator Gateway")
	log.Println("========================================")

	ctx := context.Background()

	// Load configuration
	cfg := config.LoadConfig()
	log.Printf("[Config] Max CPU Threshold: %.0f%%", cfg.MaxCPUThreshold)
	log.Printf("[Config] Pre-spawn Threshold: %.0f%%", cfg.PreSpawnThreshold)
	log.Printf("[Config] Gateway Port: %d", cfg.GatewayPort)
	log.Printf("[Config] Initial Workers: %d", cfg.InitialWorkers)

	// Initialize orchestrator
	orch, err := gateway.NewOrchestrator(ctx, cfg.WorkerBasePort)
	if err != nil {
		log.Fatalf("[FATAL] Orchestrator initialization failed: %v", err)
	}

	// Verify Docker connectivity
	orch.CheckConnectivity()

	// Initialize scheduler
	sched := gateway.NewScheduler(orch, cfg) // Spawn initial workers
	log.Printf("[Startup] Spawning %d initial worker(s)", cfg.InitialWorkers)
	for i := 0; i < cfg.InitialWorkers; i++ {
		coreID := i + 1
		if coreID > 3 {
			log.Printf("[Startup] Cannot spawn more than 3 workers (hardware limit)")
			break
		}

		_, err := orch.StartWorker(coreID)
		if err != nil {
			log.Printf("[WARNING] Failed to start initial worker on core %d: %v", coreID, err)
			continue
		}

		// Brief pause to avoid overwhelming Docker daemon
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("[Startup] %d worker(s) ready", orch.GetWorkerCount())
	log.Println("========================================")

	// Start HTTP server
	server := gateway.NewServer(sched, cfg.GatewayPort)
	log.Printf("[Gateway] Ready to accept client connections")

	if err := server.Start(); err != nil {
		log.Fatalf("[FATAL] HTTP server failed: %v", err)
	}
}
