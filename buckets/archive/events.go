package archive

type TrackedJobType int

const (
	TrackedJobTypeArchive TrackedJobType = iota
	TrackedJobTypeRetrieval
)

type TrackedJobStatus int

const (
	TrackedJobStatusQueued TrackedJobStatus = iota
	TrackedJobStatusExecuting
	TrackedJobStatusSuccess
	TrackedJobStatusFailed
)

type JobEvent struct {
	JobID  string
	Type   TrackedJobType
	Status TrackedJobStatus
	AccKey string

	// Only set if Status=Failed
	FailureCause string
}
