package gateway

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Hardware Topology for i5-1135G7
// Core 0 is reserved for this Gateway/System.
var coreMaps = map[int]string{
	1: "1,5", // Execution Zone A
	2: "2,6", // Execution Zone B
	3: "3,7", // Execution Zone C
}

// WorkerInfo tracks the state and metrics of a running worker container
type WorkerInfo struct {
	CoreID        int
	ContainerID   string
	HostPort      int
	CurrentCPU    float64   // Current CPU usage percentage (0-100)
	LastHeartbeat time.Time // Last successful health check
	IsHealthy     bool
}

type Orchestrator struct {
	cli            *client.Client
	ctx            context.Context
	mu             sync.RWMutex        // Thread-safe lock (RWMutex for better concurrency)
	workers        map[int]*WorkerInfo // Map[CoreID] -> WorkerInfo
	workerBasePort int                 // Base port for workers (e.g., 8000)
}

// NewOrchestrator initializes the Docker client and internal state
func NewOrchestrator(ctx context.Context, basePort int) (*Orchestrator, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &Orchestrator{
		cli:            cli,
		ctx:            ctx,
		workers:        make(map[int]*WorkerInfo),
		workerBasePort: basePort,
	}, nil
}

// CheckConnectivity verifies we can talk to the Docker Daemon
func (o *Orchestrator) CheckConnectivity() {
	info, err := o.cli.Info(o.ctx)
	if err != nil {
		log.Fatalf("CRITICAL: Cannot connect to Docker Daemon. Is it running? %v", err)
	}
	fmt.Printf("✅ Docker Daemon Connected: %s (CPUs: %d)\n", info.Name, info.NCPU)
}

// StartWorker spins up a worker container pinned to a specific physical core
func (o *Orchestrator) StartWorker(coreID int) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Validate core ID
	if _, validCore := coreMaps[coreID]; !validCore {
		return "", fmt.Errorf("invalid core ID: %d (valid: 1, 2, 3)", coreID)
	}

	// Check if core is already occupied
	if worker, exists := o.workers[coreID]; exists {
		return "", fmt.Errorf("core %d already has worker %s", coreID, worker.ContainerID[:12])
	}

	// Topology Lookup
	cpuSet := coreMaps[coreID]
	hostPort := o.workerBasePort + coreID

	log.Printf("[Orchestrator] Spawning worker on Core %d (CPUs: %s, Port: %d)", coreID, cpuSet, hostPort)

	// Container Config
	config := &container.Config{
		Image: "container-orchestrator-worker:latest",
		Env:   []string{fmt.Sprintf("WORKER_ID=Worker-Core-%d", coreID)},
	}

	// Host Config - CPU pinning and port mapping
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			CpusetCpus: cpuSet,
		},
		PortBindings: nat.PortMap{
			"8080/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", hostPort)}},
		},
	}

	// Create container
	resp, err := o.cli.ContainerCreate(o.ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("container creation failed: %w", err)
	}

	// Start container
	if err := o.cli.ContainerStart(o.ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("container start failed: %w", err)
	}

	// Update internal state
	o.workers[coreID] = &WorkerInfo{
		CoreID:        coreID,
		ContainerID:   resp.ID,
		HostPort:      hostPort,
		CurrentCPU:    0.0,
		LastHeartbeat: time.Now(),
		IsHealthy:     true,
	}

	log.Printf("[Orchestrator] Worker started: Core=%d, Container=%s, Port=%d",
		coreID, resp.ID[:12], hostPort)

	return resp.ID, nil
}

// GetWorkerByCore retrieves worker info for a specific core
func (o *Orchestrator) GetWorkerByCore(coreID int) (*WorkerInfo, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	worker, exists := o.workers[coreID]
	return worker, exists
}

// GetAllWorkers returns a snapshot of all active workers
func (o *Orchestrator) GetAllWorkers() []*WorkerInfo {
	o.mu.RLock()
	defer o.mu.RUnlock()

	workers := make([]*WorkerInfo, 0, len(o.workers))
	for _, worker := range o.workers {
		workers = append(workers, worker)
	}
	return workers
}

// UpdateWorkerCPU updates the CPU usage metric for a worker
func (o *Orchestrator) UpdateWorkerCPU(coreID int, cpuPercent float64) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if worker, exists := o.workers[coreID]; exists {
		worker.CurrentCPU = cpuPercent
		worker.LastHeartbeat = time.Now()
	}
}

// GetNextAvailableCore finds the first unoccupied core
func (o *Orchestrator) GetNextAvailableCore() (int, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	for coreID := 1; coreID <= 3; coreID++ {
		if _, exists := o.workers[coreID]; !exists {
			return coreID, nil
		}
	}

	return 0, fmt.Errorf("no available cores (all 3 cores occupied)")
}

// GetWorkerCount returns the number of active workers
func (o *Orchestrator) GetWorkerCount() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.workers)
}
