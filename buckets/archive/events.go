package archive

type TrackedJobType int

const (
	TrackedJobTypeArchive TrackedJobType = iota
	TrackedJobTypeRetrieval
)

type JobFinalizedEvent struct {
	JobID        string
	Type         TrackedJobType
	AccKey       string
	Success      bool
	FailureCause string
}
