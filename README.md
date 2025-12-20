# Container Orchestrator

A high-performance container orchestration system with CPU-aware load balancing and automatic scaling.

## Architecture

### Components

1. **Gateway** - HTTP server accepting client requests on port 3000
2. **Scheduler** - Intelligent load balancer with CPU estimation
3. **Orchestrator** - Docker container lifecycle management
4. **Workers** - CPU-pinned containers executing compute jobs

### Features

- **Hardware Isolation**: Workers pinned to specific CPU cores (i5-1135G7 topology)
- **Smart Scheduling**: Routes jobs based on estimated CPU usage
- **Configurable Thresholds**: Control max CPU usage per worker
- **Auto-scaling**: Spawns workers on-demand when load increases
- **Proactive Spawning**: Pre-spawns containers when all workers approach threshold
- **Clean Logging**: Structured, informative logs without clutter

## Configuration

Set via environment variables:

```bash
MAX_CPU_THRESHOLD=80        # Don't schedule if worker exceeds this (default: 80%)
PRESPAWN_THRESHOLD=70       # Spawn new worker when all exceed this (default: 70%)
GATEWAY_PORT=3000           # HTTP server port (default: 3000)
INITIAL_WORKERS=1           # Workers to spawn on startup (default: 1)
```

## Usage

### Build and Start

```bash
chmod +x start.sh
./start.sh
```

Or manually:

```bash
# Build worker image
docker build -f Dockerfile.worker -t container-orchestrator-worker:latest .

# Start gateway
go run ./cmd/gateway/main.go
```

### Submit Jobs

```bash
# Small job (quick burst)
curl -X POST http://localhost:3000/submit \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "monte_carlo_pi",
    "data": {
      "iterations": 1000000
    }
  }'

# Medium job (moderate CPU)
curl -X POST http://localhost:3000/submit \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "monte_carlo_pi",
    "data": {
      "iterations": 100000000
    }
  }'

# Large job (heavy CPU)
curl -X POST http://localhost:3000/submit \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "monte_carlo_pi",
    "data": {
      "iterations": 10000000000
    }
  }'
```

### Check Status

```bash
curl http://localhost:3000/status
```

### Health Check

```bash
curl http://localhost:3000/health
```

## How It Works

### Job Submission Flow

1. Client sends POST to `/submit` with operation and parameters
2. Scheduler estimates CPU usage based on algorithm and iterations
3. Scheduler finds worker with lowest CPU usage that can handle the job
4. If no suitable worker exists and cores are available, spawns new worker
5. Job is routed to selected worker via HTTP
6. Worker executes job and returns result
7. Scheduler updates CPU tracking and checks for proactive spawn

### CPU Estimation

The scheduler estimates CPU usage based on:

- **Algorithm type**: Currently supports Monte Carlo Pi calculation
- **Iteration count**: More iterations = higher CPU usage
- **Expected duration**: Sub-second jobs = low CPU, multi-second jobs = high CPU

### Load Balancing Strategy

1. **Check existing workers**: Find worker with lowest current CPU
2. **Validate threshold**: Ensure projected CPU stays below `MAX_CPU_THRESHOLD`
3. **Spawn if needed**: Create new worker if no suitable worker found
4. **Proactive scaling**: Pre-spawn when all workers exceed `PRESPAWN_THRESHOLD`

### Hardware Topology (i5-1135G7)

- Core 0: Reserved for Gateway/System
- Core 1 (threads 1,5): Execution Zone A → Port 8001
- Core 2 (threads 2,6): Execution Zone B → Port 8002
- Core 3 (threads 3,7): Execution Zone C → Port 8003

## API Reference

### POST /submit

Submit a compute job.

**Request:**

```json
{
  "operation": "monte_carlo_pi",
  "data": {
    "iterations": 1000000
  }
}
```

**Response:**

```json
{
  "job_id": "JOB-1734739200",
  "worker_id": "Worker-Core-1",
  "result": 3.141592,
  "time_taken": "1.24s"
}
```

### GET /status

Get current system status.

**Response:**

```json
{
  "status": "running",
  "worker_count": 2,
  "workers": [
    {
      "core_id": 1,
      "container_id": "c8acf2fb2714",
      "host_port": 8001,
      "cpu_usage": "45.2%",
      "is_healthy": true
    }
  ]
}
```

### GET /health

Simple health check (returns "OK").

## Development

### Project Structure

```
container-orchestrator/
├── cmd/
│   ├── gateway/        # Gateway entry point
│   └── worker/         # Worker entry point
├── internal/
│   ├── gateway/        # Orchestrator and HTTP server
│   ├── scheduler/      # Load balancing and CPU estimation
│   └── worker/         # Job handlers and CPU-intensive algorithms
├── pkg/
│   ├── config/         # Configuration management
│   └── protocol/       # Shared types and protocols
├── Dockerfile.worker   # Worker container image
└── start.sh           # Build and run script
```

### Adding New Algorithms

1. Add case to `EstimateCPUUsage()` in `internal/scheduler/estimator.go`
2. Implement handler in `internal/worker/handler.go`
3. Update protocol types if needed in `pkg/protocol/types.go`

## License

MIT
