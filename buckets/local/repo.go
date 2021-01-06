package local

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	flatfs "github.com/ipfs/go-ds-flatfs"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	chunker "github.com/ipfs/go-ipfs-chunker"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	md "github.com/ipfs/go-merkledag"
	du "github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/go-unixfs"
	"github.com/ipfs/go-unixfs/importer/balanced"
	"github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/ipfs/go-unixfs/importer/trickle"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	mh "github.com/multiformats/go-multihash"
)

func init() {
	bstore.BlockPrefix = ds.NewKey("")
	ipld.Register(cid.DagProtobuf, md.DecodeProtobufBlock)
	ipld.Register(cid.Raw, md.DecodeRawBlock)
	ipld.Register(cid.DagCBOR, cbor.DecodeBlock)
	gob.Register(pathMap{})
}

var (
	// patchExt is used to ignore tmp files during a pull.
	patchExt = ".buckpatch"

	// ignoredFilenames is a list of default ignored file names.
	ignoredFilenames = []string{
		".DS_Store",
	}
)

// pathMap hold details about a local path map.
type pathMap struct {
	Local  cid.Cid
	Remote cid.Cid
}

// Repo tracks a local bucket tree structure.
type Repo struct {
	path   string
	name   string
	ds     ds.Batching
	bsrv   bserv.BlockService
	dag    ipld.DAGService
	layout options.Layout
	cidver int
}

// NewRepo creates a new bucket with the given path.
func NewRepo(pth, name string, layout options.Layout) (*Repo, error) {
	repo := filepath.Join(pth, name)
	if err := os.MkdirAll(repo, os.ModePerm); err != nil {
		return nil, err
	}
	bd, err := flatfs.CreateOrOpen(repo, flatfs.NextToLast(2), true)
	if err != nil {
		return nil, err
	}
	bs := bstore.NewBlockstore(bd)
	bsrv := bserv.New(bs, offline.Exchange(bs))
	return &Repo{
		path:   pth,
		name:   name,
		ds:     bd,
		bsrv:   bsrv,
		dag:    md.NewDAGService(bsrv),
		layout: layout,
		cidver: 1,
	}, nil
}

// Path returns the repo path.
func (b *Repo) Path() string {
	return filepath.Join(b.path, b.name)
}

// Root returns the local and remote root cids.
func (b *Repo) Root() (local, remote cid.Cid, err error) {
	k, err := getPathKey("")
	if err != nil {
		return
	}
	pm, err := b.getPathMap(k)
	if errors.Is(err, ds.ErrNotFound) {
		return local, remote, nil
	} else if err != nil {
		return
	}
	return pm.Local, pm.Remote, nil
}

// getPathKey returns a flatfs safe hash of a file path.
func getPathKey(pth string) (key ds.Key, err error) {
	hash, err := mh.Encode([]byte(filepath.Clean(pth)), mh.SHA2_256)
	if err != nil {
		return
	}
	return dshelp.MultihashToDsKey(hash), nil
}

// GetPathMap returns details about a local path.
func (b *Repo) GetPathMap(pth string) (local, remote cid.Cid, err error) {
	k, err := getPathKey(pth)
	if err != nil {
		return
	}
	pm, err := b.getPathMap(k)
	if err != nil {
		return
	}
	return pm.Local, pm.Remote, nil
}

// getPathMap returns details about a local path by key.
func (b *Repo) getPathMap(k ds.Key) (m pathMap, err error) {
	v, err := b.ds.Get(k)
	if err != nil {
		return
	}
	dec := gob.NewDecoder(bytes.NewReader(v))
	var pm pathMap
	if err = dec.Decode(&pm); err != nil {
		return
	}
	return pm, nil
}

// CidVersion gets the repo cid version (0 or 1).
func (b *Repo) CidVersion() int {
	return b.cidver
}

// SetCidVersion set the repo cid version (0 or 1).
// The default version is 1.
func (b *Repo) SetCidVersion(v int) {
	b.cidver = v
}

// Save saves the bucket as a node describing the file tree at the current path.
func (b *Repo) Save(ctx context.Context) error {
	_, maps, err := b.recursiveAddPath(ctx, b.path, b.dag)
	if err != nil {
		return err
	}
	for p, c := range maps {
		if err := b.setLocalPath(p, c); err != nil {
			return err
		}
	}
	return nil
}

// setLocalPath sets the local cid for an existing path map.
func (b *Repo) setLocalPath(pth string, local cid.Cid) error {
	k, err := getPathKey(pth)
	if err != nil {
		return err
	}
	xm, err := b.getPathMap(k)
	if errors.Is(err, ds.ErrNotFound) {
		return b.putPathMap(k, pathMap{Local: local})
	}
	if err != nil {
		return err
	}
	xm.Local = local
	return b.putPathMap(k, xm)
}

// putPathMap saves a path map under key.
func (b *Repo) putPathMap(k ds.Key, pm pathMap) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(pm); err != nil {
		return err
	}
	return b.ds.Put(k, buf.Bytes())
}

// recursiveAddPath walks path and adds files to the dag service.
// This method returns the resulting root node and a list of path maps.
func (b *Repo) recursiveAddPath(
	ctx context.Context,
	pth string,
	dag ipld.DAGService,
) (ipld.Node, map[string]cid.Cid, error) {
	root := unixfs.EmptyDirNode()
	prefix, err := md.PrefixForCidVersion(b.cidver)
	if err != nil {
		return nil, nil, err
	}
	root.SetCidBuilder(prefix)
	editor := du.NewDagEditor(root, dag)
	maps := make(map[string]cid.Cid)
	abs, err := filepath.Abs(pth)
	if err != nil {
		return nil, nil, err
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
			n = strings.TrimPrefix(n, abs+string(os.PathSeparator))
			if strings.HasPrefix(n, filepath.Dir(b.name)+string(os.PathSeparator)) || strings.HasSuffix(n, patchExt) {
				return nil
			}
			file, err := os.Open(p)
			if err != nil {
				return err
			}
			defer file.Close()
			nd, err := addFile(editor.GetDagService(), b.layout, prefix, file)
			if err != nil {
				return err
			}
			if err = editor.InsertNodeAtPath(ctx, n, nd, unixfs.EmptyDirNode); err != nil {
				return err
			}
			maps[strings.TrimPrefix(p, abs+string(os.PathSeparator))] = nd.Cid()
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	en := editor.GetNode()
	if err := copyLinks(ctx, en, editor.GetDagService(), dag); err != nil {
		return nil, nil, err
	}
	maps[strings.TrimPrefix(pth, b.path)] = en.Cid()
	return en, maps, nil
}

// copyLinks recursively adds all link nodes in node to the dag service.
// Data-nodes are ignored.
// Modified from https://github.com/ipfs/go-merkledag/blob/master/dagutils/utils.go#L210
func copyLinks(ctx context.Context, nd ipld.Node, from, to ipld.DAGService) error {
	err := to.Add(ctx, nd)
	if err != nil {
		return err
	}
	for _, lnk := range nd.Links() {
		if lnk.Name == "" {
			continue
		}
		child, err := lnk.GetNode(ctx, from)
		if err != nil {
			if err == ipld.ErrNotFound {
				// not found means we didnt modify it, and it should
				// already be in the target datastore
				continue
			}
			return err
		}
		err = copyLinks(ctx, child, from, to)
		if err != nil {
			return err
		}
	}
	return nil
}

// SaveFile saves the file at path to the repo.
func (b *Repo) SaveFile(ctx context.Context, pth string, name string) error {
	r, err := os.Open(pth)
	if err != nil {
		return err
	}
	defer r.Close()
	root := unixfs.EmptyDirNode()
	prefix, err := md.PrefixForCidVersion(b.cidver)
	if err != nil {
		return err
	}
	root.SetCidBuilder(prefix)
	editor := du.NewDagEditor(root, b.dag)
	f, err := addFile(editor.GetDagService(), b.layout, prefix, r)
	if err != nil {
		return err
	}
	if err = editor.InsertNodeAtPath(ctx, name, f, unixfs.EmptyDirNode); err != nil {
		return err
	}
	n := editor.GetNode()
	if err := copyLinks(ctx, n, editor.GetDagService(), b.dag); err != nil {
		return err
	}
	if err := b.setLocalPath(pth, n.Cid()); err != nil {
		return err
	}
	return b.setLocalPath("", n.Cid())
}

// HashFile returns the cid of the file at path.
// This method does not alter the bucket.
func (b *Repo) HashFile(pth string) (cid.Cid, error) {
	r, err := os.Open(pth)
	if err != nil {
		return cid.Undef, err
	}
	defer r.Close()
	prefix, err := md.PrefixForCidVersion(b.cidver)
	if err != nil {
		return cid.Undef, err
	}
	n, err := addFile(du.NewMemoryDagService(), b.layout, prefix, r)
	if err != nil {
		return cid.Undef, err
	}
	return n.Cid(), nil
}

// GetNode returns the node at cid from the bucket.
func (b *Repo) GetNode(ctx context.Context, c cid.Cid) (ipld.Node, error) {
	return b.dag.Get(ctx, c)
}

// Diff returns a list of changes that are present in path compared to the bucket.
func (b *Repo) Diff(ctx context.Context, pth string) (diff []*du.Change, err error) {
	tmp := du.NewMemoryDagService()
	var an ipld.Node
	lc, _, err := b.Root()
	if err != nil {
		return
	}
	if lc.Defined() {
		an, err = b.dag.Get(ctx, lc)
		if err != nil {
			return
		}
	} else {
		an = unixfs.EmptyDirNode()
	}
	if err = copyLinks(ctx, an, b.dag, tmp); err != nil {
		return
	}
	if err = tmp.Add(ctx, an); err != nil {
		return
	}
	bn, _, err := b.recursiveAddPath(ctx, pth, tmp)
	if err != nil {
		return
	}
	return Diff(ctx, tmp, an, bn)
}

// SetRemotePath sets or creates a mapping from a local path to a remote cid.
func (b *Repo) SetRemotePath(pth string, remote cid.Cid) error {
	k, err := getPathKey(pth)
	if err != nil {
		return err
	}
	xm, err := b.getPathMap(k)
	if errors.Is(err, ds.ErrNotFound) {
		return b.putPathMap(k, pathMap{Remote: remote})
	}
	if err != nil {
		return err
	}
	xm.Remote = remote
	return b.putPathMap(k, xm)
}

// MatchPath returns whether or not the path exists and has matching local and remote cids.
func (b *Repo) MatchPath(pth string, local, remote cid.Cid) (bool, error) {
	k, err := getPathKey(pth)
	if err != nil {
		return false, err
	}
	m, err := b.getPathMap(k)
	if err != nil {
		return false, err
	}
	if m.Local.Defined() && m.Local.Equals(local) && m.Remote.Equals(remote) {
		return true, nil
	}
	return false, nil
}

// RemovePath removes a local path map from the store.
func (b *Repo) RemovePath(ctx context.Context, pth string) error {
	k, err := getPathKey(pth)
	if err != nil {
		return err
	}
	pm, err := b.getPathMap(k)
	if errors.Is(err, ds.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := b.dag.Remove(ctx, pm.Local); err != nil {
		if !errors.Is(err, ipld.ErrNotFound) {
			return err
		}
	}
	return b.ds.Delete(k)
}

// Close closes the store and blocks service.
func (b *Repo) Close() error {
	if err := b.ds.Close(); err != nil {
		return err
	}
	if err := b.bsrv.Close(); err != nil {
		return err
	}
	return nil
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
func addFile(dag ipld.DAGService, layout options.Layout, prefix cid.Prefix, r io.Reader) (ipld.Node, error) {
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

// Diff returns a set of changes that transform node 'a' into node 'b'.
// Modified from https://github.com/ipfs/go-merkledag/blob/master/dagutils/diff.go#L104
// It only traverses links in the following cases:
// 1. two node's links number are greater than 0.
// 2. both of two nodes are ProtoNode.
// 3. neither of the nodes is a file node, which contains only unnamed raw blocks
// Otherwise, it compares the cid and emits a Mod change object.
func Diff(ctx context.Context, ds ipld.DAGService, a, b ipld.Node) ([]*du.Change, error) {
	// Base case where both nodes are leaves, just compare
	// their CIDs.
	if len(a.Links()) == 0 && len(b.Links()) == 0 {
		return getChange(a, b)
	}

	var out []*du.Change
	cleanA, okA := a.Copy().(*md.ProtoNode)
	cleanB, okB := b.Copy().(*md.ProtoNode)
	if !okA || !okB {
		return getChange(a, b)
	}

	if isFileNode(cleanA) || isFileNode(cleanB) {
		return []*du.Change{
			{
				Type:   du.Mod,
				Before: a.Cid(),
				After:  b.Cid(),
			},
		}, nil
	}

	// strip out unchanged stuff
	for _, lnk := range a.Links() {
		l, _, err := b.ResolveLink([]string{lnk.Name})
		if err == nil {
			if l.Cid.Equals(lnk.Cid) {
				// no change... ignore it
			} else {
				anode, err := lnk.GetNode(ctx, ds)
				if err != nil {
					return nil, err
				}

				bnode, err := l.GetNode(ctx, ds)
				if err != nil {
					return nil, err
				}

				sub, err := Diff(ctx, ds, anode, bnode)
				if err != nil {
					return nil, err
				}

				for _, subc := range sub {
					subc.Path = path.Join(lnk.Name, subc.Path)
					out = append(out, subc)
				}
			}
			_ = cleanA.RemoveNodeLink(l.Name)
			_ = cleanB.RemoveNodeLink(l.Name)
		}
	}

	for _, lnk := range cleanA.Links() {
		out = append(out, &du.Change{
			Type:   du.Remove,
			Path:   lnk.Name,
			Before: lnk.Cid,
		})
	}
	for _, lnk := range cleanB.Links() {
		out = append(out, &du.Change{
			Type:  du.Add,
			Path:  lnk.Name,
			After: lnk.Cid,
		})
	}

	return out, nil
}

// getChange copied from https://github.com/ipfs/go-merkledag/blob/master/dagutils/diff.go#L203
func getChange(a, b ipld.Node) ([]*du.Change, error) {
	if a.Cid().Equals(b.Cid()) {
		return []*du.Change{}, nil
	}
	return []*du.Change{
		{
			Type:   du.Mod,
			Before: a.Cid(),
			After:  b.Cid(),
		},
	}, nil
}

func isFileNode(n ipld.Node) bool {
	for _, l := range n.Links() {
		if l.Name == "" {
			return true
		}
	}
	return false
}

func stashChanges(diff []Change) error {
	for _, c := range diff {
		switch c.Type {
		case du.Mod, du.Add:
			if err := os.Rename(c.Name, c.Name+patchExt); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyChanges(diff []Change) error {
	for _, c := range diff {
		switch c.Type {
		case du.Mod, du.Add:
			if err := os.Rename(c.Name+patchExt, c.Name); err != nil {
				return err
			}
		case du.Remove:
			// If the file was also deleted on the remote,
			// the local deletion will already have been handled by getPath.
			// So, we just ignore the error here.
			_ = os.RemoveAll(c.Name)
		}
	}
	return nil
}
