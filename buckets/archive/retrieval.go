package archive

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/textileio/go-threads/core/thread"
	pow "github.com/textileio/powergate/api/client"
	powc "github.com/textileio/powergate/api/client"
)

type RetrievalStatus int

const (
	RetrievalStatusQueued RetrievalStatus = iota
	RetrievalStatusExecuting
	RetrievalStatusMoveToBucket
	RetrievalStatusSuccess
	RetrievalStatusFailed
)

type RetrievalType int

const (
	RetrievalTypeNewBucket RetrievalType = iota
	RetrievalTypeExistingBucket
)

type Retrieval struct {
	Type       RetrievalType
	AccountKey string
	JobID      string
	Cid        cid.Cid
	Selector   string
	Status     RetrievalStatus
	// TODO: do this.
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
	ds       datastore.TxnDatastore
	pgClient *powc.Client
}

func NewFilRetrieval(ds datastore.TxnDatastore, pgClient *powc.Client) (*FilRetrieval, error) {
	return &FilRetrieval{
		ds:       ds,
		pgClient: pgClient,
	}, nil
}

func (fr *FilRetrieval) CreateForNewBucket(
	ctx context.Context,
	accKey string,
	dbID thread.ID,
	dbToken thread.Token,
	buckName string,
	buckPrivate bool,
	dataCid cid.Cid,
	powToken string,
) error {
	jobID, err := fr.createRetrieval(ctx, dataCid, powToken)
	if err != nil {
		return fmt.Errorf("creating retrieval in Powergate: %s", err)
	}
	r := Retrieval{
		Type:       RetrievalTypeNewBucket,
		AccountKey: accKey,
		JobID:      jobID,
		Cid:        dataCid,
		Selector:   "",
		Status:     RetrievalStatusQueued,
		CreatedAt:  time.Now().Unix(),

		DbID:    dbID,
		DbToken: dbToken,
		Name:    buckName,
		Private: buckPrivate,
	}

	if err := fr.save(accKey, r); err != nil {
		return fmt.Errorf("saving retrieval request: %s", err)
	}

	return nil
}

func (fr *FilRetrieval) createRetrieval(ctx context.Context, c cid.Cid, powToken string) (string, error) {
	// # Step 1.
	// Check that we have imported DealIDs for that Cid. Fail fast mechanism.
	ctx = context.WithValue(ctx, pow.AuthKey, powToken)
	r, err := fr.pgClient.StorageInfo.ListStorageInfo(ctx, c.String())
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
	ci, err := fr.pgClient.Data.CidInfo(ctx, c.String())
	if err != nil {
		return "", fmt.Errorf("getting latest storage-config: %s", err)
	}
	// Paranoid check to avoid panic. Shouldn't happen.
	if len(ci.CidInfos) != 1 {
		return "", fmt.Errorf("unexpected cid info length: %d", len(ci.CidInfos))
	}
	sc := ci.CidInfos[0].LatestPushedStorageConfig
	sc.Hot.Enabled = true
	opts := []pow.ApplyOption{pow.WithStorageConfig(sc), pow.WithOverride(true)}
	a, err := fr.pgClient.StorageConfig.Apply(ctx, c.String(), opts...)
	if err != nil {
		return "", fmt.Errorf("applying new storage-config: %s", err)
	}
	createdJobID := a.JobId

	// # Step 4.
	// Powergate is doing it's retrieval work. Register this JobID to track the
	// status in Tracker. When Tracker updates the Job status, it will call
	// fr.UpdateRetrievalStatus, so the retrieval process can continue with
	// further phases.
	if err := fr.tracker.TrackRetrieval(createdJobID, powToken); err != nil {
		return "", fmt.Errorf("tracking retrieval job: %s", err)
	}

	return createdJobID, nil
}

// TODO: UpdateRetrievalStatus
// Receives updates about how the jobID is running.
//
// If success:
// 1. Update the retrieval status to Finalizing
// 2. Put this acckey/jobID in some other datastore namespace
// 3. Have some other daemon deal with now on.
//
// If failed:
// 1. Switch status to failed. The end.
func (fr *FilRetrieval) UpdateRetrievalStatus(accKey string, jobID string, success bool, failureCause string) {
	r, err := fr.GetByAccountAndID(accKey, jobID)
	if err != nil {
		// go-datastore is unavailable, which is a very rare-case.
		// keep a log fo what should have happened in case we wan't to recover this
		// case manually.
		log.Errorf("getting retrieval from store (accKey:%s, jobID:%s, success:%s, failureCause: %s)", accKey, jobID, success, failureCause)
		return
	}

	if !success {
		r.Status = RetrievalStatusFailed
		r.FailureCause = failureCause
	} else {
		r.Status = RetrievalStatusMoveToBucket
	}

	txn, err := fr.ds.NewTransaction(false)
	if err != nil {
		log.Errorf("creating txn (accKey:%s, jobID:%s, success:%s, failureCause: %s)", accKey, jobID, success, failureCause)
		return
	}
	defer txn.Discard()

	// Save Retrieval with new status *and* insert this retrieval
	// in the special MoveToBucket stage, to jump into next phase
	// handeled by the daemon().
	if err := fr.save(txn, accKey, r); err != nil {
		log.Errorf("saving to datastore (accKey:%s, jobID:%s, success:%s, failureCause: %s)", accKey, jobID, success, failureCause)
		return
	}

	// If the retrieval wasn't successful, the status was changed to
	// StatusRetrievalFailed, nothing more to do here.
	if !success {
		return
	}

	// Now that we know the Cid data is available, move to next phase to move the data
	// to the bucket. We do it in phases as to avoid failing all the retrieval if the process
	// crashes, or shutdowns. When is spinned up again, we can recover the process to move the data
	// to the bucket without having retrievals stuck or forcing the user to create a new retrieval
	// starting from zero.
	//
	// The daemon() will take these retrievals, and continue with the process. After it finishes,
	// it will be removed from the pending list (reached final status).
	if err := fr.insertIntoMoveToBucketQueue(accKey, jobID); err != nil {
		log.Errorf("inserting into move-to-bucket queue (accKey:%s, jobID:%s, success:%s, failureCause: %s)", accKey, jobID, success, failureCause)
		return
	}

	if err := txn.Commit(); err != nil {
		log.Errorf("commiting txn (accKey:%s, jobID:%s, success:%s, failureCause: %s)", accKey, jobID, success, failureCause)
		return
	}
}

// TODO
func (fr *FilRetrieval) Close() error {
}

// TODO: daemon
//
// Finalizing statuses:
// 1. Create bucket from Cid.
// 2. If all good, switch to Success and push config with Hot disabled.
func (fr *FilRetrieval) daemon() {
}

func (fr *FilRetrieval) save(txn datastore.Txn, accKey string, r Retrieval) error {
	if len(accKey) == 0 || len(r.JobID) == 0 {
		return fmt.Errorf("account key and job-id can't be empty")
	}

	key := dsAccountAndIDKey(accKey, r.JobID)
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
	if err := fr.pgClient.Data.WatchLogs(
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

// ToDo: Delete?
func (s RetrievalStatus) String() string {
	switch s {
	case RetrievalStatusQueued:
		return "Queued"
	case RetrievalStatusExecuting:
		return "Executing"
	case RetrievalStatusMoveToBucket:
		return "Move to bucket"
	case RetrievalStatusSuccess:
		return "Success"
	case RetrievalStatusFailed:
		return "Failed"
	default:
		return "Invalid"
	}
}

// Datastore keys
// Keyspace is: /<account-key>/<id> -> json(Request)
func dsAccountKey(accKey string) datastore.Key {
	return datastore.NewKey(accKey)
}

func dsAccountAndIDKey(accKey string, jobID string) datastore.Key {
	return dsAccountKey(accKey).ChildString(jobID)

}
