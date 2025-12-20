package protocol

type ComputeRequest struct {
	// CPULoad is the target CPU usage percentage (0-100)
	// Example: 50 means 50% CPU utilization
	CPULoad float64 `json:"cpu_load"`

	// LoadTime is how long the CPU should be loaded (in seconds)
	// Example: 5.0 means sustain the load for 5 seconds
	LoadTime float64 `json:"load_time"`
}

type JobParameters struct {
	// Deprecated: kept for backwards compatibility
	Iterations int64 `json:"iterations,omitempty"`
	Seed       int64 `json:"seed,omitempty"`
}

type JobResponse struct {
	JobID     string  `json:"job_id"`
	WorkerID  string  `json:"worker_id"`
	Result    float64 `json:"result"`     // The actual math answer
	TimeTaken string  `json:"time_taken"` // "1.24s"
}

type Status int

const (
	StatusAccepted Status = iota
	StatusQueued
	StatusInProgress
	StatusCompleted
	StatusFailed
)

type JobStatus struct {
	JobID      string `json:"job_id"`
	Percentage int    `json:"percentage_complete"`
	Result     string `json:"result,omitempty"`
}
