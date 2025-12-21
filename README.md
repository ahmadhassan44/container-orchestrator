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
- **Job Queuing**: Optional FIFO queue for jobs when all workers are busy (see [JOB_QUEUE_README.md](JOB_QUEUE_README.md))
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
# Low CPU load (25% for 3 seconds)
curl -X POST http://localhost:3000/submit \
  -H "Content-Type: application/json" \
  -d '{
    "cpu_load": 25,
    "load_time": 3
  }'

# Medium CPU load (50% for 5 seconds)
curl -X POST http://localhost:3000/submit \
  -H "Content-Type: application/json" \
  -d '{
    "cpu_load": 50,
    "load_time": 5
  }'

# High CPU load (80% for 10 seconds)
curl -X POST http://localhost:3000/submit \
  -H "Content-Type: application/json" \
  -d '{
    "cpu_load": 80,
    "load_time": 10
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

### CPU Load Model

The system uses a **direct CPU load specification** model:

- **Client specifies exact CPU percentage** (0-100): Target CPU utilization
- **Client specifies load duration** (seconds): How long to sustain the load
- **Worker generates synthetic load**: Uses work/sleep cycles to match requested percentage
- **Accurate scheduling**: Scheduler directly uses client-specified values for routing decisions

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

Submit a CPU load job.

**Request:**

```json
{
  "cpu_load": 50,
  "load_time": 5
}
```

- `cpu_load`: Target CPU utilization percentage (0-100)
- `load_time`: Duration in seconds to sustain the load

**Response:**

```json
{
  "job_id": "JOB-1734739200",
  "worker_id": "Worker-Core-1",
  "result": 125000000,
  "time_taken": "5.01s"
}
```

- `result`: Total operations performed (metric)

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

### Customizing CPU Load Generation

The CPU load generator in `internal/worker/cpu_load.go` uses work/sleep cycles:

1. **Work phase**: Performs CPU-intensive math operations
2. **Sleep phase**: Reduces CPU to achieve target percentage
3. **Quantum**: 10ms time slices for smooth load distribution

To modify load characteristics, adjust the `GenerateCPULoad()` function.

## License

MIT
