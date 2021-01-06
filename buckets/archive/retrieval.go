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
	powc "github.com/textileio/powergate/api/client"
)

type RetrievalID uint64

type RetrievalStatus int

const (
	RetrievalStatusQueued RetrievalStatus = iota
	RetrievalStatusExecuting
	RetrievalStatusSuccess
	RetrievalStatusFailed
)

type RetrievalType int

const (
	RetrievalTypeNewBucket RetrievalType = iota
	RetrievalTypeExistingBucket
)

type Retrieval struct {
	Type      RetrievalType
	JobID     string
	Cid       cid.Cid
	Selector  string
	Status    RetrievalStatus
	CreatedAt uint64

	// If Type == RetrievalTypeNewBucket
	Name    string
	Private bool

	// If Type == RetrievalTypeExistingBucket
	BuckKey  string
	BuckPath string
}

// FilRetrieval manages Retrievals from Accounts in
// go-datastore.
type FilRetrieval struct {
	ds       datastore.Datastore
	pgClient *powc.Client
}

func NewFilRetrieval(ds datastore.Datastore, pgClient *powc.Client) (*FilRetrieval, error) {
	return &FilRetrieval{
		ds:       ds,
		pgClient: pgClient,
	}, nil
}

func (fr *FilRetrieval) CreateForNewBucket(
	dbID thread.ID,
	dbToken thread.Token,
	bucketName string,
	bucketPrivate bool,
	dataCid cid.Cid,
) error {
	r := Retrieval{
		Type:      RetrievalTypeNewBucket,
		JobID:     XXXX,
		Cid:       dataCid,
		Selector:  "",
		Status:    RetrievalStatusQueued,
		CreatedAt: time.Now().Unix(),
	}

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

func (fr *FilRetrieval) GetByAccountAndID(accKey string, ID RetrievalID) (Retrieval, error) {
	key := dsAccountAndIDKey(accKey, ID)

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

func (fr *FilRetrieval) Logs(ctx context.Context, accKey string, ID RetrievalID, powToken string, ch chan<- string) error {
	r, err := fr.GetByAccountAndID(accKey, ID)
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

func dsAccountAndIDKey(accKey string, ID RetrievalID) datastore.Key {
	return dsAccountKey(accKey).ChildString(fmt.Sprintf("%d", ID))

}
