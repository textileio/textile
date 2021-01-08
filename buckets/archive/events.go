package archive

type JobFinalizedEvent struct {
	JobID        string
	Success      bool
	FailureCause string
}
