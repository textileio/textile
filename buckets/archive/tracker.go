package archive

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log"
	"github.com/textileio/go-threads/core/thread"
	powc "github.com/textileio/powergate/api/client"
	userPb "github.com/textileio/powergate/api/gen/powergate/user/v1"
	"github.com/textileio/textile/v2/api/common"
	mdb "github.com/textileio/textile/v2/mongodb"
	tdb "github.com/textileio/textile/v2/threaddb"
)

const (
	maxConcurrent = 20
)

var (
	CheckInterval         = time.Second * 15
	JobStatusPollInterval = time.Minute * 30

	log = logger.Logger("pow-archive")
)

type Tracker struct {
	lock   sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	closed chan struct{}

	internalSession string
	colls           *mdb.Collections
	buckets         *tdb.Buckets
	pgClient        *powc.Client
}

func New(
	colls *mdb.Collections,
	buckets *tdb.Buckets,
	pgClient *powc.Client,
	internalSession string,
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
	}
	go t.run()
	return t, nil
}

func (t *Tracker) Close() error {
	t.cancel()
	<-t.closed
	return nil
}

func (t *Tracker) run() {
	defer close(t.closed)
	for {
		select {
		case <-t.ctx.Done():
			log.Info("shutting down archive tracker daemon")
			return
		case <-time.After(CheckInterval):
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
					go func(a *mdb.TrackedArchive) {
						defer wg.Done()

						ctx, cancel := context.WithTimeout(t.ctx, time.Second*10)
						defer cancel()

						account, err := t.colls.Accounts.Get(ctx, a.Owner)
						if err != nil {
							log.Errorf("getting account: %s", err)
							return
						}

						if account.PowInfo == nil {
							err := t.colls.ArchiveTracking.Finalize(ctx, a.JID, "no powergate info found")
							if err != nil {
								log.Errorf("finalizing errored/rescheduled archive tracking: %s", err)
							}
							return
						}

						reschedule, cause, err := t.trackArchiveProgress(
							ctx,
							a.BucketKey,
							a.DbID,
							a.DbToken,
							a.JID,
							a.BucketRoot,
							account.PowInfo,
						)
						if err != nil || !reschedule {
							if err != nil {
								cause = err.Error()
							}
							log.Infof("tracking archive finalized with cause: %s", cause)
							if err := t.colls.ArchiveTracking.Finalize(ctx, a.JID, cause); err != nil {
								log.Errorf("finalizing errored/rescheduled archive tracking: %s", err)
							}
							return
						}
						log.Infof("rescheduling tracking archive with job %s, cause %s", a.JID, cause)
						err = t.colls.ArchiveTracking.Reschedule(ctx, a.JID, JobStatusPollInterval, cause)
						if err != nil {
							log.Errorf("rescheduling tracked archive: %s", err)
						}
					}(a)
				}
				wg.Wait()
			}

		}
	}
}

func (t *Tracker) Track(
	ctx context.Context,
	dbID thread.ID,
	dbToken thread.Token,
	bucketKey string,
	jid string,
	bucketRoot cid.Cid,
	owner thread.PubKey,
) error {
	if err := t.colls.ArchiveTracking.Create(ctx, dbID, dbToken, bucketKey, jid, bucketRoot, owner); err != nil {
		return fmt.Errorf("saving tracking information: %s", err)
	}
	return nil
}

// trackArchiveProgress queries the current archive status.
// If a fatal error in tracking happens, it will return an error, which indicates the archive should be untracked.
// If the archive didn't reach a final status yet, or a possibly recoverable error (by retrying) happens,
// it will return (true, "retry cause", nil).
// If the archive reach final status, it will return (false, "", nil) and the tracking can be considered done.
func (t *Tracker) trackArchiveProgress(
	ctx context.Context,
	buckKey string,
	dbID thread.ID,
	dbToken thread.Token,
	jid string,
	bucketRoot cid.Cid,
	powInfo *mdb.PowInfo,
) (bool, string, error) {
	log.Infof("querying archive status of job %s", jid)
	defer log.Infof("finished querying archive status of job %s", jid)
	// Step 1: watch for the Job status, and keep updating on Mongo until
	// we're in a final state.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = context.WithValue(ctx, powc.AuthKey, powInfo.Token)
	res, err := t.pgClient.StorageJobs.StorageJob(ctx, jid)
	if err != nil {
		// if error specifies that the auth token isn't found, powergate must have been reset.
		// return the error as fatal so the archive will be untracked
		if strings.Contains(err.Error(), "auth token not found") {
			return false, "", err
		}
		return true, fmt.Sprintf("getting current job %s for bucket %s: %s", jid, buckKey, err), nil
	}

	// Step 2: On success, save Deal data in the underlying Bucket thread. On
	// failure save the error message. Also update status on Mongo for the archive.
	if res.StorageJob.Status == userPb.JobStatus_JOB_STATUS_SUCCESS {
		if err := t.saveDealsInArchive(ctx, buckKey, dbID, dbToken, powInfo.Token, bucketRoot); err != nil {
			return true, fmt.Sprintf("saving deal data in archive: %s", err), nil
		}
	}
	if err := t.updateArchiveStatus(ctx, buckKey, res.StorageJob, false, ""); err != nil {
		return true, fmt.Sprintf("updating archive status: %s", err), nil
	}

	message := "reached final status"
	reschedule := !isJobStatusFinal(res.StorageJob.Status)
	if reschedule {
		message = "non-final status"
	}

	return reschedule, message, nil
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

func (t *Tracker) saveDealsInArchive(
	ctx context.Context,
	buckKey string,
	dbID thread.ID,
	dbToken thread.Token,
	authToken string,
	c cid.Cid,
) error {
	opts := tdb.WithToken(dbToken)
	ctx = common.NewSessionContext(ctx, t.internalSession)
	buck := &tdb.Bucket{}
	if err := t.buckets.Get(ctx, dbID, buckKey, &buck, opts); err != nil {
		return fmt.Errorf("getting bucket for save deals: %s", err)
	}
	ctxAuth := context.WithValue(ctx, powc.AuthKey, authToken)
	res, err := t.pgClient.Data.CidInfo(ctxAuth, c.String())
	if err != nil {
		return fmt.Errorf("getting cid info: %s", err)
	}
	if len(res.CidInfos) == 0 {
		return fmt.Errorf("no cid info found")
	}

	proposals := res.CidInfos[0].CurrentStorageInfo.Cold.Filecoin.Proposals

	deals := make([]tdb.Deal, len(proposals))
	for i, p := range proposals {
		deals[i] = tdb.Deal{
			ProposalCid: p.ProposalCid,
			Miner:       p.Miner,
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
