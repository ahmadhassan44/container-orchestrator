package protocol

type ComputeRequest struct {
	// Operation tells the worker which algorithm to run.
	// e.g., "monte_carlo_pi", "prime_search", "matrix_determinant"
	Operation string `json:"operation"`

	// Data contains the parameters that govern the CPU complexity.
	Data JobParameters `json:"data"`
}

type JobParameters struct {
	// Iterations is the "N".
	// A value of 1,000 is fast. A value of 10,000,000,000 is heavy.
	Iterations int64 `json:"iterations"`

	// Seed is used for deterministic random number generation (optional but realistic).
	Seed int64 `json:"seed"`
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
