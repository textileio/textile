package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	badger "github.com/ipfs/go-ds-badger"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	chunker "github.com/ipfs/go-ipfs-chunker"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	md "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/go-unixfs"
	"github.com/ipfs/go-unixfs/importer/balanced"
	"github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/ipfs/go-unixfs/importer/trickle"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/multiformats/go-multihash"
	tutil "github.com/textileio/go-threads/util"
)

func init() {
	ipld.Register(cid.DagProtobuf, md.DecodeProtobufBlock)
	ipld.Register(cid.Raw, md.DecodeRawBlock)
	ipld.Register(cid.DagCBOR, cbor.DecodeBlock)
}

var log = logging.Logger("local")

const (
	// repoName is the name used for the blockstore repo directory.
	repoName = ".textile/repo"
)

// bucket wraps an local and a null dag service for local operations.
type bucket struct {
	layout options.Layout
	tmp    ipld.DAGService
	local  ipld.DAGService
	store  ds.Batching
}

// NewBucket creates a new bucket with the given repo path.
func NewBucket(root string, layout options.Layout, debug bool) (*bucket, error) {
	if debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"local": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	mem := dagutils.NewMemoryDagService()
	repoPath := filepath.Join(root, repoName)
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return nil, err
	}
	store, err := badger.NewDatastore(repoPath, &badger.DefaultOptions)
	if err != nil {
		return nil, err
	}
	bs := bstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	bsrv := bserv.New(bs, offline.Exchange(bs))
	return &bucket{
		layout: layout,
		tmp:    mem,
		local:  md.NewDAGService(bsrv),
		store:  store,
	}, nil
}

// Close closes the local bucket.
func (b *bucket) Close() error {
	return b.store.Close()
}

// Save chunks files under the directory path and saves the node structure to the local dag service.
// The file nodes are not saved.
// This is useful for tracking directory state without duplicating file bytes.
func (b *bucket) Save(ctx context.Context, pth string) (ipld.Node, error) {
	return b.walkPath(ctx, b.local, pth)
}

// Get returns a saved node from the local dag service.
func (b *bucket) Get(ctx context.Context, c cid.Cid) (ipld.Node, error) {
	return b.local.Get(ctx, c)
}

// Diff returns a list of changes that are present in pth compared to the node at cid.
func (b *bucket) Diff(ctx context.Context, c cid.Cid, pth string) ([]*dagutils.Change, error) {
	an, err := b.local.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	bn, err := b.walkPath(ctx, b.tmp, pth)
	if err != nil {
		return nil, err
	}
	if err := copyDag(ctx, an, b.local, b.tmp); err != nil {
		return nil, err
	}
	return dagutils.Diff(ctx, b.tmp, an, bn)
}

// walkPath walks the path and adds files to the dag service.
func (b *bucket) walkPath(ctx context.Context, dag ipld.DAGService, pth string) (ipld.Node, error) {
	root := unixfs.EmptyDirNode()
	prefix, err := getCidBuilder()
	if err != nil {
		return nil, err
	}
	root.SetCidBuilder(prefix)
	editor := dagutils.NewDagEditor(root, dag)

	abs, err := filepath.Abs(pth)
	if err != nil {
		return nil, err
	}
	if err := filepath.Walk(abs, func(n string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			if strings.HasSuffix(n, ".DS_Store") {
				return nil
			}
			file, err := os.Open(n)
			if err != nil {
				return err
			}
			defer file.Close()
			nd, err := addFile(b.tmp, file, b.layout)
			if err != nil {
				return err
			}
			n = strings.TrimPrefix(n, abs+"/")

			log.Debugf("adding node %s with link %s", nd, n)
			if err = editor.InsertNodeAtPath(ctx, n, nd, unixfs.EmptyDirNode); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return editor.Finalize(ctx, dag)
}

// addFile chunks reader with layout and adds blocks to the dag service.
// SHA2-256 is used as the hash function and CidV1 as the cid version.
func addFile(dag ipld.DAGService, r io.Reader, layout options.Layout) (ipld.Node, error) {
	prefix, err := getCidBuilder()
	if err != nil {
		return nil, err
	}

	dbp := helpers.DagBuilderParams{
		Dagserv:    dag,
		RawLeaves:  false,
		Maxlinks:   helpers.DefaultLinksPerBlock,
		NoCopy:     false,
		CidBuilder: prefix,
	}

	chnk, err := chunker.FromString(r, "default")
	if err != nil {
		return nil, err
	}
	dbh, err := dbp.New(chnk)
	if err != nil {
		return nil, err
	}

	var n ipld.Node
	switch layout {
	case options.TrickleLayout:
		n, err = trickle.Layout(dbh)
	case options.BalancedLayout:
		n, err = balanced.Layout(dbh)
	default:
		return nil, fmt.Errorf("invalid layout")
	}
	return n, err
}

func getCidBuilder() (*cid.Prefix, error) {
	prefix, err := md.PrefixForCidVersion(1)
	if err != nil {
		return nil, err
	}
	hashFunCode := multihash.Names["sha2-256"]
	prefix.MhType = hashFunCode
	prefix.MhLength = -1
	return &prefix, nil
}

// Copied from https://github.com/ipfs/go-merkledag/blob/master/dagutils/utils.go#L210
func copyDag(ctx context.Context, nd ipld.Node, from, to ipld.DAGService) error {
	err := to.Add(ctx, nd)
	if err != nil {
		return err
	}

	for _, lnk := range nd.Links() {
		child, err := lnk.GetNode(ctx, from)
		if err != nil {
			if err == ipld.ErrNotFound {
				// not found means we didnt modify it, and it should
				// already be in the target datastore
				continue
			}
			return err
		}

		err = copyDag(ctx, child, from, to)
		if err != nil {
			return err
		}
	}
	return nil
}
