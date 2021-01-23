package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/ipfs/go-cid"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/v2/api/bucketsd/client"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/buckets"
	"github.com/textileio/textile/v2/cmd"
	"github.com/textileio/textile/v2/util"
)

var (
	// ErrNotABucket indicates the given path is not within a bucket.
	ErrNotABucket = errors.New("not a bucket (or any of the parent directories): .textile")
	// ErrBucketExists is used during initialization to indicate the path already contains a bucket.
	ErrBucketExists = errors.New("bucket is already initialized")
	// ErrThreadRequired indicates the operation requires a thread ID but none was given.
	ErrThreadRequired = errors.New("thread ID is required")

	flags = map[string]cmd.Flag{
		"key":    {Key: "key", DefValue: ""},
		"thread": {Key: "thread", DefValue: ""},
	}
)

// DefaultConfConfig returns the default ConfConfig.
func DefaultConfConfig() cmd.ConfConfig {
	return cmd.ConfConfig{
		Dir:       ".textile",
		Name:      "config",
		Type:      "yaml",
		EnvPrefix: "BUCK",
	}
}

// Buckets is used to create new individual buckets based on the provided clients and config.
type Buckets struct {
	config  cmd.ConfConfig
	clients *cmd.Clients
	auth    AuthFunc
}

// NewBuckets creates Buckets from clients and config.
func NewBuckets(clients *cmd.Clients, config cmd.ConfConfig) *Buckets {
	return &Buckets{clients: clients, config: config}
}

// AuthFunc is a function that's used to add additional context information
// to the outgoing API requests.
type AuthFunc func(context.Context) context.Context

// NewBucketsWithAuth creates Buckets from clients and config and auth.
func NewBucketsWithAuth(clients *cmd.Clients, config cmd.ConfConfig, auth AuthFunc) *Buckets {
	return &Buckets{clients: clients, config: config, auth: auth}
}

// Context gets a context wrapped with auth if it exists.
func (b *Buckets) Context(ctx context.Context) context.Context {
	if b.auth != nil {
		ctx = b.auth(ctx)
	}
	return ctx
}

// Clients returns the underlying clients object.
func (b *Buckets) Clients() *cmd.Clients {
	return b.clients
}

// Config contains details for a new local bucket.
type Config struct {
	// Path is the path in which the new bucket should be created (required).
	Path string
	// Key is a key of an existing bucket (optional).
	// It's value may be inflated from a --key flag or {EnvPrefix}_KEY env variable.
	Key string
	// Thread is the thread ID of the target thread (required).
	// It's value may be inflated from a --thread flag or {EnvPrefix}_THREAD env variable.
	Thread thread.ID
}

// NewConfigFromCmd returns a config by inflating values from the given cobra command and path.
// First, flags for "key" and "thread" are used if they exist.
// If still unset, the env vars {EnvPrefix}_KEY and {EnvPrefix}_THREAD are used.
func (b *Buckets) NewConfigFromCmd(c *cobra.Command, pth string) (conf Config, err error) {
	conf.Path = pth
	conf.Key = cmd.GetFlagOrEnvValue(c, "key", b.config.EnvPrefix)
	id := cmd.GetFlagOrEnvValue(c, "thread", b.config.EnvPrefix)
	if id != "" {
		conf.Thread, err = thread.Decode(id)
		if err != nil {
			return conf, err
		}
	}
	if conf.Key != "" && !conf.Thread.Defined() {
		return conf, ErrThreadRequired
	}
	return conf, nil
}

// NewBucket initializes a new bucket from the config.
// A local blockstore is created that's used to sync local changes with the remote.
// By default, this will be an unencrypted, unnamed, empty bucket.
// The remote bucket will also be created if it doesn't already exist.
// See NewOption for more info.
// If the Unfreeze flag is set, the bucket gets created async thus `*Bucket` return
// value will nil (i.e: the method return will be (nil, nil))
func (b *Buckets) NewBucket(ctx context.Context, conf Config, opts ...NewOption) (buck *Bucket, err error) {
	args := &newOptions{}
	for _, opt := range opts {
		opt(args)
	}

	// Ensure we're not going to overwrite an existing local config
	cwd, err := filepath.Abs(conf.Path)
	if err != nil {
		return
	}
	bc, found, err := b.config.NewConfig(cwd, flags, false)
	if err != nil {
		return
	}
	if found {
		return nil, ErrBucketExists
	}

	// Check config values
	if !conf.Thread.Defined() {
		return nil, ErrThreadRequired
	}
	bc.Viper.Set("thread", conf.Thread.String())
	bc.Viper.Set("key", conf.Key)

	buck = &Bucket{
		cwd:       cwd,
		conf:      bc,
		clients:   b.clients,
		auth:      b.auth,
		pushBlock: make(chan struct{}, 1),
	}
	ctx, err = buck.context(ctx)
	if err != nil {
		return nil, err
	}

	initRemote := conf.Key == ""
	if initRemote {
		rep, err := b.clients.Buckets.Create(
			ctx,
			client.WithName(args.name),
			client.WithPrivate(args.private),
			client.WithCid(args.fromCid),
			client.WithUnfreeze(args.unfreeze))
		if err != nil {
			return nil, err
		}

		// If we're unfreezing, we simply return since the
		// bucket will get created async if the Filecoin retrieval
		// is successful. The user will `[hub] buck init -e` in the future
		// to pull the new bucket.
		if args.unfreeze {
			buck.retrID = rep.RetrievalId
			return buck, nil
		}
		buck.conf.Viper.Set("key", rep.Root.Key)

		seed := filepath.Join(cwd, buckets.SeedName)
		file, err := os.Create(seed)
		if err != nil {
			return nil, err
		}
		_, err = file.Write(rep.Seed)
		if err != nil {
			file.Close()
			return nil, err
		}
		file.Close()

		if err = buck.loadLocalRepo(ctx, cwd, b.repoName(), false); err != nil {
			return nil, err
		}
		if err = buck.repo.SaveFile(ctx, seed, buckets.SeedName); err != nil {
			return nil, err
		}
		sc, err := cid.Decode(rep.SeedCid)
		if err != nil {
			return nil, err
		}
		if err = buck.repo.SetRemotePath(buckets.SeedName, sc); err != nil {
			return nil, err
		}
		rp, err := util.NewResolvedPath(rep.Root.Path)
		if err != nil {
			return nil, err
		}
		if err = buck.repo.SetRemotePath("", rp.Cid()); err != nil {
			return nil, err
		}

		buck.links = &Links{URL: rep.Links.Url, WWW: rep.Links.Www, IPNS: rep.Links.Ipns}
	} else {
		if err := buck.loadLocalRepo(ctx, cwd, b.repoName(), true); err != nil {
			return nil, err
		}
		r, err := buck.Roots(ctx)
		if err != nil {
			return nil, err
		}
		if err = buck.repo.SetRemotePath("", r.Remote); err != nil {
			return nil, err
		}

		if _, err = buck.RemoteLinks(ctx, ""); err != nil {
			return nil, err
		}
	}

	// Write the local config to disk
	dir := filepath.Join(cwd, buck.conf.Dir)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return
	}
	config := filepath.Join(dir, buck.conf.Name+".yml")
	if err = buck.conf.Viper.WriteConfigAs(config); err != nil {
		return
	}
	cfile, err := filepath.Abs(config)
	if err != nil {
		return
	}
	buck.conf.Viper.SetConfigFile(cfile)

	// Pull remote bucket contents
	if !initRemote || args.fromCid.Defined() {
		if err := buck.repo.Save(ctx); err != nil {
			return nil, err
		}
		switch args.strategy {
		case Soft, Hybrid:
			diff, missing, remove, err := buck.diffPath(ctx, "", cwd, args.strategy == Hybrid)
			if err != nil {
				return nil, err
			}
			if err = stashChanges(diff); err != nil {
				return nil, err
			}
			if _, err = buck.handleChanges(ctx, missing, remove, args.events); err != nil {
				return nil, err
			}
			if err := buck.repo.Save(ctx); err != nil {
				return nil, err
			}
			if err = applyChanges(diff); err != nil {
				return nil, err
			}
		case Hard:
			if _, err := buck.getPath(ctx, "", cwd, nil, false, args.events); err != nil {
				return nil, err
			}
			if err := buck.repo.Save(ctx); err != nil {
				return nil, err
			}
		}
	}
	return buck, nil
}

func (b *Buckets) repoName() string {
	return filepath.Join(b.config.Dir, "repo")
}

// GetLocalBucket loads and returns the bucket at path if it exists.
func (b *Buckets) GetLocalBucket(ctx context.Context, conf Config) (*Bucket, error) {
	cwd, err := filepath.Abs(conf.Path)
	if err != nil {
		return nil, err
	}
	bc, found, err := b.config.NewConfig(cwd, flags, false)
	if err != nil {
		return nil, err
	}
	if conf.Thread.Defined() {
		bc.Viper.Set("thread", conf.Thread.String())
	}
	if conf.Key != "" {
		bc.Viper.Set("key", conf.Key)
	}
	if bc.Viper.Get("thread") == nil || bc.Viper.Get("key") == nil {
		return nil, ErrNotABucket
	}
	cmd.ExpandConfigVars(bc.Viper, bc.Flags)
	buck := &Bucket{
		cwd:       cwd,
		conf:      bc,
		clients:   b.clients,
		auth:      b.auth,
		pushBlock: make(chan struct{}, 1),
	}
	if found {
		bp, err := buck.Path()
		if err != nil {
			return nil, err
		}
		if err = buck.loadLocalRepo(ctx, bp, b.repoName(), true); err != nil {
			return nil, err
		}
	}
	return buck, nil
}

// RemoteBuckets lists all existing remote buckets in the thread.
// If id is not defined, this will return buckets from all threads.
// In a hub context, this will only list buckets that the context
// has access to.
func (b *Buckets) RemoteBuckets(ctx context.Context, id thread.ID) (list []Info, err error) {
	ctx = b.Context(ctx)
	var threads []cmd.Thread
	if id.Defined() {
		threads = []cmd.Thread{{ID: id}}
	} else {
		threads = b.clients.ListThreads(ctx, true)
	}
	for _, t := range threads {
		ctx = common.NewThreadIDContext(ctx, t.ID)
		res, err := b.clients.Buckets.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, root := range res.Roots {
			info, err := pbRootToInfo(root)
			if err != nil {
				return nil, err
			}
			list = append(list, info)
		}
	}
	return list, nil
}
