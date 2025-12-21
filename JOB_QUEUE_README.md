# Job Queuing Feature

## Overview

The job queuing feature allows jobs to wait in a queue when all workers are busy, instead of being immediately rejected. This improves job acceptance rates and system utilization.

## Configuration

Edit `/internal/gateway/scheduler.go`:

```go
const (
    ENABLE_JOB_QUEUE = true  // Set to false to disable job queuing
    MAX_QUEUE_SIZE   = 100   // Maximum number of queued jobs
    QUEUE_TIMEOUT    = 30    // Seconds to wait in queue before giving up
)
```

## How It Works

### With Job Queuing (ENABLE_JOB_QUEUE = true):

1. **Job arrives** → Scheduler tries to find available worker
2. **No worker available** → Job is added to queue
3. **Background processor** checks queue every 500ms
4. **Worker becomes available** → Queued job is assigned
5. **Timeout protection** → Jobs expire after QUEUE_TIMEOUT seconds

### Without Job Queuing (ENABLE_JOB_QUEUE = false):

1. **Job arrives** → Scheduler tries to find available worker
2. **No worker available** → Job is immediately rejected with error
3. **Client must retry** manually

## Benefits

✅ **Higher job acceptance rate** - Jobs wait instead of being rejected  
✅ **Better resource utilization** - Workers stay busy processing queued jobs  
✅ **Automatic retries** - No need for client-side retry logic  
✅ **Fair scheduling** - FIFO queue ensures fairness  

## API Changes

### New Endpoint: `/queue`

Get current queue status:

```bash
curl http://localhost:3000/queue
```

Response:
```json
{
  "enabled": true,
  "queue_size": 5,
  "max_size": 100,
  "timeout": 30
}
```

### Updated Endpoint: `/status`

Now includes queue information:

```json
{
  "status": "running",
  "worker_count": 3,
  "workers": [...],
  "queue": {
    "enabled": true,
    "queue_size": 2,
    "max_size": 100,
    "timeout": 30
  }
}
```

## Log Messages

### With Queuing Enabled:
```
[Scheduler] Job queuing ENABLED (max queue size: 100, timeout: 30s)
[Scheduler] All workers busy, queueing job (cpu_load=60.0%)
[Scheduler] Dequeued job (waited 2.3s) → Worker-Core-2
```

### With Queuing Disabled:
```
(No queue-related messages)
[Gateway] Job scheduling failed: cannot spawn worker: no available cores
```

## Testing

### Test Queue Behavior:

1. Start gateway with queuing enabled
2. Submit many concurrent jobs to saturate all workers:

```bash
# Submit 10 concurrent jobs (will exceed capacity)
for i in {1..10}; do
  curl -X POST http://localhost:3000/submit \
    -H "Content-Type: application/json" \
    -d '{"cpu_load": 70, "load_time": 10}' &
done
wait
```

3. Check queue status:
```bash
curl http://localhost:3000/queue
```

### Expected Behavior:

- **With Queue**: All 10 jobs accepted, 3 execute immediately, 7 wait in queue
- **Without Queue**: 3 jobs succeed, 7 jobs rejected with errors

## Disabling Job Queuing

To disable (revert to original behavior):

1. Edit `internal/gateway/scheduler.go`
2. Change: `const ENABLE_JOB_QUEUE = false`
3. Rebuild: `./start.sh`

The queue code remains in place but is not executed, making it easy to re-enable later.

## Performance Considerations

- **Queue overhead**: Minimal (500ms polling interval)
- **Memory usage**: ~1KB per queued job
- **Max queue size**: 100 jobs (configurable)
- **Timeout**: 30 seconds (configurable)

## Troubleshooting

### Jobs timing out in queue:
- Increase `QUEUE_TIMEOUT` constant
- Reduce job execution time
- Increase `MAX_CPU_THRESHOLD` to allow more concurrent jobs per worker

### Queue filling up:
- Increase `MAX_QUEUE_SIZE` constant
- Add more workers (increase hardware capacity)
- Optimize job execution time

### Queue not processing:
- Check logs for "Queue processor started" message
- Verify `ENABLE_JOB_QUEUE = true`
- Check for errors in queue processor

## Code Structure

```
scheduler.go
├── Constants (ENABLE_JOB_QUEUE, MAX_QUEUE_SIZE, QUEUE_TIMEOUT)
├── QueuedJob struct
├── Scheduler struct (with jobQueue channel)
├── NewScheduler() - Initializes queue if enabled
├── ScheduleJob() - Routes to queue or direct scheduling
├── scheduleJobDirect() - Original non-queuing logic
├── scheduleJobWithQueue() - Queuing logic
├── processJobQueue() - Background queue processor
├── tryProcessQueue() - Attempts to schedule queued jobs
├── StopQueueProcessor() - Cleanup on shutdown
└── GetQueueStatus() - Returns queue statistics
```

## Future Enhancements

Possible improvements (not yet implemented):

- **Priority queuing**: High-priority jobs jump the queue
- **Job cancellation**: API to cancel queued jobs
- **Queue persistence**: Survive gateway restarts
- **Advanced metrics**: Queue wait times, throughput statistics
- **Dynamic timeout**: Adjust timeout based on queue depth
