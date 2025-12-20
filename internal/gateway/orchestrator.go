package gateway

import (
	"context"
	"fmt"
	"log"
	"sync"

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

type Orchestrator struct {
	cli         *client.Client
	ctx         context.Context
	mu          sync.Mutex     // Thread-safe lock
	workerState map[int]string // Map[CoreID] -> ContainerID
}

// NewOrchestrator initializes the Docker client and internal state
func NewOrchestrator(ctx context.Context) (*Orchestrator, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &Orchestrator{
		cli: cli,
		ctx: ctx,
		// Initialize state: All cores (1, 2, 3) are currently empty ("")
		workerState: map[int]string{1: "", 2: "", 3: ""},
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

	// 1. Validation
	currentID, validCore := o.workerState[coreID]
	if !validCore {
		return "", fmt.Errorf("invalid core ID: %d", coreID)
	}
	if currentID != "" {
		return "", fmt.Errorf("core %d is already busy with container %s", coreID, currentID)
	}

	// 2. Topology Lookup
	cpuSet := coreMaps[coreID]
	hostPort := fmt.Sprintf("%d", 8000+coreID) // Core 1 -> Port 8001

	fmt.Printf("⚡ Spawning Worker on Core %d (Threads: %s) -> Port %s\n", coreID, cpuSet, hostPort)

	// 3. Container Config
	config := &container.Config{
		Image: "container-orchestrator-worker:latest",
		Env:   []string{fmt.Sprintf("WORKER_ID=Worker-Core-%d", coreID)},
	}

	// 4. Host Config (THE ISOLATION LOGIC)
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			CpusetCpus: cpuSet, // This is what pins the process to hardware!
		},
		PortBindings: nat.PortMap{
			"8080/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPort}},
		},
	}

	// 5. Create & Start
	resp, err := o.cli.ContainerCreate(o.ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create failed: %w", err)
	}

	if err := o.cli.ContainerStart(o.ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("start failed: %w", err)
	}

	// 6. Update State
	o.workerState[coreID] = resp.ID
	return resp.ID, nil
}
