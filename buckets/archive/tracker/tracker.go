package tracker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/core/thread"
	powc "github.com/textileio/powergate/v2/api/client"
	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/buckets/archive"
	mdb "github.com/textileio/textile/v2/mongodb"
	tdb "github.com/textileio/textile/v2/threaddb"
)

const (
	// maxConcurrent is the maximum amount of concurrent
	// tracked jobs that are processed in parallel.
	maxConcurrent = 20
)

var (
	// CheckInterval is the frequency in which new ready-to-check
	// tracked jobs are queried.
	CheckInterval = time.Second * 15

	log = logger.Logger("job-tracker")
)

// Tracker tracks Powergate jobs to their final status.
// Tracked jobs corresponds to Archives or Retrievals. Depending
// of the case, is what logic is applied when the Job reach a final
// status.
type Tracker struct {
	lock   sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	closed chan struct{}

	internalSession string
	colls           *mdb.Collections
	buckets         *tdb.Buckets
	pgClient        *powc.Client
	jfe             chan<- archive.JobEvent

	jobPollIntervalSlow time.Duration
	jobPollIntervalFast time.Duration
}

// New returns a *Tracker.
func New(
	colls *mdb.Collections,
	buckets *tdb.Buckets,
	pgClient *powc.Client,
	internalSession string,
	jobPollIntervalSlow time.Duration,
	jobPollIntervalFast time.Duration,
	jfe chan<- archive.JobEvent,
) (*Tracker, error) {
	ctx, cancel := context.WithCancel(context.Background())
	t := &Tracker{
		ctx:    ctx,
		cancel: cancel,
		closed: make(chan struct{}),

		internalSession: internalSession,
		colls:           colls,
		buckets:         buckets,
		pgClient:        pgClient,
		jfe:             jfe,

		jobPollIntervalSlow: jobPollIntervalSlow,
		jobPollIntervalFast: jobPollIntervalFast,
	}
	go t.run()

	return t, nil
}

// TrackArchive registers a new tracking for a Job, which corresponds
// to an bucket Archive action.
func (t *Tracker) TrackArchive(
	ctx context.Context,
	dbID thread.ID,
	dbToken thread.Token,
	bucketKey string,
	jid string,
	bucketRoot cid.Cid,
	owner thread.PubKey,
) error {
	if err := t.colls.ArchiveTracking.CreateArchive(ctx, dbID, dbToken, bucketKey, jid, bucketRoot, owner); err != nil {
		return fmt.Errorf("saving tracking archive information: %s", err)
	}
	return nil
}

// TrackRetrieval registers a new tracking for a Job, which corresponds
// to a Retrieval.
func (t *Tracker) TrackRetrieval(ctx context.Context, accKey, jobID, powToken string) error {
	if err := t.colls.ArchiveTracking.CreateRetrieval(ctx, accKey, jobID, powToken); err != nil {
		return fmt.Errorf("saving tracking retrieval information: %s", err)
	}
	return nil
}

// Close closes the module gracefully.
func (t *Tracker) Close() error {
	t.cancel()
	<-t.closed

	return nil
}

// run is the main daemon logic. It checks on a defined interval
// all non-finalized tracked jobs.
func (t *Tracker) run() {
	defer close(t.closed)
	for {
		select {
		case <-t.ctx.Done():
			log.Info("shutting down archive tracker daemon")
			return
		case <-time.After(CheckInterval):
			t.checkPendingTrackings()
		}
	}
}

// checkPendingTrackings scans for pending trackings in `maxConcurrent`
// batches until all are handeled.
func (t *Tracker) checkPendingTrackings() {
	for {
		archives, err := t.colls.ArchiveTracking.GetReadyToCheck(t.ctx, maxConcurrent)
		if err != nil {
			log.Errorf("getting tracked archives: %s", err)
			break
		}
		log.Debugf("got %d ready archives to check", len(archives))
		if len(archives) == 0 {
			break
		}
		var wg sync.WaitGroup
		wg.Add(len(archives))
		for _, a := range archives {
			go func(a *mdb.TrackedJob) {
				defer wg.Done()

				if err := t.processTrackedJob(a); err != nil {
					log.Errorf("processing tracked archive: %s", err)
				}
			}(a)
		}
		wg.Wait()
	}
}

// processTrackedJob checks the status of the Job in Powergate, which
// corresponds to bucket Archive action.
// - If Job was successful: change its status as to not be tracked anymore,
//   and call corresponding logic depending if was created for an Archive or Retrieval.
// - If Job failed: change its status to not being tracked anymore.
// - If encountered a transient error: It will reschedule the tracked job to
//   be evaluated in a further iteration.
func (t *Tracker) processTrackedJob(a *mdb.TrackedJob) error {
	ctx, cancel := context.WithTimeout(t.ctx, time.Second*10)
	defer cancel()

	var rescheduleDuration time.Duration
	var finCause string
	var err error
	switch a.Type {
	case archive.TrackedJobTypeArchive:
		// TODO: Unfortunately, we can't use a.PowToken to avoid
		// this `t.colls.Accounts` boilerplate to get it. Mostly
		// because a.PowToken was a field created when we implemented
		// Retrievals; so *most of existing mdb.TrackedJob* of type
		// TrackedJobTypeArchive have that value empty, but have
		// set `a.Owner`.
		//
		// Whenever we do the migration to go-datastore, we could include
		// populating all a.PowToken with using this below logic,
		// and after we know all `mdb.TrackedJob` have a.PowToken set,
		// we can delete this `t.cools.Accounts.Get` and just use
		// `a.PowToken` (as is used in `case mdb.TrackedJobTypeRetrieval`.
		// We should also change `Tracker.TrackArchive` to receive
		// a `PowToken string` instead of `owner thread.PublicKey`, and
		// just save `PowToken` to make `a.PowToken` available from that
		// moment forward.
		account, err := t.colls.Accounts.Get(ctx, a.Owner)
		if err != nil {
			return fmt.Errorf("getting account: %s", err)
		}
		if account.PowInfo == nil {
			err := t.colls.ArchiveTracking.Finalize(ctx, a.JID, "no powergate info found")
			if err != nil {
				return fmt.Errorf("finalizing errored/rescheduled job tracking: %s", err)
			}
		}

		ctx = context.WithValue(ctx, powc.AuthKey, account.PowInfo.Token)
		rescheduleDuration, finCause, err = t.trackArchiveProgress(
			ctx,
			a.BucketKey,
			a.DbID,
			a.DbToken,
			a.JID,
			a.BucketRoot,
		)
	case archive.TrackedJobTypeRetrieval:
		ctx = context.WithValue(ctx, powc.AuthKey, a.PowToken)
		rescheduleDuration, finCause, err = t.trackRetrievalProgress(ctx, a.AccKey, a.JID)
	default:
		return fmt.Errorf("unkown tracked job type %d", a.Type)
	}

	if err != nil || rescheduleDuration == 0 {
		if err != nil {
			finCause = err.Error()
		}
		log.Infof("tracking archive finalized with cause: %s", finCause)
		if err := t.colls.ArchiveTracking.Finalize(ctx, a.JID, finCause); err != nil {
			return fmt.Errorf("finalizing errored/rescheduled archive tracking: %s", err)
		}
	}
	log.Infof("rescheduling tracking archive with job %s, cause %s", a.JID, finCause)
	err = t.colls.ArchiveTracking.Reschedule(ctx, a.JID, rescheduleDuration, finCause)
	if err != nil {
		return fmt.Errorf("rescheduling tracked archive: %s", err)
	}

	return nil
}

// trackRetrievalProgress queries the current archive status.
// The return values have the same semantics as `trackArchiveProgress`
func (t *Tracker) trackRetrievalProgress(ctx context.Context, accKey, jid string) (time.Duration, string, error) {
	log.Debugf("querying archive status of job %s", jid)
	defer log.Debugf("finished querying retrieval status of job %s", jid)

	// Step 1: Get the Job status.
	res, err := t.pgClient.StorageJobs.Get(ctx, jid)
	if err != nil {
		// if error specifies that the auth token isn't found, powergate must have been reset.
		// return the error as fatal so the archive will be untracked
		if strings.Contains(err.Error(), "auth token not found") {
			return 0, "", err
		}
		return t.jobPollIntervalSlow, fmt.Sprintf("getting current job %s for retrieval: %s", jid, err), nil
	}

	// Step 2: Notify `JobEvent` listeners about job changing status.
	var (
		rescheduleDuration time.Duration
		message            string
		status             archive.TrackedJobStatus
	)
	switch res.StorageJob.Status {
	case userPb.JobStatus_JOB_STATUS_SUCCESS:
		rescheduleDuration = 0
		message = "success"
		status = archive.TrackedJobStatusSuccess
	case userPb.JobStatus_JOB_STATUS_CANCELED:
		rescheduleDuration = 0
		message = "canceled"
		status = archive.TrackedJobStatusFailed
	case userPb.JobStatus_JOB_STATUS_FAILED:
		rescheduleDuration = 0
		message = "job failed"
		status = archive.TrackedJobStatusFailed
	case userPb.JobStatus_JOB_STATUS_EXECUTING:
		rescheduleDuration = t.jobPollIntervalFast
		message = "non-final status"
		status = archive.TrackedJobStatusExecuting
	default:
		return 0, "", fmt.Errorf("unkown storage-job status: %d", res.StorageJob.Status)
	}

	t.jfe <- archive.JobEvent{
		JobID:        jid,
		Type:         archive.TrackedJobTypeRetrieval,
		Status:       status,
		AccKey:       accKey,
		FailureCause: message,
	}

	return rescheduleDuration, message, nil
}

// trackArchiveProgress queries the current archive status.
// If a fatal error in tracking happens, it will return an error, which indicates the archive should be untracked.
// If the archive didn't reach a final status yet, or a possibly recoverable error (by retrying) happens,
// it will return (duration > 0, "retry cause", nil) and the archive query should be rescheduled for duration in the future.
// If the archive reach final status, it will return (duration == 0, "", nil) and the tracking can be considered done.
func (t *Tracker) trackArchiveProgress(
	ctx context.Context,
	buckKey string,
	dbID thread.ID,
	dbToken thread.Token,
	jid string,
	bucketRoot cid.Cid,
) (time.Duration, string, error) {
	log.Debugf("querying archive status of job %s", jid)
	defer log.Debugf("finished querying archive status of job %s", jid)

	// Step 1: Get the Job status.
	res, err := t.pgClient.StorageJobs.Get(ctx, jid)
	if err != nil {
		// if error specifies that the auth token isn't found, powergate must have been reset.
		// return the error as fatal so the archive will be untracked
		if strings.Contains(err.Error(), "auth token not found") {
			return 0, "", err
		}
		return t.jobPollIntervalSlow, fmt.Sprintf("getting current job %s for bucket %s: %s", jid, buckKey, err), nil
	}

	// Step 2: On job success, save Deal data in the underlying Bucket thread. On
	// failure save the error message.
	if res.StorageJob.Status == userPb.JobStatus_JOB_STATUS_SUCCESS {
		if err := t.saveDealsInBucket(ctx, buckKey, dbID, dbToken, bucketRoot); err != nil {
			return t.jobPollIntervalSlow, fmt.Sprintf("saving deal data in archive: %s", err), nil
		}
	}
	// Step 3: Update status on Mongo for the archive.
	if err := t.updateArchiveStatus(ctx, buckKey, res.StorageJob, false, ""); err != nil {
		return t.jobPollIntervalSlow, fmt.Sprintf("updating archive status: %s", err), nil
	}

	rescheduleDuration := t.jobPollIntervalFast
	message := "non-final status"
	if isJobStatusFinal(res.StorageJob.Status) {
		message = "reached final status"
		rescheduleDuration = 0 // Finalize tracking.
	}

	if rescheduleDuration > 0 {
		for _, dealInfo := range res.StorageJob.DealInfo {
			if dealInfo.StateId == storagemarket.StorageDealSealing {
				rescheduleDuration = t.jobPollIntervalSlow
				break
			}
		}
	}

	return rescheduleDuration, message, nil
}

// updateArchiveStatus save the last known job status. It also receives an
// _abort_ flag which indicates that the provided newJobStatus might not be
// final. For example, if tracking the Job status changes errored by some network
// condition, we have only the last known Job status. Powergate would continue doing
// its work until a final status is reached; it only means we aren't sure how this
// archive really finished.
// To track for this situation, we use the _aborted_ and _abortMsg_ parameters.
// An archive with _aborted_ true should eventually be re-queried to understand
// how it finished (if wanted).
func (t *Tracker) updateArchiveStatus(
	ctx context.Context,
	buckKey string,
	job *userPb.StorageJob,
	aborted bool,
	abortMsg string,
) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	ba, err := t.colls.BucketArchives.GetOrCreate(ctx, buckKey)
	if err != nil {
		return fmt.Errorf("getting BucketArchive data: %s", err)
	}

	archiveToUpdate := &ba.Archives.Current
	if archiveToUpdate.JobID != job.Id {
		for i := range ba.Archives.History {
			if ba.Archives.History[i].JobID == job.Id {
				archiveToUpdate = &ba.Archives.History[i]
				break
			}
		}
	}
	archiveToUpdate.Status = int(job.Status)
	archiveToUpdate.Aborted = aborted
	archiveToUpdate.AbortedMsg = abortMsg
	archiveToUpdate.FailureMsg = prepareFailureMsg(job)

	dealInfo := make([]mdb.DealInfo, len(job.DealInfo))
	for i, info := range job.DealInfo {
		dealInfo[i] = mdb.DealInfo{
			ProposalCid:     info.ProposalCid,
			StateID:         info.StateId,
			StateName:       info.StateName,
			Miner:           info.Miner,
			PieceCID:        info.PieceCid,
			Size:            info.Size,
			PricePerEpoch:   info.PricePerEpoch,
			StartEpoch:      info.StartEpoch,
			Duration:        info.Duration,
			DealID:          info.DealId,
			ActivationEpoch: info.ActivationEpoch,
			Message:         info.Message,
		}
	}
	archiveToUpdate.DealInfo = dealInfo
	if err := t.colls.BucketArchives.Replace(ctx, ba); err != nil {
		return fmt.Errorf("updating bucket archives status: %s", err)
	}

	return nil
}

func (t *Tracker) saveDealsInBucket(
	ctx context.Context,
	buckKey string,
	dbID thread.ID,
	dbToken thread.Token,
	c cid.Cid,
) error {
	opts := tdb.WithToken(dbToken)
	ctx = common.NewSessionContext(ctx, t.internalSession)
	buck := &tdb.Bucket{}
	if err := t.buckets.Get(ctx, dbID, buckKey, &buck, opts); err != nil {
		return fmt.Errorf("getting bucket for save deals: %s", err)
	}
	res, err := t.pgClient.Data.CidInfo(ctx, c.String())
	if err != nil {
		return fmt.Errorf("getting cid info: %s", err)
	}

	proposals := res.CidInfo.CurrentStorageInfo.Cold.Filecoin.Proposals

	deals := make([]tdb.Deal, len(proposals))
	for i, p := range proposals {
		deals[i] = tdb.Deal{
			Miner: p.Miner,
		}
	}
	buck.Archives.Current = tdb.Archive{
		Cid:   c.String(),
		Deals: deals,
	}
	buck.UpdatedAt = time.Now().UnixNano()
	if err = t.buckets.Save(ctx, dbID, buck, opts); err != nil {
		return fmt.Errorf("saving deals in thread: %s", err)
	}
	return nil
}

func prepareFailureMsg(job *userPb.StorageJob) string {
	if job.ErrorCause == "" {
		return ""
	}
	var b strings.Builder
	_, _ = b.WriteString(job.ErrorCause)
	for i, de := range job.DealErrors {
		_, _ = b.WriteString(fmt.Sprintf(
			"\nDeal error %d: Proposal %s with miner %s, %s", i, de.ProposalCid, de.Miner, de.Message))
	}
	return b.String()
}

func isJobStatusFinal(js userPb.JobStatus) bool {
	return js == userPb.JobStatus_JOB_STATUS_SUCCESS ||
		js == userPb.JobStatus_JOB_STATUS_CANCELED ||
		js == userPb.JobStatus_JOB_STATUS_FAILED
}
