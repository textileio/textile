package buckets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/jsonschema"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	dbc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	db "github.com/textileio/go-threads/db"
	powc "github.com/textileio/powergate/api/client"
	"github.com/textileio/powergate/ffs"
	"github.com/textileio/textile/collections"
	"github.com/textileio/textile/util"
)

const CollectionName = "buckets"

var (
	// ErrNoCurrentArchive is returned when not status about the last archive
	// can be retrieved, since the bucket was never archived.
	ErrNoCurrentArchive = errors.New("the bucket was never archived")
)

var (
	schema  *jsonschema.Schema
	indexes = []db.IndexConfig{{
		Path: "path",
	}}
)

type Bucket struct {
	Key       string   `json:"_id"`
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	DNSRecord string   `json:"dns_record,omitempty"`
	Archives  Archives `json:"archives"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

type Archives struct {
	Current Archive   `json:"current"`
	History []Archive `json:"history"`
}

type Archive struct {
	Cid   string `json:"cid"`
	Deals []Deal `json:"deals"`
}

type Deal struct {
	ProposalCid string `json:"proposalCid"`
	Miner       string `json:"miner"`
}

func init() {
	reflector := jsonschema.Reflector{ExpandedStruct: true}
	schema = reflector.Reflect(&Bucket{})
}

type Buckets struct {
	ffsCol   *collections.FFSInstances
	threads  *dbc.Client
	pgClient *powc.Client

	lock   sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(t *dbc.Client, pgc *powc.Client, col *collections.FFSInstances) *Buckets {
	ctx, cancel := context.WithCancel(context.Background())
	return &Buckets{
		ffsCol:   col,
		threads:  t,
		pgClient: pgc,

		ctx:    ctx,
		cancel: cancel,
	}
}

func (b *Buckets) Create(ctx context.Context, dbID thread.ID, key, name string, pth path.Path, opts ...Option) (*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	now := time.Now().UnixNano()
	bucket := &Bucket{
		Key:       key,
		Name:      name,
		Path:      pth.String(),
		Archives:  Archives{Current: Archive{Deals: []Deal{}}, History: []Archive{}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	ids, err := b.threads.Create(ctx, dbID, CollectionName, dbc.Instances{bucket}, db.WithTxnToken(args.Token))
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID, opts...); err != nil {
				return nil, err
			}
			return b.Create(ctx, dbID, key, name, pth, opts...)
		}
		return nil, err
	}
	bucket.Key = ids[0]

	if err := createFFSInstance(ctx, b.ffsCol, b.pgClient, key); err != nil {
		return nil, fmt.Errorf("creating FFS instance for bucket: %s", err)
	}

	return bucket, nil
}

func (b *Buckets) IsArchivingEnabled() bool {
	return b.pgClient != nil
}

func createFFSInstance(ctx context.Context, col *collections.FFSInstances, c *powc.Client, bucketKey string) error {
	// If the Powergate client isn't configured, don't do anything.
	if c == nil {
		return nil
	}
	_, token, err := c.FFS.Create(ctx)
	if err != nil {
		return fmt.Errorf("creating FFS instance: %s", err)
	}
	if err := col.Create(ctx, bucketKey, token); err != nil {
		return fmt.Errorf("saving FFS instances data: %s", err)
	}
	return nil
}

func (b *Buckets) Get(ctx context.Context, dbID thread.ID, key string, opts ...Option) (*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	buck := &Bucket{}
	if err := b.threads.FindByID(ctx, dbID, CollectionName, key, buck, db.WithTxnToken(args.Token)); err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID, opts...); err != nil {
				return nil, err
			}
			return b.Get(ctx, dbID, key, opts...)
		}
		return nil, err
	}
	return buck, nil
}

func (b *Buckets) List(ctx context.Context, dbID thread.ID, opts ...Option) ([]*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	res, err := b.threads.Find(ctx, dbID, CollectionName, &db.Query{}, &Bucket{}, db.WithTxnToken(args.Token))
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID, opts...); err != nil {
				return nil, err
			}
			return b.List(ctx, dbID, opts...)
		}
		return nil, err
	}
	return res.([]*Bucket), nil
}

func (b *Buckets) Save(ctx context.Context, dbID thread.ID, bucket *Bucket, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	return b.threads.Save(ctx, dbID, CollectionName, dbc.Instances{bucket}, db.WithTxnToken(args.Token))
}

func (b *Buckets) Delete(ctx context.Context, dbID thread.ID, key string, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	return b.threads.Delete(ctx, dbID, CollectionName, []string{key}, db.WithTxnToken(args.Token))
}

func (b *Buckets) Archive(ctx context.Context, dbID thread.ID, key string, c cid.Cid, opts ...Option) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	ffsi, err := b.ffsCol.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("getting ffs instance data: %s", err)
	}

	ctxFFS := context.WithValue(ctx, powc.AuthKey, ffsi.FFSToken)
	var jid ffs.JobID
	if ffsi.Archives.Current.JobID != "" {
		ffsi.Archives.History = append(ffsi.Archives.History, ffsi.Archives.Current)
		oldCid, err := cid.Decode(ffsi.Archives.Current.Cid)
		if err != nil {
			return fmt.Errorf("parsing old Cid archive: %s", err)
		}
		jid, err = b.pgClient.FFS.Replace(ctxFFS, oldCid, c)
		if err != nil {
			return fmt.Errorf("replacing cid: %s", err)
		}
	} else {
		jid, err = b.pgClient.FFS.PushConfig(ctxFFS, c)
		if err != nil {
			return fmt.Errorf("pushing config: %s", err)
		}
	}

	ffsi.Archives.Current = collections.Archive{
		Cid:       c.String(),
		CreatedAt: time.Now().Unix(),
		JobID:     jid.String(),
		JobStatus: int(ffs.Queued),
	}
	if err := b.ffsCol.Replace(ctx, ffsi); err != nil {
		return fmt.Errorf("updating ffs instance data: %s", err)
	}
	go b.trackArchiveProgress(util.NewClonedContext(ctx), dbID, key, jid, ffsi.FFSToken, c, opts...)

	return nil
}

// ArchiveStatus returns the last known archive status on Powergate. If the return status is Failed,
// an extra string with the error message is provided.
func (b *Buckets) ArchiveStatus(ctx context.Context, key string) (ffs.JobStatus, string, error) {
	ffsi, err := b.ffsCol.Get(ctx, key)
	if err != nil {
		return ffs.Failed, "", fmt.Errorf("getting ffs instance data: %s", err)
	}

	if ffsi.Archives.Current.JobID == "" {
		return ffs.Failed, "", ErrNoCurrentArchive
	}
	current := ffsi.Archives.Current
	if current.Aborted {
		return ffs.Failed, "", fmt.Errorf("job status tracking was aborted: %s", current.AbortedMsg)
	}
	return ffs.JobStatus(current.JobStatus), current.FailureMsg, nil
}

func (b *Buckets) Close() error {
	b.cancel()
	b.wg.Wait()
	return nil
}

func (b *Buckets) trackArchiveProgress(ctx context.Context, dbID thread.ID, key string, jid ffs.JobID, ffsToken string, c cid.Cid, opts ...Option) {
	b.wg.Add(1)
	defer b.wg.Done()

	// Step 1: watch for the Job status, and keep updating on Mongo until
	// we're in a final state.
	ctx = context.WithValue(ctx, powc.AuthKey, ffsToken)
	ch := make(chan powc.JobEvent, 1)
	if err := b.pgClient.FFS.WatchJobs(ctx, ch, ffs.JobID(jid)); err != nil {
		log.Errorf("watching current job %s for bucket %s: %s", jid, key, err)
		return
	}

	var aborted bool
	var abortMsg string
	var job ffs.Job
Loop:
	for {
		select {
		case <-b.ctx.Done():
			log.Infof("job %s status watching canceled", jid)
			return
		case s, ok := <-ch:
			if !ok {
				log.Errorf("getting job %s status stream closed", jid)
				aborted = true
				abortMsg = "server closed communication channel"
				break Loop
			}
			if s.Err != nil {
				log.Errorf("job %s update: %s", jid, s.Err)
				aborted = true
				abortMsg = s.Err.Error()
				break Loop
			}
			if isJobStatusFinal(s.Job.Status) {
				job = s.Job
				log.Infof("archive job for bucket %s reached final state %s", key, ffs.JobStatusStr[s.Job.Status])
				break Loop
			}
		}
	}

	if err := b.updateArchiveStatus(ctx, key, jid.String(), job, aborted, abortMsg); err != nil {
		log.Errorf("updating archive status: %s", err)
	}

	// Step 2: On success, save Deal data in the underlying Bucket thread. On
	// failure save the error message.
	if job.Status == ffs.Success {
		if err := b.saveDealsInArchive(ctx, key, dbID, ffsToken, c, opts...); err != nil {
			log.Errorf("saving deal data in archive: %s", err)
		}
	}
}

func (b *Buckets) saveDealsInArchive(ctx context.Context, key string, dbID thread.ID, ffsToken string, c cid.Cid, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	buck := &Bucket{}
	if err := b.threads.FindByID(ctx, dbID, CollectionName, key, buck, db.WithTxnToken(args.Token)); err != nil {
		return fmt.Errorf("finding bucket: %s", err)
	}
	ctxFFS := context.WithValue(ctx, powc.AuthKey, ffsToken)
	sh, err := b.pgClient.FFS.Show(ctxFFS, c)
	if err != nil {
		return fmt.Errorf("getting cid info: %s", err)
	}

	proposals := sh.GetCidInfo().GetCold().GetFilecoin().GetProposals()

	deals := make([]Deal, len(proposals))
	for i, p := range proposals {
		deals[i] = Deal{
			ProposalCid: p.GetProposalCid(),
			Miner:       p.GetMiner(),
		}
	}
	buck.Archives.Current = Archive{
		Cid:   c.String(),
		Deals: deals,
	}

	if err = b.threads.Save(ctx, dbID, CollectionName, dbc.Instances{buck}, db.WithTxnToken(args.Token)); err != nil {
		return fmt.Errorf("saving deals in thread: %s", err)
	}
	return nil
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
func (b *Buckets) updateArchiveStatus(ctx context.Context, key string, jid string, job ffs.Job, aborted bool, abortMsg string) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	ffsi, err := b.ffsCol.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("getting ffs instance data: %s", err)
	}

	archive := &ffsi.Archives.Current
	if archive.JobID != jid {
		for i := range ffsi.Archives.History {
			if ffsi.Archives.History[i].JobID == jid {
				archive = &ffsi.Archives.History[i]
				break
			}
		}
	}
	archive.JobStatus = int(job.Status)
	archive.Aborted = aborted
	archive.AbortedMsg = abortMsg
	archive.FailureMsg = prepareFailureMsg(job)
	if err := b.ffsCol.Replace(ctx, ffsi); err != nil {
		return fmt.Errorf("updating ffs status update instance data: %s", err)
	}
	return nil
}

func (b *Buckets) addCollection(ctx context.Context, dbID thread.ID, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	return b.threads.NewCollection(ctx, dbID, db.CollectionConfig{
		Name:    CollectionName,
		Schema:  schema,
		Indexes: indexes,
	}, db.WithManagedDBToken(args.Token))
}

type Options struct {
	Token thread.Token
}

type Option func(*Options)

func WithToken(t thread.Token) Option {
	return func(args *Options) {
		args.Token = t
	}
}

func isCollNotFoundErr(err error) bool {
	return strings.Contains(err.Error(), "collection not found")
}

func isJobStatusFinal(js ffs.JobStatus) bool {
	return js == ffs.Success ||
		js == ffs.Canceled ||
		js == ffs.Failed
}

func prepareFailureMsg(job ffs.Job) string {
	if job.ErrCause == "" {
		return ""
	}
	var b strings.Builder
	_, _ = b.WriteString(job.ErrCause)
	for i, de := range job.DealErrors {
		_, _ = b.WriteString(fmt.Sprintf("\nDeal error %d: Proposal %s with miner %s, %s", i, de.ProposalCid, de.Miner, de.Message))
	}
	return b.String()
}
