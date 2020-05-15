package dagger

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
	"github.com/multiformats/go-multihash"
	tutil "github.com/textileio/go-threads/util"
)

func init() {
	ipld.Register(cid.DagProtobuf, md.DecodeProtobufBlock)
	ipld.Register(cid.Raw, md.DecodeRawBlock)
	ipld.Register(cid.DagCBOR, cbor.DecodeBlock)
}

var log = logging.Logger("dagger")

const (
	// repoName is the name used for the blockstore repo directory.
	repoName = ".textile/repo"
)

// dagger wraps an offline and a null dag service for local operations.
type dagger struct {
	mem     ipld.DAGService
	offline ipld.DAGService
	store   ds.Batching
}

// NewDagger creates a new dagger with the given repo path.
func NewDagger(root string, debug bool) (*dagger, error) {
	if debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"dagger": logging.LevelDebug,
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
	return &dagger{
		mem:     mem,
		offline: md.NewDAGService(bsrv),
		store:   store,
	}, nil
}

// Close closes the offline blockstore.
func (d *dagger) Close() error {
	return d.store.Close()
}

// Layout indicates which type of DAG to layout.
type Layout int

const (
	// Trickle DAG layout
	Trickle Layout = iota
	// Balanced DAG layout
	Balanced
)

// SaveDir chunks files under the directory path and saves the node structure to the offline dag service.
// The file nodes are not saved.
// This is useful for tracking directory state without duplicating file bytes.
func (d *dagger) SaveDir(ctx context.Context, pth string, layout Layout) (ipld.Node, error) {
	abs, err := filepath.Abs(pth)
	if err != nil {
		return nil, err
	}

	root := unixfs.EmptyDirNode()
	prefix, err := getCidBuilder()
	root.SetCidBuilder(prefix)
	editor := dagutils.NewDagEditor(root, nil)

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
			nd, err := d.HashFile(file, layout)
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
	return editor.Finalize(ctx, d.offline)
}

// GetDir returns a saved node from the offline dag service.
func (d *dagger) GetDir(ctx context.Context, c cid.Cid) (ipld.Node, error) {
	return d.offline.Get(ctx, c)
}

// HashFile chunks reader with the given layout.
// SHA2-256 is used as the hash function and CidV1 as the cid version.
func (d *dagger) HashFile(r io.Reader, layout Layout) (ipld.Node, error) {
	return addFile(d.mem, r, layout)
}

// addFile chunks reader and adds the blocks to the provided dag service.
func addFile(dag ipld.DAGService, r io.Reader, layout Layout) (ipld.Node, error) {
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
	case Trickle:
		n, err = trickle.Layout(dbh)
	case Balanced:
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
