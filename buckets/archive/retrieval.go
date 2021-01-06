package archive

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	powc "github.com/textileio/powergate/api/client"
)

type RetrievalID uint64
type Status int

const (
	StatusQueued Status = iota
	StatusExecuting
	StatusSuccess
	StatusFailed
)

type Retrieval struct {
	Id       RetrievalID
	Cid      cid.Cid
	Selector string
	BuckKey  string
	BuckPath string
	JobID    string
	Status   Status
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
func (s Status) String() string {
	switch s {
	case StatusQueued:
		return "Queued"
	case StatusExecuting:
		return "Executing"
	case StatusSuccess:
		return "Success"
	case StatusFailed:
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
