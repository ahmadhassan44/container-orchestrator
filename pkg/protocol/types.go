package protocol

type JobRequest struct {
	LoadEstimate int    `json:"load_estimate"` // e.g., 30 for 30% CPU
	DurationMS   int    `json:"duration_ms"`   // How long to run the load
	Data         string `json:"data"`          // Arbitrary payload
}

type JobResponse struct {
	JobID    string `json:"job_id"`    // Format: ContainerID:UUID
	WorkerID string `json:"worker_id"` // Internal ID (e.g., C1)
	Status   Status `json:"status"`    // "accepted", "queued"
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
