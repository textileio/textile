package collection

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/jsonschema"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/interface-go-ipfs-core/path"
	dbc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	db "github.com/textileio/go-threads/db"
	tutil "github.com/textileio/go-threads/util"
	powc "github.com/textileio/powergate/api/client"
	"github.com/textileio/powergate/ffs"
	"github.com/textileio/textile/buckets"
	"github.com/textileio/textile/collections"
	"github.com/textileio/textile/util"
)

var (
	log = logging.Logger("buckets")

	schema  *jsonschema.Schema
	indexes = []db.Index{{
		Path: "path",
	}}

	// ffsDefaultCidConfig is a default hardcoded CidConfig to be used
	// on newly created FFS instances as the default CidConfig of archived Cids,
	// if none is provided in constructor.
	ffsDefaultCidConfig = ffs.DefaultConfig{
		Hot: ffs.HotConfig{
			Enabled:       false,
			AllowUnfreeze: true,
			Ipfs: ffs.IpfsConfig{
				AddTimeout: 60 * 2,
			},
		},
		Cold: ffs.ColdConfig{
			Enabled: true,
			Filecoin: ffs.FilConfig{
				RepFactor:       10,     // Aim high for testnet
				DealMinDuration: 200000, // ~2 months
			},
		},
	}
)

// Bucket represents the buckets threaddb collection schema.
type Bucket struct {
	Key       string   `json:"_id"`
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	EncKey    string   `json:"key,omitempty"`
	DNSRecord string   `json:"dns_record,omitempty"`
	Archives  Archives `json:"archives"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// GetEncKey returns the encryption key as bytes if present.
func (b *Bucket) GetEncKey() []byte {
	if b.EncKey == "" {
		return nil
	}
	key, _ := base64.StdEncoding.DecodeString(b.EncKey)
	return key
}

// Archives contains all archives for a single bucket.
type Archives struct {
	Current Archive   `json:"current"`
	History []Archive `json:"history"`
}

// Archive is a single archive containing a list of deals.
type Archive struct {
	Cid   string `json:"cid"`
	Deals []Deal `json:"deals"`
}

// Deal contains details about a Filecoin deal.
type Deal struct {
	ProposalCid string `json:"proposal_cid"`
	Miner       string `json:"miner"`
}

func init() {
	reflector := jsonschema.Reflector{ExpandedStruct: true}
	schema = reflector.Reflect(&Bucket{})
}

// Buckets is a wrapper around a threaddb collection that performs object storage on IPFS and Filecoin.
type Buckets struct {
	ffsCol   *collections.FFSInstances
	threads  *dbc.Client
	pgClient *powc.Client

	buckCidConfig ffs.DefaultConfig

	lock   sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New returns a new buckets collection mananger.
func New(t *dbc.Client, pgc *powc.Client, col *collections.FFSInstances, defaultCidConfig *ffs.DefaultConfig, debug bool) (*Buckets, error) {
	if debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"buckets": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}
	buckCidConfig := ffsDefaultCidConfig
	if defaultCidConfig != nil {
		buckCidConfig = *defaultCidConfig
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Buckets{
		ffsCol:   col,
		threads:  t,
		pgClient: pgc,

		buckCidConfig: buckCidConfig,

		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Create a bucket instance.
func (b *Buckets) Create(ctx context.Context, dbID thread.ID, key string, pth path.Path, opts ...Option) (*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	var encKey string
	if args.Key != nil {
		encKey = base64.StdEncoding.EncodeToString(args.Key)
	}
	now := time.Now().UnixNano()
	bucket := &Bucket{
		Key:       key,
		Name:      args.Name,
		Path:      pth.String(),
		EncKey:    encKey,
		Archives:  Archives{Current: Archive{Deals: []Deal{}}, History: []Archive{}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	ids, err := b.threads.Create(ctx, dbID, buckets.CollectionName, dbc.Instances{bucket}, db.WithTxnToken(args.Token))
	if isColNotFoundErr(err) {
		if err := b.addCollection(ctx, dbID, opts...); err != nil {
			return nil, err
		}
		return b.Create(ctx, dbID, key, pth, opts...)
	}
	if isInvalidSchemaErr(err) {
		if err := b.updateCollection(ctx, dbID, opts...); err != nil {
			return nil, err
		}
		return b.Create(ctx, dbID, key, pth, opts...)
	}
	if err != nil {
		return nil, fmt.Errorf("creating bucket in thread: %s", err)
	}
	bucket.Key = ids[0]

	if err := b.createFFSInstance(ctx, key); err != nil {
		return nil, fmt.Errorf("creating FFS instance for bucket: %s", err)
	}

	return bucket, nil
}

// IsArchivingEnabled returns whether or not Powergate archiving is enabled.
func (b *Buckets) IsArchivingEnabled() bool {
	return b.pgClient != nil
}

func (b *Buckets) createFFSInstance(ctx context.Context, bucketKey string) error {
	// If the Powergate client isn't configured, don't do anything.
	if b.pgClient == nil {
		return nil
	}
	_, token, err := b.pgClient.FFS.Create(ctx)
	if err != nil {
		return fmt.Errorf("creating FFS instance: %s", err)
	}

	ctxFFS := context.WithValue(ctx, powc.AuthKey, token)
	i, err := b.pgClient.FFS.Info(ctxFFS)
	if err != nil {
		return fmt.Errorf("getting information about created ffs instance: %s", err)
	}
	waddr := i.Balances[0].Addr
	if err := b.ffsCol.Create(ctx, bucketKey, token, waddr); err != nil {
		return fmt.Errorf("saving FFS instances data: %s", err)
	}
	defaultBucketCidConfig := ffs.DefaultConfig{
		Cold:       b.buckCidConfig.Cold,
		Hot:        b.buckCidConfig.Hot,
		Repairable: b.buckCidConfig.Repairable,
	}
	defaultBucketCidConfig.Cold.Filecoin.Addr = waddr
	if err := b.pgClient.FFS.SetDefaultConfig(ctxFFS, defaultBucketCidConfig); err != nil {
		return fmt.Errorf("setting default bucket FFS cidconfig: %s", err)
	}
	return nil
}

// Get a bucket instance.
func (b *Buckets) Get(ctx context.Context, dbID thread.ID, key string, opts ...Option) (*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	buck := &Bucket{}
	err := b.threads.FindByID(ctx, dbID, buckets.CollectionName, key, buck, db.WithTxnToken(args.Token))
	if isColNotFoundErr(err) {
		if err := b.addCollection(ctx, dbID, opts...); err != nil {
			return nil, err
		}
		return b.Get(ctx, dbID, key, opts...)
	}
	if err != nil {
		return nil, fmt.Errorf("getting bucket in thread: %s", err)
	}
	return buck, nil
}

// List bucket instances.
func (b *Buckets) List(ctx context.Context, dbID thread.ID, opts ...Option) ([]*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	res, err := b.threads.Find(ctx, dbID, buckets.CollectionName, &db.Query{}, &Bucket{}, db.WithTxnToken(args.Token))
	if isColNotFoundErr(err) {
		if err := b.addCollection(ctx, dbID, opts...); err != nil {
			return nil, err
		}
		return b.List(ctx, dbID, opts...)
	}
	if err != nil {
		return nil, fmt.Errorf("listing bucket in thread: %s", err)
	}
	return res.([]*Bucket), nil
}

// Save a bucket instance.
func (b *Buckets) Save(ctx context.Context, dbID thread.ID, bucket *Bucket, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	ensureNoNulls(bucket)
	err := b.threads.Save(ctx, dbID, buckets.CollectionName, dbc.Instances{bucket}, db.WithTxnToken(args.Token))
	if isInvalidSchemaErr(err) {
		if err := b.updateCollection(ctx, dbID, opts...); err != nil {
			return err
		}
		return b.Save(ctx, dbID, bucket, opts...)
	}
	if err != nil {
		return fmt.Errorf("saving bucket in thread: %s", err)
	}
	return nil

}

func ensureNoNulls(b *Bucket) {
	if len(b.Archives.History) == 0 {
		current := b.Archives.Current
		if len(current.Deals) == 0 {
			b.Archives.Current = Archive{Deals: []Deal{}}
		}
		b.Archives = Archives{Current: current, History: []Archive{}}
	}
}

// Delete a bucket instance.
func (b *Buckets) Delete(ctx context.Context, dbID thread.ID, key string, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	err := b.threads.Delete(ctx, dbID, buckets.CollectionName, []string{key}, db.WithTxnToken(args.Token))
	if err != nil {
		return fmt.Errorf("deleting bucket in thread: %s", err)
	}
	return nil
}

// Archive pushes the current root Cid to the corresponding FFS instance of the bucket.
// The behaviour changes depending on different cases, depending on a previous archive.
// 0. No previous archive or last one aborted: simply pushes the Cid to the FFS instance.
// 1. Last archive exists with the same Cid:
//   a. Last archive Successful: fails, there's nothing to do.
//   b. Last archive Executing/Queued: fails, that work already starting and is in progress.
//   c. Last archive Failed/Canceled: work to do, push again with override flag to try again.
// 2. Archiving on new Cid: work to do, it will always call Replace(,) in the FFS instance.
func (b *Buckets) Archive(ctx context.Context, dbID thread.ID, key string, newCid cid.Cid, opts ...Option) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	ffsi, err := b.ffsCol.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("getting ffs instance data: %s", err)
	}

	ctxFFS := context.WithValue(ctx, powc.AuthKey, ffsi.FFSToken)

	// Check that FFS wallet addr balance is > 0, if not, fail fast.
	bal, err := b.pgClient.Wallet.WalletBalance(ctx, ffsi.WalletAddr)
	if err != nil {
		return fmt.Errorf("getting ffs wallet address balance: %s", err)
	}
	if bal == 0 {
		return buckets.ErrZeroBalance
	}

	var jid ffs.JobID
	firstTimeArchive := ffsi.Archives.Current.JobID == ""
	if firstTimeArchive || ffsi.Archives.Current.Aborted { // Case 0.
		// On the first archive, we simply push the Cid with
		// the default CidConfig configured at bucket creation.
		jid, err = b.pgClient.FFS.PushConfig(ctxFFS, newCid, powc.WithOverride(true))
		if err != nil {
			return fmt.Errorf("pushing config: %s", err)
		}
	} else {
		oldCid, err := cid.Cast(ffsi.Archives.Current.Cid)
		if err != nil {
			return fmt.Errorf("parsing old Cid archive: %s", err)
		}

		if oldCid.Equals(newCid) { // Case 1.
			switch ffs.JobStatus(ffsi.Archives.Current.JobStatus) {
			// Case 1.a.
			case ffs.Success:
				return fmt.Errorf("the same bucket cid is already archived successfully")
			// Case 1.b.
			case ffs.Executing, ffs.Queued:
				return fmt.Errorf("there is an in progress archive")
			// Case 1.c.
			case ffs.Failed, ffs.Canceled:
				jid, err = b.pgClient.FFS.PushConfig(ctxFFS, newCid, powc.WithOverride(true))
				if err != nil {
					return fmt.Errorf("pushing config: %s", err)
				}
			default:
				return fmt.Errorf("unexpected current archive status: %d", ffsi.Archives.Current.JobStatus)
			}
		} else { // Case 2.
			jid, err = b.pgClient.FFS.Replace(ctxFFS, oldCid, newCid)
			if err != nil {
				return fmt.Errorf("replacing cid: %s", err)
			}
		}

		// Include the existing archive in history,
		// since we're going to set a new _current_ archive.
		ffsi.Archives.History = append(ffsi.Archives.History, ffsi.Archives.Current)
	}
	ffsi.Archives.Current = collections.Archive{
		Cid:       newCid.Bytes(),
		CreatedAt: time.Now().Unix(),
		JobID:     jid.String(),
		JobStatus: int(ffs.Queued),
	}
	if err := b.ffsCol.Replace(ctx, ffsi); err != nil {
		return fmt.Errorf("updating ffs instance data: %s", err)
	}
	go b.trackArchiveProgress(util.NewClonedContext(ctx), dbID, key, jid, ffsi.FFSToken, newCid, opts...)

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
		return ffs.Failed, "", buckets.ErrNoCurrentArchive
	}
	current := ffsi.Archives.Current
	if current.Aborted {
		return ffs.Failed, "", fmt.Errorf("job status tracking was aborted: %s", current.AbortedMsg)
	}
	return ffs.JobStatus(current.JobStatus), current.FailureMsg, nil
}

// ArchiveWatch allows to have the last log execution for the last archive, plus realtime
// human-friendly log output of how the current archive is executing.
// If the last archive is already done, it will simply return the log history and close the channel.
func (b *Buckets) ArchiveWatch(ctx context.Context, key string, ch chan<- string) error {
	ffsi, err := b.ffsCol.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("getting ffs instance data: %s", err)
	}

	if ffsi.Archives.Current.JobID == "" {
		return buckets.ErrNoCurrentArchive
	}
	current := ffsi.Archives.Current
	if current.Aborted {
		return fmt.Errorf("job status tracking was aborted: %s", current.AbortedMsg)
	}
	c, err := cid.Cast(current.Cid)
	if err != nil {
		return fmt.Errorf("parsing current archive cid: %s", err)
	}
	ctx = context.WithValue(ctx, powc.AuthKey, ffsi.FFSToken)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ffsCh := make(chan powc.LogEvent)
	if err := b.pgClient.FFS.WatchLogs(ctx, ffsCh, c, powc.WithJidFilter(ffs.JobID(current.JobID)), powc.WithHistory(true)); err != nil {
		return fmt.Errorf("watching log events in Powergate: %s", err)
	}
	for le := range ffsCh {
		if le.Err != nil {
			return le.Err
		}
		ch <- le.LogEntry.Msg
	}
	return nil
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
	if err := b.pgClient.FFS.WatchJobs(ctx, ch, jid); err != nil {
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
	buck, err := b.Get(ctx, dbID, key, opts...)
	if err != nil {
		return fmt.Errorf("getting bucket for save deals: %s", err)
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

	buck.UpdatedAt = time.Now().UnixNano()
	if err = b.Save(ctx, dbID, buck, opts...); err != nil {
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
		Name:    buckets.CollectionName,
		Schema:  schema,
		Indexes: indexes,
	}, db.WithManagedToken(args.Token))
}

func (b *Buckets) updateCollection(ctx context.Context, dbID thread.ID, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	return b.threads.UpdateCollection(ctx, dbID, db.CollectionConfig{
		Name:    buckets.CollectionName,
		Schema:  schema,
		Indexes: indexes,
	}, db.WithManagedToken(args.Token))
}

type Options struct {
	Name  string
	Key   []byte
	Token thread.Token
}

type Option func(*Options)

func WithName(n string) Option {
	return func(args *Options) {
		args.Name = n
	}
}

func WithKey(k []byte) Option {
	return func(args *Options) {
		args.Key = k
	}
}

func WithToken(t thread.Token) Option {
	return func(args *Options) {
		args.Token = t
	}
}

func isColNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "collection not found")
}

func isInvalidSchemaErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "instance doesn't correspond to schema: (root)")
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
