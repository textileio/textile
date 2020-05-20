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
	bstore "github.com/ipfs/go-ipfs-blockstore"
	chunker "github.com/ipfs/go-ipfs-chunker"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	md "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/go-unixfs"
	"github.com/ipfs/go-unixfs/importer/balanced"
	"github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/ipfs/go-unixfs/importer/trickle"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	car "github.com/ipld/go-car"
)

func init() {
	ipld.Register(cid.DagProtobuf, md.DecodeProtobufBlock)
	ipld.Register(cid.Raw, md.DecodeRawBlock)
	ipld.Register(cid.DagCBOR, cbor.DecodeBlock)
}

var (
	// ignoredFilenames is a list of default ignored file names.
	ignoredFilenames = []string{
		".DS_Store",
	}
)

const (
	// archiveName is the name used for the car archive.
	archiveName = ".textile/buck.car"
)

// Bucket tracks a local bucket tree structure.
type Bucket struct {
	path   string
	root   cid.Cid
	tmp    ipld.DAGService
	store  bstore.Blockstore
	layout options.Layout
}

// NewBucket creates a new bucket with the given path.
func NewBucket(pth string, layout options.Layout) (*Bucket, error) {
	bs := bstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	bsrv := bserv.New(bs, offline.Exchange(bs))
	b := &Bucket{
		path:   pth,
		tmp:    md.NewDAGService(bsrv),
		store:  bs,
		layout: layout,
	}
	_ = b.load()
	return b, nil
}

// load the car archive into the tmp blockstore.
func (b *Bucket) load() error {
	file, err := os.Open(filepath.Join(b.path, archiveName))
	if err != nil {
		return err
	}
	defer file.Close()
	h, err := car.LoadCar(b.store, file)
	if err != nil {
		return err
	}
	if len(h.Roots) > 0 {
		b.root = h.Roots[0]
	}
	return nil
}

// Path returns the current archive's root cid.
func (b *Bucket) Path() path.Resolved {
	return path.IpfsPath(b.root)
}

// Get returns the node at cid from the current archive.
func (b *Bucket) Get(ctx context.Context, c cid.Cid) (ipld.Node, error) {
	return b.tmp.Get(ctx, c)
}

// Archive creates an archive describing the current path.
func (b *Bucket) Archive(ctx context.Context) error {
	n, err := b.walkPath(ctx, b.path)
	if err != nil {
		return err
	}
	file, err := b.createArchive()
	if err != nil {
		return err
	}
	defer file.Close()
	if err = car.WriteCarWithWalker(ctx, b.tmp, []cid.Cid{n.Cid()}, file, carWalker); err != nil {
		return err
	}
	b.root = n.Cid()
	return nil
}

func carWalker(n ipld.Node) ([]*ipld.Link, error) {
	var links []*ipld.Link
	for _, l := range n.Links() {
		if l.Name != "" {
			links = append(links, l)
		}
	}
	return links, nil
}

// walkPath walks path and adds files to the dag service.
func (b *Bucket) walkPath(ctx context.Context, pth string) (ipld.Node, error) {
	root := unixfs.EmptyDirNode()
	prefix, err := md.PrefixForCidVersion(1)
	if err != nil {
		return nil, err
	}
	root.SetCidBuilder(prefix)
	editor := dagutils.NewDagEditor(root, b.tmp)
	abs, err := filepath.Abs(pth)
	if err != nil {
		return nil, err
	}
	if err = filepath.Walk(abs, func(n string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			if Ignore(n) {
				return nil
			}
			p := n
			n = strings.TrimPrefix(n, abs+"/")
			if strings.HasPrefix(n, filepath.Dir(archiveName)+"/") {
				return nil
			}
			file, err := os.Open(p)
			if err != nil {
				return err
			}
			defer file.Close()
			nd, err := addFile(b.tmp, b.layout, file)
			if err != nil {
				return err
			}
			if err = editor.InsertNodeAtPath(ctx, n, nd, unixfs.EmptyDirNode); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return editor.Finalize(ctx, b.tmp)
}

// createArchive creates the empty archive file under path.
func (b *Bucket) createArchive() (*os.File, error) {
	name := filepath.Join(b.path, archiveName)
	if err := os.MkdirAll(filepath.Dir(name), os.ModePerm); err != nil {
		return nil, err
	}
	return os.Create(name)
}

// ArchiveFile creates an archive describing a directory containing reader.
func (b *Bucket) ArchiveFile(ctx context.Context, pth string, name string) error {
	r, err := os.Open(pth)
	if err != nil {
		return err
	}
	defer r.Close()
	root := unixfs.EmptyDirNode()
	prefix, err := md.PrefixForCidVersion(1)
	if err != nil {
		return err
	}
	root.SetCidBuilder(prefix)
	editor := dagutils.NewDagEditor(root, b.tmp)
	f, err := addFile(b.tmp, b.layout, r)
	if err != nil {
		return err
	}
	if err = editor.InsertNodeAtPath(ctx, name, f, unixfs.EmptyDirNode); err != nil {
		return err
	}
	n, err := editor.Finalize(ctx, b.tmp)
	if err != nil {
		return err
	}
	file, err := b.createArchive()
	if err != nil {
		return err
	}
	defer file.Close()
	if err = car.WriteCarWithWalker(ctx, b.tmp, []cid.Cid{n.Cid()}, file, carWalker); err != nil {
		return err
	}
	b.root = n.Cid()
	return nil
}

// HashFile returns the cid of the file at path.
func (b *Bucket) HashFile(pth string) (cid.Cid, error) {
	r, err := os.Open(pth)
	if err != nil {
		return cid.Undef, err
	}
	defer r.Close()
	n, err := addFile(b.tmp, b.layout, r)
	if err != nil {
		return cid.Undef, err
	}
	return n.Cid(), nil
}

// Diff returns a list of changes that are present in pth compared to the current archive.
func (b *Bucket) Diff(ctx context.Context, pth string) ([]*dagutils.Change, error) {
	an, err := b.tmp.Get(ctx, b.root)
	if err != nil {
		return nil, err
	}
	bn, err := b.walkPath(ctx, pth)
	if err != nil {
		return nil, err
	}
	return dagutils.Diff(ctx, b.tmp, an, bn)
}

// Ignore returns true if the path contains an ignored file.
func Ignore(pth string) bool {
	for _, n := range ignoredFilenames {
		if strings.HasSuffix(pth, n) {
			return true
		}
	}
	return false
}

// addFile chunks reader with layout and adds blocks to the dag service.
// SHA2-256 is used as the hash function and CidV1 as the cid version.
func addFile(dag ipld.DAGService, layout options.Layout, r io.Reader) (ipld.Node, error) {
	prefix, err := md.PrefixForCidVersion(1)
	if err != nil {
		return nil, err
	}
	dbp := helpers.DagBuilderParams{
		Dagserv:    dag,
		RawLeaves:  true,
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
