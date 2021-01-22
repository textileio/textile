package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/core/thread"
	pow "github.com/textileio/powergate/v2/api/client"
	powc "github.com/textileio/powergate/v2/api/client"
	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/buckets/archive"
	"google.golang.org/grpc/codes"
)

const (
	ipfsAddTimeoutDefault       = 500 // seconds
	buckCreationTimeout         = time.Minute * 30
	maxConcurrentBucketCreation = 10
)

var (
	log = logging.Logger("fil-retrieval")

	CITest = false
)

// BucketCreator provides creating bucket functionality.
type BucketCreator interface {
	// CreateBucket creates a bucket with the provided information.
	CreateBucket(ctx context.Context,
		threadID thread.ID,
		threadToken thread.Token,
		buckName string,
		buckPrivate bool,
		dataCid cid.Cid) error
}

// TrackerRegistrator manages retrieval Job tracking.
type TrackerRetrievalRegistrator interface {
	// TrackRetrieval tracks JobID job.
	TrackRetrieval(ctx context.Context, accKey, jobID, powToken string) error
}

type Status int

const (
	StatusQueued Status = iota
	StatusExecuting
	StatusMoveToBucket
	StatusSuccess
	StatusFailed
)

type Type int

const (
	TypeNewBucket Type = iota
	TypeExistingBucket
)

type Retrieval struct {
	Type         Type
	AccountKey   string
	PowToken     string
	JobID        string
	Cid          cid.Cid
	Selector     string
	Status       Status
	FailureCause string
	CreatedAt    int64

	// If Type == RetrievalTypeNewBucket
	DbID    thread.ID
	DbToken thread.Token
	Name    string
	Private bool

	// If Type == RetrievalTypeExistingBucket
	BuckKey  string
	BuckPath string
}

// FilRetrieval manages Retrievals from Accounts in
// go-datastore.
type FilRetrieval struct {
	ds              datastore.TxnDatastore
	bc              BucketCreator
	trr             TrackerRetrievalRegistrator
	pgc             *powc.Client
	jfe             <-chan archive.JobEvent
	internalSession string

	// daemon vars
	started    bool
	daemonWork chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc
	closedChan chan struct{}
	closed     bool
}

func NewFilRetrieval(
	ds datastore.TxnDatastore,
	pgc *powc.Client,
	trr TrackerRetrievalRegistrator,
	jfe <-chan archive.JobEvent,
	is string,
) (*FilRetrieval, error) {
	ctx, cancel := context.WithCancel(context.Background())
	fr := &FilRetrieval{
		ds:  ds,
		pgc: pgc,
		trr: trr,
		jfe: jfe,

		internalSession: is,

		ctx:        ctx,
		cancel:     cancel,
		closedChan: make(chan struct{}),
		daemonWork: make(chan struct{}, 1),
	}

	return fr, nil
}

func (fr *FilRetrieval) SetBucketCreator(bc BucketCreator) {
	fr.bc = bc
}

func (fr *FilRetrieval) RunDaemon() {
	if !fr.started {
		go fr.daemon()
		fr.started = true
	}
}

// CreateForNewBucket creates a queued Retrieval to bootstrap a new bucket
// with the provided information, only if the retrieval succeeds.
func (fr *FilRetrieval) CreateForNewBucket(
	ctx context.Context,
	accKey string,
	dbID thread.ID,
	dbToken thread.Token,
	buckName string,
	buckPrivate bool,
	dataCid cid.Cid,
	powToken string,
) (string, error) {
	if powToken == "" {
		return "", fmt.Errorf("powergate token can't be empty")
	}
	if accKey == "" {
		return "", fmt.Errorf("account key can't be empty")
	}

	jobID, err := fr.createRetrieval(ctx, dataCid, accKey, powToken)
	if err != nil {
		return "", fmt.Errorf("creating retrieval in Powergate: %s", err)
	}
	r := Retrieval{
		Type:       TypeNewBucket,
		AccountKey: accKey,
		PowToken:   powToken,
		JobID:      jobID,
		Cid:        dataCid,
		Selector:   "",
		Status:     StatusQueued,
		CreatedAt:  time.Now().Unix(),

		DbID:    dbID,
		DbToken: dbToken,
		Name:    buckName,
		Private: buckPrivate,
	}

	if err := fr.save(nil, r); err != nil {
		return "", fmt.Errorf("saving retrieval request: %s", err)
	}

	return jobID, nil
}

func (fr *FilRetrieval) createRetrieval(ctx context.Context, c cid.Cid, accKey, powToken string) (string, error) {
	// # Step 1.
	// Check that we have imported DealIDs for that Cid. Fail fast mechanism.
	ctx = context.WithValue(ctx, pow.AuthKey, powToken)
	r, err := fr.pgc.StorageInfo.List(ctx, c.String())
	if err != nil {
		return "", fmt.Errorf("getting archived cids: %s", err)
	}
	if len(r.StorageInfo) == 0 {
		return "", fmt.Errorf("no data is available for this cid")
	}
	if len(r.StorageInfo[0].Cold.Filecoin.Proposals) == 0 {
		return "", fmt.Errorf("there aren't known deals to retrieve from Filecoin")
	}

	// # Step 2.
	// Check that hot-storage is disabled. If it's enabled, then this data
	// is already hot in Powergate. No need to do a Filecoin retrieval, we can
	// directly jump to the bucket creation. The data is already available in
	// go-ipfs from Powergate, which is connected with go-ipfs from Hub.
	// It can be considered that the data is in the ipfs network, so
	// a user can do `hub buck init --cid` (without --unfreeze).
	// In the future, we can think if make sense to let this case behave as an
	// automatic call to the above CLI command.
	//
	// Note (07/01/21): This is just for future coverage. In the current use-case
	// this situation shouldn't happen, since after unfreezing from Filecoin we
	// disable the Cid from hot-storage (so can be GCed). But if in a future use-case
	// we enable the user to keep that unfreezed data hot (for whatever reason),
	// then already leverage that future feature.
	if r.StorageInfo[0].Hot.Enabled {
		return "", fmt.Errorf("no need to unfreeze, the cid is already in the IPFS network")
	}

	// # Step 3.
	// At this point we're sure we have imported DealIDs and the data isn't
	// in hot-storage. We proceed to pushing the current StorageConfig with
	// hot-storage enabled. This will signal attempting a retrieval.

	// Get the base storage-config to modify.
	var notFound bool
	ci, err := fr.pgc.Data.CidInfo(ctx, c.String())
	if err != nil {
		sc, ok := status.FromError(err)
		if !ok || sc.Code() != codes.NotFound {
			return "", fmt.Errorf("getting latest storage-config: %s", err)
		}
		notFound = true
	}

	var sc *userPb.StorageConfig
	if notFound {
		// If no storage-config exist for this Cid, then the user
		// had Remove or Replace the storage-config.
		// We ne need a storage-config to do the retrieval, so we
		// use the default one but with cold-storage disabled as
		// to avoid any other cold-storage work that might be
		// enabled as default.
		dfsc, err := fr.pgc.StorageConfig.Default(ctx)
		if err != nil {
			return "", fmt.Errorf("no storage-config, getting default: %s", err)
		}
		sc := dfsc.DefaultStorageConfig
		sc.Cold.Enabled = false
	} else {
		sc = ci.CidInfo.LatestPushedStorageConfig

		// Cover some bad configuration.
		if sc.Hot.Ipfs.AddTimeout == 0 {
			sc.Hot.Ipfs.AddTimeout = ipfsAddTimeoutDefault
		}
	}
	// Flag only used for tests, to speed-up Filecoin unfreezing
	// behaviour. Under normal circumstances, this timeout would be
	// in the order of minutes to give a good chance of finding the
	// data in the IPFS network.
	if CITest { // Flag only used in CI test.
		sc.Hot.Ipfs.AddTimeout = 3
	}
	sc.Hot.Enabled = true
	sc.Hot.AllowUnfreeze = true
	opts := []pow.ApplyOption{pow.WithStorageConfig(sc), pow.WithOverride(true)}
	a, err := fr.pgc.StorageConfig.Apply(ctx, c.String(), opts...)
	if err != nil {
		return "", fmt.Errorf("applying new storage-config: %s", err)
	}
	createdJobID := a.JobId

	// # Step 4.
	// Powergate is doing it's retrieval work. Register this JobID to track the
	// status in Tracker. When Tracker updates the Job status, it will signal in
	// fr.jfe channel, so the retrieval process can continue with further phases.
	if err := fr.trr.TrackRetrieval(ctx, accKey, createdJobID, powToken); err != nil {
		return "", fmt.Errorf("tracking retrieval job: %s", err)
	}

	return createdJobID, nil
}

// Close closes gracefully all background work that
// is being executed.
func (fr *FilRetrieval) Close() error {
	if fr.closed {
		return nil
	}
	fr.cancel()
	<-fr.closedChan

	return nil
}

func (fr *FilRetrieval) daemon() {
	defer func() { close(fr.closedChan) }()

	fr.daemonWork <- struct{}{} // Force first run.
	for {
		select {
		case <-fr.ctx.Done():
			return
		case <-fr.daemonWork:
			fr.processQueuedMoveToBucket()
		case ju := <-fr.jfe:
			if ju.Type != archive.TrackedJobTypeRetrieval {
				// We're not interested in other jobs type events.
				continue
			}
			fr.updateRetrievalStatus(ju)
		}
	}
}

// updateRetrievalStatus updates the status and continues with the process.
// If `successful` is true, its status is changed to RetrievalStatusMoveToBucket,
// and it continues with the process of copying the data to the bucket.
// If `successful` is false, its status is changed to RetrievalStatusFailed and the
// FailureCause is set to `failureCause`.
func (fr *FilRetrieval) updateRetrievalStatus(ju archive.JobEvent) {
	r, err := fr.GetByAccountAndID(ju.AccKey, ju.JobID)
	if err != nil {
		// go-datastore is unavailable, which is a very rare-case.
		// keep a log fo what should have happened in case we wan't to recover this
		// case manually.
		log.Errorf("getting retrieval from store (accKey:%s, jobID:%s, status:%d, failureCause: %s): %s", ju.AccKey, ju.JobID, ju.Status, ju.FailureCause, err)
		return
	}

	switch ju.Status {
	case archive.TrackedJobStatusFailed:
		r.Status = StatusFailed
		r.FailureCause = ju.FailureCause
	case archive.TrackedJobStatusExecuting:
		r.Status = StatusExecuting
	case archive.TrackedJobStatusSuccess:
		r.Status = StatusMoveToBucket
	default:
		log.Errorf("unkown job event status: %d", ju.Status)
		return
	}

	txn, err := fr.ds.NewTransaction(false)
	if err != nil {
		log.Errorf("creating txn (accKey:%s, jobID:%s, status:%d, failureCause: %s): %s", ju.AccKey, ju.JobID, ju.Status, ju.FailureCause, err)
		return
	}
	defer txn.Discard()

	// Save Retrieval with new status *and* insert this retrieval
	// in the special MoveToBucket stage, to jump into next phase
	// handeled by the daemon().
	if err := fr.save(txn, r); err != nil {
		log.Errorf("saving to datastore (accKey:%s, jobID:%s, status:%d, failureCause: %s): %s", ju.AccKey, ju.JobID, ju.Type, ju.FailureCause, err)
		return
	}

	// If the retrieval switched to executing, or failed we're done.
	if ju.Status != archive.TrackedJobStatusSuccess {
		return
	}

	// We know the Cid data is available (Success), move to next phase to move the data
	// to the bucket. We do it in phases as to avoid failing all the retrieval if the process
	// crashes, or shutdowns. When is spinned up again, we can recover the process to move the data
	// to the bucket without having retrievals stuck or forcing the user to create a new retrieval
	// starting from zero.
	//
	// The daemon() will take these retrievals, and continue with the process. After it finishes,
	// it will be removed from the pending list (reached final status).
	key := dsMoveToBucketQueueKey(ju.AccKey, ju.JobID)
	if err := txn.Put(key, []byte{}); err != nil {
		log.Errorf("inserting into move-to-bucket queue (accKey:%s, jobID:%s, status:%d, failureCause: %s): %s", ju.AccKey, ju.JobID, ju.Status, ju.FailureCause, err)
		return
	}

	if err := txn.Commit(); err != nil {
		log.Errorf("commiting txn (accKey:%s, jobID:%s, type:%s, failureCause: %s): %s", ju.AccKey, ju.JobID, ju.Type, ju.FailureCause, err)
		return
	}

	// Signal daemon that new work exist.
	select {
	case fr.daemonWork <- struct{}{}:
	default:
	}
}

func (fr *FilRetrieval) processQueuedMoveToBucket() {
	prefixKey := dsMoveToBucketQueueBaseKey()
	q := query.Query{Prefix: prefixKey.String()}
	qr, err := fr.ds.Query(q)
	if err != nil {
		log.Errorf("querying datastore for pending move-to-bucket: %s", err)
		return
	}
	defer qr.Close()

	lim := make(chan struct{}, maxConcurrentBucketCreation)
	for item := range qr.Next() {
		if item.Error != nil {
			log.Errorf("getting next item result: %s", err)
			break
		}
		lim <- struct{}{}
		item := item
		go func() {
			defer func() { <-lim }()

			key := datastore.NewKey(item.Key)
			accKey := key.Namespaces()[2]
			jobID := key.Namespaces()[3]

			r, err := fr.GetByAccountAndID(accKey, jobID)
			if err != nil {
				log.Errorf("get move-to-bucket retrieval: %s", err)
				return
			}

			if err := fr.processMoveToBucket(r); err != nil {
				log.Errorf("processing move-to-bucket: %s", err)
				r.Status = StatusFailed
				r.FailureCause = err.Error()
			} else {
				r.Status = StatusSuccess
			}

			txn, err := fr.ds.NewTransaction(false)
			if err != nil {
				log.Errorf("creating txn: %s", err)
				return
			}

			if err := fr.save(txn, r); err != nil {
				txn.Discard()
				log.Errorf("saving finalized status: %s", err)
				return
			}

			if err := txn.Delete(key); err != nil {
				txn.Discard()
				log.Errorf("deleting from move-to-bucket queue: %s", err)
				return
			}

			if err := txn.Commit(); err != nil {
				txn.Discard()
				log.Errorf("committing transaction: %s", err)
			}
		}()
	}

	for i := 0; i < maxConcurrentBucketCreation; i++ {
		lim <- struct{}{}
	}
}

func (fr *FilRetrieval) processMoveToBucket(r Retrieval) error {
	switch r.Type {
	case TypeNewBucket:
		// # Step 1.
		// Create the bucket.
		ctx, cancel := context.WithTimeout(context.Background(), buckCreationTimeout)
		defer cancel()
		ctx = common.NewSessionContext(ctx, fr.internalSession)
		if err := fr.bc.CreateBucket(ctx, r.DbID, r.DbToken, r.Name, r.Private, r.Cid); err != nil {
			return fmt.Errorf("creating bucket: %s", err)
		}

		// # Step 2.
		// Now that the bucket was bootstraped, we're ready to push a new StorageConfig
		// disabling hot-storage. We don't need to hold the data in Powergate anymore.
		ctx = context.WithValue(ctx, pow.AuthKey, r.PowToken)
		ci, err := fr.pgc.Data.CidInfo(ctx, r.Cid.String())
		if err != nil {
			return fmt.Errorf("getting latest storage-config: %s", err)
		}
		sc := ci.CidInfo.LatestPushedStorageConfig
		sc.Hot.Enabled = false
		sc.Hot.AllowUnfreeze = false
		opts := []pow.ApplyOption{pow.WithStorageConfig(sc), pow.WithOverride(true)}
		_, err = fr.pgc.StorageConfig.Apply(ctx, r.Cid.String(), opts...)
		if err != nil {
			log.Errorf("applying new storage-config: %s", err)
		}

		// Note: Notice that we're ignoring the JobID or error for Apply().
		// At this point the bucket was created, and all is fine for the user.
		//
		// Disabling hot-storage is rather uninteresting. It's a change that
		// will most probably be executed correctly since it's only unpinning from go-ipfs.
		// Even if this failed for whatever reason its kind of independant of
		// the retrieval success; since the bucket got created correctly and
		// all is ready now for the user.
		//
		// If we think this should require attention, we can create some
		// process thatkeeps track of that job in the background but completely
		// unattached from the retrieval use-case. This sounds more like something
		// that an external daemon should do.
	default:
		return fmt.Errorf("retrieval type %d not implemented", r.Type)
	}

	return nil
}

func (fr *FilRetrieval) save(txn datastore.Txn, r Retrieval) error {
	if len(r.AccountKey) == 0 || len(r.JobID) == 0 {
		return fmt.Errorf("account key and job-id can't be empty")
	}

	key := dsAccountAndIDKey(r.AccountKey, r.JobID)
	buf, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("mashaling retrieval: %s", err)
	}

	dsWriter := datastore.Write(fr.ds)
	if txn != nil {
		dsWriter = txn
	}

	if err := dsWriter.Put(key, buf); err != nil {
		return fmt.Errorf("saving in datastore: %s", err)
	}

	return nil
}

func (fr *FilRetrieval) GetAllByAccount(accKey string) ([]Retrieval, error) {
	q := query.Query{
		Prefix: dsAccountKey(accKey).String(),
	}
	res, err := fr.ds.Query(q)
	if err != nil {
		return nil, fmt.Errorf("initiating query: %s", err)
	}
	defer res.Close()

	var ret []Retrieval
	for it := range res.Next() {
		if it.Error != nil {
			return nil, fmt.Errorf("fetching query item: %s", it.Error)
		}

		r := Retrieval{}
		if err := json.Unmarshal(it.Entry.Value, &r); err != nil {
			return nil, fmt.Errorf("unmarshaling retrieval: %s", err)
		}

		ret = append(ret, r)
	}

	return ret, nil
}

func (fr *FilRetrieval) GetByAccountAndID(accKey string, jobID string) (Retrieval, error) {
	key := dsAccountAndIDKey(accKey, jobID)

	b, err := fr.ds.Get(key)
	if err != nil {
		return Retrieval{}, fmt.Errorf("get retrieval: %s", err)
	}
	r := Retrieval{}
	if err := json.Unmarshal(b, &r); err != nil {
		return Retrieval{}, fmt.Errorf("unmarshaling retrieval: %s", err)
	}

	return r, nil
}

func (fr *FilRetrieval) Logs(ctx context.Context, accKey string, jobID string, powToken string, ch chan<- string) error {
	r, err := fr.GetByAccountAndID(accKey, jobID)
	if err != nil {
		return fmt.Errorf("get retrieval for logs: %s", err)
	}

	// This should never happen. All retrievals are created with a JobID
	// value. Paranoid check.
	if len(r.JobID) == 0 {
		return fmt.Errorf("job-id is empty")
	}

	ctx = context.WithValue(ctx, powc.AuthKey, powToken)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logsCh := make(chan powc.WatchLogsEvent)
	if err := fr.pgc.Data.WatchLogs(
		ctx,
		logsCh,
		r.Cid.String(),
		powc.WithJobIDFilter(r.JobID),
		powc.WithHistory(true),
	); err != nil {
		return fmt.Errorf("start watching job logs: %s", err)
	}
	for le := range logsCh {
		if le.Err != nil {
			return le.Err
		}
		ch <- le.Res.LogEntry.Message
	}
	return nil
}

// Datastore keys
// Requests: /retrievals/<account-key>/<job-id> -> json(Request)
// MoveToBucketQueue: /movetobucket/<timestamp>/<account-key>/<job-id> -> Empty
func dsAccountKey(accKey string) datastore.Key {
	return datastore.NewKey("retrieval").ChildString(accKey)
}

func dsAccountAndIDKey(accKey, jobID string) datastore.Key {
	return dsAccountKey(accKey).ChildString(jobID)
}

func dsMoveToBucketQueueBaseKey() datastore.Key {
	key := datastore.NewKey("movetobucket")
	return key
}

func dsMoveToBucketQueueKey(accKey, jobID string) datastore.Key {
	key := dsMoveToBucketQueueBaseKey()
	key = key.ChildString(strconv.FormatInt(time.Now().Unix(), 10))
	key = key.ChildString(accKey)
	key = key.ChildString(jobID)
	return key
}
