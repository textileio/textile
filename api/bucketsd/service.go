package bucketsd

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	gopath "path"
	"strings"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/ipfs/go-cid"
	ipfsfiles "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-unixfs"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/dcrypto"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	nutil "github.com/textileio/go-threads/net/util"
	pow "github.com/textileio/powergate/api/client"
	userPb "github.com/textileio/powergate/api/gen/powergate/user/v1"
	powUtil "github.com/textileio/powergate/util"
	pb "github.com/textileio/textile/v2/api/bucketsd/pb"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/buckets"
	"github.com/textileio/textile/v2/buckets/archive"
	"github.com/textileio/textile/v2/buckets/retrieval"
	"github.com/textileio/textile/v2/ipns"
	mdb "github.com/textileio/textile/v2/mongodb"
	tdb "github.com/textileio/textile/v2/threaddb"
	"github.com/textileio/textile/v2/util"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("bucketsapi")

	// ErrArchivingFeatureDisabled indicates an archive was requested with archiving disabled.
	ErrArchivingFeatureDisabled = errors.New("archiving feature is disabled")

	// ErrMaxBucketSizeExceeded indicates the requested operation exceeds the max bucket size.
	ErrMaxBucketSizeExceeded = errors.New("max bucket size exceeded")

	// ErrStorageQuotaExhausted indicates the requested operation exceeds the storage allowance.
	ErrStorageQuotaExhausted = errors.New("storage quota exhausted")

	// errInvalidNodeType indicates a node with type other than raw of proto was encountered.
	errInvalidNodeType = errors.New("invalid node type")

	// errDBRequired indicates the request requires a thread ID.
	errDBRequired = errors.New("db required")

	// baseArchiveStorageConfig is used to build the final StorageConfig after being
	// combined with information from the ArchiveConfig
	baseArchiveStorageConfig = &userPb.StorageConfig{
		Hot: &userPb.HotConfig{
			Enabled: false,
		},
		Cold: &userPb.ColdConfig{
			Enabled: true,
		},
	}

	// @todo: Export the default storage config from powergate so we can create this from it.
	defaultDefaultArchiveConfig = mdb.ArchiveConfig{
		RepFactor:       1,
		DealMinDuration: powUtil.MinDealDuration,
		FastRetrieval:   true,
		DealStartOffset: 72 * 60 * 60 / powUtil.EpochDurationSeconds, // 72hs
	}
)

const (
	// chunkSize for get file requests.
	chunkSize = 1024 * 32
	// pinNotRecursiveMsg is used to match an IPFS "recursively pinned already" error.
	pinNotRecursiveMsg = "'from' cid was not recursively pinned already"
)

type ctxKey string

// Service is a gRPC service for buckets.
type Service struct {
	Collections               *mdb.Collections
	Buckets                   *tdb.Buckets
	GatewayURL                string
	GatewayBucketsHost        string
	IPFSClient                iface.CoreAPI
	IPNSManager               *ipns.Manager
	PowergateClient           *pow.Client
	PowergateAdminToken       string
	ArchiveTracker            *archive.Tracker
	FilRetrieval              *retrieval.FilRetrieval
	Semaphores                *nutil.SemaphorePool
	MaxBucketSize             int64
	MaxBucketArchiveRepFactor int
}

var (
	_ nutil.SemaphoreKey = (*buckLock)(nil)
)

type buckLock string

func (l buckLock) Key() string {
	return string(l)
}

func (s *Service) List(ctx context.Context, _ *pb.ListRequest) (*pb.ListResponse, error) {
	log.Debugf("received list request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	list, err := s.Buckets.List(ctx, dbID, &db.Query{}, &tdb.Bucket{}, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	bucks := list.([]*tdb.Bucket)
	roots := make([]*pb.Root, len(bucks))
	for i, buck := range bucks {
		roots[i], err = getPbRoot(dbID, buck)
		if err != nil {
			return nil, err
		}
	}
	return &pb.ListResponse{Roots: roots}, nil
}

func getPbRoot(dbID thread.ID, buck *tdb.Bucket) (*pb.Root, error) {
	var pmd *pb.Metadata
	md, ok := buck.Metadata[""]
	if ok {
		var err error
		pmd, err = metadataToPb(md)
		if err != nil {
			return nil, err
		}
	}
	return &pb.Root{
		Key:       buck.Key,
		Owner:     buck.Owner,
		Name:      buck.Name,
		Version:   int32(buck.Version),
		Path:      buck.Path,
		Metadata:  pmd,
		Thread:    dbID.String(),
		CreatedAt: buck.CreatedAt,
		UpdatedAt: buck.UpdatedAt,
	}, nil
}

func metadataToPb(md tdb.Metadata) (*pb.Metadata, error) {
	roles := make(map[string]pb.PathAccessRole)
	for k, r := range md.Roles {
		var pr pb.PathAccessRole
		switch r {
		case buckets.None:
			pr = pb.PathAccessRole_PATH_ACCESS_ROLE_UNSPECIFIED
		case buckets.Reader:
			pr = pb.PathAccessRole_PATH_ACCESS_ROLE_READER
		case buckets.Writer:
			pr = pb.PathAccessRole_PATH_ACCESS_ROLE_WRITER
		case buckets.Admin:
			pr = pb.PathAccessRole_PATH_ACCESS_ROLE_ADMIN
		default:
			return nil, fmt.Errorf("unknown path access role %d", r)
		}
		roles[k] = pr
	}
	return &pb.Metadata{
		Roles:     roles,
		UpdatedAt: md.UpdatedAt,
	}, nil
}

func (s *Service) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	log.Debugf("received create request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	var bootCid cid.Cid
	if req.BootstrapCid != "" {
		var err error
		bootCid, err = cid.Decode(req.BootstrapCid)
		if err != nil {
			return nil, fmt.Errorf("invalid bootstrap cid: %s", err)
		}
	}

	// If the bucket is created from some imported archive,
	// create the retrieval request, and let all the process
	// happend async in the background.
	if req.Unfreeze {
		account, _ := mdb.AccountFromContext(ctx)
		owner := account.Owner()
		accKey := owner.Key.String()
		powToken := owner.PowInfo.Token

		if err := s.FilRetrieval.CreateForNewBucket(ctx, accKey, dbID, dbToken, req.Name, req.Private, bootCid, powToken); err != nil {
			return nil, fmt.Errorf("creating retrieval: %s", err)
		}
		return &pb.CreateResponse{}, nil
	}

	// If not created with --unfreeze, just do the normal case.
	ctx, buck, seed, err := s.createBucket(ctx, dbID, dbToken, req.Name, req.Private, bootCid)
	if err != nil {
		return nil, err
	}
	var seedData []byte
	if buck.IsPrivate() {
		fileKey, err := buck.GetFileEncryptionKeyForPath("")
		if err != nil {
			return nil, err
		}
		seedData, err = decryptData(seed.RawData(), fileKey)
		if err != nil {
			return nil, err
		}
	} else {
		seedData = seed.RawData()
	}

	root, err := getPbRoot(dbID, buck)
	if err != nil {
		return nil, err
	}
	links, err := s.createLinks(ctx, dbID, buck, "", dbToken)
	if err != nil {
		return nil, err
	}
	return &pb.CreateResponse{
		Root:    root,
		Links:   links,
		Seed:    seedData,
		SeedCid: seed.Cid().String(),
		Pinned:  s.getPinnedBytes(ctx),
	}, nil
}

// CreateBucket is a wrapper to enable creating a bucket from an internal method.
// TODO: We should consider refactor createBucket code (and other related stuff) out of
// `Service`, since this logic is useful for other cases than serving APIs.
func (s *Service) CreateBucket(ctx context.Context,
	threadID thread.ID,
	threadToken thread.Token,
	buckName string,
	buckPrivate bool,
	dataCid cid.Cid) error {
	_, _, _, err := s.createBucket(ctx, threadID, threadToken, buckName, buckPrivate, dataCid)
	return err
}

// createBucket returns a new bucket and seed node.
func (s *Service) createBucket(
	ctx context.Context,
	dbID thread.ID,
	dbToken thread.Token,
	name string,
	private bool,
	bootCid cid.Cid,
) (nctx context.Context, buck *tdb.Bucket, seed ipld.Node, err error) {
	var owner thread.PubKey
	if dbToken.Defined() {
		owner, err = dbToken.PubKey()
		if err != nil {
			return ctx, nil, nil, fmt.Errorf("creating bucket: invalid token public key")
		}
	}

	// Create bucket keys if private
	var linkKey, fileKey []byte
	if private {
		var err error
		linkKey, err = dcrypto.NewKey()
		if err != nil {
			return ctx, nil, nil, err
		}
		fileKey, err = dcrypto.NewKey()
		if err != nil {
			return ctx, nil, nil, err
		}
	}

	// Make a random seed, which ensures a bucket's uniqueness
	seed, err = makeSeed(fileKey)
	if err != nil {
		return
	}

	// Create the bucket directory
	var buckPath path.Resolved
	if bootCid.Defined() {
		ctx, buckPath, err = s.createBootstrappedPath(ctx, "", seed, bootCid, linkKey, fileKey)
		if err != nil {
			return ctx, nil, nil, fmt.Errorf("creating prepared bucket: %s", err)
		}
	} else {
		ctx, buckPath, err = s.createPristinePath(ctx, seed, linkKey)
		if err != nil {
			return ctx, nil, nil, fmt.Errorf("creating pristine bucket: %s", err)
		}
	}

	// Create top-level metadata
	now := time.Now()
	md := map[string]tdb.Metadata{
		"": tdb.NewDefaultMetadata(owner, fileKey, now),
		buckets.SeedName: {
			Roles:     make(map[string]buckets.Role),
			UpdatedAt: now.UnixNano(),
		},
	}

	// Create a new IPNS key
	buckKey, err := s.IPNSManager.CreateKey(ctx, dbID)
	if err != nil {
		return
	}

	// Create the bucket using the IPNS key as instance ID
	buck, err = s.Buckets.New(
		ctx,
		dbID,
		buckKey,
		buckPath,
		now,
		owner,
		md,
		tdb.WithNewBucketName(name),
		tdb.WithNewBucketKey(linkKey),
		tdb.WithNewBucketToken(dbToken))
	if err != nil {
		return
	}

	// Finally, publish the new bucket's address to the name system
	go s.IPNSManager.Publish(buckPath, buck.Key)
	return ctx, buck, seed, nil
}

// makeSeed returns a raw ipld node containing a random seed.
func makeSeed(key []byte) (ipld.Node, error) {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	// Encrypt seed if key is set (treating the seed differently here would complicate bucket reading)
	if key != nil {
		var err error
		seed, err = encryptData(seed, nil, key)
		if err != nil {
			return nil, err
		}
	}
	return dag.NewRawNode(seed), nil
}

// encryptData encrypts data with the new key, decrypting with current key if needed.
func encryptData(data, currentKey, newKey []byte) ([]byte, error) {
	if currentKey != nil {
		var err error
		data, err = decryptData(data, currentKey)
		if err != nil {
			return nil, err
		}
	}
	r, err := dcrypto.NewEncrypter(bytes.NewReader(data), newKey)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(r)
}

// createPristinePath creates an IPFS path which only contains the seed file.
// The returned path will be pinned.
func (s *Service) createPristinePath(
	ctx context.Context,
	seed ipld.Node,
	key []byte,
) (context.Context, path.Resolved, error) {
	// Create the initial bucket directory
	n, err := newDirWithNode(seed, buckets.SeedName, key)
	if err != nil {
		return ctx, nil, err
	}
	if err = s.IPFSClient.Dag().AddMany(ctx, []ipld.Node{n, seed}); err != nil {
		return ctx, nil, err
	}
	pins := []ipld.Node{n}
	if key != nil {
		pins = append(pins, seed)
	}
	ctx, err = s.pinBlocks(ctx, pins)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, path.IpfsPath(n.Cid()), nil
}

// pinBlocks pins blocks, accounting for sum bytes pinned for context.
func (s *Service) pinBlocks(ctx context.Context, nodes []ipld.Node) (context.Context, error) {
	var totalAddedSize int64
	for _, n := range nodes {
		s, err := n.Stat()
		if err != nil {
			return ctx, fmt.Errorf("getting size of node: %s", err)
		}
		totalAddedSize += int64(s.CumulativeSize)
	}

	// Check context owner's storage allowance
	owner, ok := buckets.BucketOwnerFromContext(ctx)
	if ok && totalAddedSize > owner.StorageAvailable {
		return ctx, ErrStorageQuotaExhausted
	}

	if err := s.IPFSClient.Dag().Pinning().AddMany(ctx, nodes); err != nil {
		return ctx, fmt.Errorf("pinning set of nodes: %s", err)
	}
	return s.addPinnedBytes(ctx, totalAddedSize), nil
}

// addPinnedBytes adds the provided delta to a running total for context.
func (s *Service) addPinnedBytes(ctx context.Context, delta int64) context.Context {
	total, _ := ctx.Value(ctxKey("pinnedBytes")).(int64)
	ctx = context.WithValue(ctx, ctxKey("pinnedBytes"), total+delta)
	owner, ok := buckets.BucketOwnerFromContext(ctx)
	if ok {
		owner.StorageUsed += delta
		owner.StorageAvailable -= delta
		owner.StorageDelta += delta
		ctx = buckets.NewBucketOwnerContext(ctx, owner)
	}
	return ctx
}

// getPinnedBytes returns the total pinned bytes for context.
func (s *Service) getPinnedBytes(ctx context.Context) int64 {
	pinned, _ := ctx.Value(ctxKey("pinnedBytes")).(int64)
	return pinned
}

// createBootstrapedPath creates an IPFS path which is the bootCid UnixFS DAG,
// with tdb.SeedName seed file added to the root of the DAG. The returned path will
// be pinned.
func (s *Service) createBootstrappedPath(
	ctx context.Context,
	destPath string,
	seed ipld.Node,
	bootCid cid.Cid,
	linkKey,
	fileKey []byte,
) (context.Context, path.Resolved, error) {
	pth := path.IpfsPath(bootCid)
	bootStatn, err := s.IPFSClient.Object().Stat(ctx, pth)
	if err != nil {
		return ctx, nil, fmt.Errorf("resolving boot cid node: %s", err)
	}

	// Check context owner's storage allowance
	owner, ok := buckets.BucketOwnerFromContext(ctx)
	if ok && int64(bootStatn.CumulativeSize) > owner.StorageAvailable {
		return ctx, nil, ErrStorageQuotaExhausted
	}

	// Here we have to walk and possibly encrypt the boot path dag
	n, nodes, err := s.newDirFromExistingPath(ctx, pth, destPath, linkKey, fileKey, seed, buckets.SeedName)
	if err != nil {
		return ctx, nil, err
	}
	if err = s.IPFSClient.Dag().AddMany(ctx, nodes); err != nil {
		return ctx, nil, err
	}
	var pins []ipld.Node
	if linkKey != nil {
		pins = nodes
	} else {
		pins = []ipld.Node{n}
	}
	ctx, err = s.pinBlocks(ctx, pins)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, path.IpfsPath(n.Cid()), nil
}

// newDirWithNode returns a new proto node directory wrapping the node,
// which is encrypted if key is not nil.
func newDirWithNode(n ipld.Node, name string, key []byte) (ipld.Node, error) {
	dir := unixfs.EmptyDirNode()
	dir.SetCidBuilder(dag.V1CidPrefix())
	if err := dir.AddNodeLink(name, n); err != nil {
		return nil, err
	}
	return encryptNode(dir, key)
}

// encryptNode returns the encrypted version of node if key is not nil.
func encryptNode(n *dag.ProtoNode, key []byte) (*dag.ProtoNode, error) {
	if key == nil {
		return n, nil
	}
	cipher, err := encryptData(n.RawData(), nil, key)
	if err != nil {
		return nil, err
	}
	en := dag.NodeWithData(unixfs.FilePBData(cipher, uint64(len(cipher))))
	en.SetCidBuilder(dag.V1CidPrefix())
	return en, nil
}

// newDirFromExistingPath returns a new dir based on path.
// If keys are not nil, this method recursively walks the path, encrypting files and directories.
// If add is not nil, it will be included in the resulting (possibly encrypted) node under a link named addName.
// This method returns the root node and a list of all new nodes (which also includes the root).
func (s *Service) newDirFromExistingPath(
	ctx context.Context,
	pth path.Path,
	destPath string,
	linkKey,
	fileKey []byte,
	add ipld.Node,
	addName string,
) (ipld.Node, []ipld.Node, error) {
	rn, err := s.IPFSClient.ResolveNode(ctx, pth)
	if err != nil {
		return nil, nil, err
	}
	top, ok := rn.(*dag.ProtoNode)
	if !ok {
		return nil, nil, dag.ErrNotProtobuf
	}
	if linkKey == nil && fileKey == nil {
		nodes := []ipld.Node{top}
		if add != nil {
			if err := top.AddNodeLink(addName, add); err != nil {
				return nil, nil, err
			}
			nodes = append(nodes, add)
		}
		return top, nodes, nil
	} else if linkKey == nil || fileKey == nil {
		return nil, nil, fmt.Errorf("invalid link or file key")
	}

	// Walk the node, encrypting the leaves and directories
	var addNode *namedNode
	if add != nil {
		addNode = &namedNode{
			name: addName,
			node: add,
		}
	}
	nmap, err := s.encryptDag(
		ctx,
		s.IPFSClient.Dag(),
		top,
		destPath,
		linkKey,
		nil,
		nil,
		fileKey,
		addNode,
	)
	if err != nil {
		return nil, nil, err
	}

	// Collect new nodes
	nodes := make([]ipld.Node, len(nmap))
	i := 0
	for _, tn := range nmap {
		nodes[i] = tn.node
		i++
	}
	return nmap[top.Cid()].node, nodes, nil
}

type namedNode struct {
	name string
	path string
	node ipld.Node
	cid  cid.Cid
}

type namedNodes struct {
	sync.RWMutex
	m map[cid.Cid]*namedNode
}

func newNamedNodes() *namedNodes {
	return &namedNodes{
		m: make(map[cid.Cid]*namedNode),
	}
}

func (nn *namedNodes) Get(c cid.Cid) *namedNode {
	nn.RLock()
	defer nn.RUnlock()
	return nn.m[c]
}

func (nn *namedNodes) Store(c cid.Cid, n *namedNode) {
	nn.Lock()
	defer nn.Unlock()
	nn.m[c] = n
}

// encryptDag creates an encrypted version of root that includes all child nodes.
// Leaf nodes are encrypted and linked to parents, which are then encrypted and
// linked to their parents, and so on up to root.
// add will be added to the encrypted root node if not nil.
// This method returns a map of all nodes keyed by their _original_ plaintext cid,
// and a list of the root's direct links.
func (s *Service) encryptDag(
	ctx context.Context,
	ds ipld.DAGService,
	root ipld.Node,
	destPath string,
	linkKey []byte,
	currentFileKeys,
	newFileKeys map[string][]byte,
	newFileKey []byte,
	add *namedNode,
) (map[cid.Cid]*namedNode, error) {
	// Step 1: Create a preordered list of joint and leaf nodes
	var stack, joints []*namedNode
	jmap := make(map[cid.Cid]*namedNode)
	lmap := make(map[cid.Cid]*namedNode)
	stack = append(stack, &namedNode{node: root, path: destPath})
	var cur *namedNode

	for len(stack) > 0 {
		n := len(stack) - 1
		cur = stack[n]
		stack = stack[:n]

		if _, ok := jmap[cur.node.Cid()]; ok {
			continue
		}
		if _, ok := lmap[cur.node.Cid()]; ok {
			continue
		}

	types:
		switch cur.node.(type) {
		case *dag.RawNode:
			lmap[cur.node.Cid()] = cur
		case *dag.ProtoNode:
			// Add links to the stack
			cur.cid = cur.node.Cid()
			if currentFileKeys != nil {
				var err error
				cur.node, _, err = decryptNode(cur.node, linkKey)
				if err != nil {
					return nil, err
				}
			}
			for _, l := range cur.node.Links() {
				if l.Name == "" {
					// We have discovered a raw file node wrapper
					// Use the original cur node because file node wrappers aren't encrypted
					lmap[cur.cid] = cur
					break types
				}
				ln, err := l.GetNode(ctx, ds)
				if err != nil {
					return nil, err
				}
				stack = append(stack, &namedNode{
					name: l.Name,
					path: gopath.Join(cur.path, l.Name),
					node: ln,
				})
			}
			joints = append(joints, cur)
			jmap[cur.cid] = cur
		default:
			return nil, errInvalidNodeType
		}
	}

	// Step 2: Encrypt all leaf nodes in parallel
	nmap := newNamedNodes()
	eg, gctx := errgroup.WithContext(ctx)
	for _, l := range lmap {
		l := l
		cfk := getFileKey(nil, currentFileKeys, l.path)
		nfk := getFileKey(newFileKey, newFileKeys, l.path)
		if nfk == nil {
			// This shouldn't happen
			return nil, fmt.Errorf("new file key not found for path %s", l.path)
		}
		eg.Go(func() error {
			if gctx.Err() != nil {
				return nil
			}
			var cn ipld.Node
			switch l.node.(type) {
			case *dag.RawNode:
				data, err := encryptData(l.node.RawData(), cfk, nfk)
				if err != nil {
					return err
				}
				cn = dag.NewRawNode(data)
			case *dag.ProtoNode:
				var err error
				cn, err = s.encryptFileNode(gctx, l.node, cfk, nfk)
				if err != nil {
					return err
				}
			}
			nmap.Store(l.node.Cid(), &namedNode{
				name: l.name,
				node: cn,
			})
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// Step 3: Encrypt joint nodes in reverse, walking up to root
	// Note: In the case where we're re-encrypting a dag, joints will already be decrypted.
	for i := len(joints) - 1; i >= 0; i-- {
		j := joints[i]
		jn := j.node.(*dag.ProtoNode)
		dir := unixfs.EmptyDirNode()
		dir.SetCidBuilder(dag.V1CidPrefix())
		for _, l := range jn.Links() {
			ln := nmap.Get(l.Cid)
			if ln == nil {
				return nil, fmt.Errorf("link node not found")
			}
			if err := dir.AddNodeLink(ln.name, ln.node); err != nil {
				return nil, err
			}
		}
		if i == 0 && add != nil {
			if err := dir.AddNodeLink(add.name, add.node); err != nil {
				return nil, err
			}
			nmap.Store(add.node.Cid(), add)
		}
		cn, err := encryptNode(dir, linkKey)
		if err != nil {
			return nil, err
		}
		nmap.Store(j.cid, &namedNode{
			name: j.name,
			node: cn,
		})
	}
	return nmap.m, nil
}

func getFileKey(key []byte, pathKeys map[string][]byte, pth string) []byte {
	if pathKeys == nil {
		return key
	}
	k, ok := pathKeys[pth]
	if ok {
		return k
	}
	return key
}

// encryptFileNode encrypts node with the new key, decrypting with current key if needed.
func (s *Service) encryptFileNode(ctx context.Context, n ipld.Node, currentKey, newKey []byte) (ipld.Node, error) {
	fn, err := s.IPFSClient.Unixfs().Get(ctx, path.IpfsPath(n.Cid()))
	if err != nil {
		return nil, err
	}
	defer fn.Close()
	file := ipfsfiles.ToFile(fn)
	if file == nil {
		return nil, fmt.Errorf("node is a directory")
	}
	var r1 io.Reader
	if currentKey != nil {
		r1, err = dcrypto.NewDecrypter(file, currentKey)
		if err != nil {
			return nil, err
		}
	} else {
		r1 = file
	}
	r2, err := dcrypto.NewEncrypter(r1, newKey)
	if err != nil {
		return nil, err
	}
	pth, err := s.IPFSClient.Unixfs().Add(
		ctx,
		ipfsfiles.NewReaderFile(r2),
		options.Unixfs.CidVersion(1),
		options.Unixfs.Pin(false),
	)
	if err != nil {
		return nil, err
	}
	return s.IPFSClient.ResolveNode(ctx, pth)
}

func (s *Service) Root(ctx context.Context, req *pb.RootRequest) (*pb.RootResponse, error) {
	log.Debugf("received root request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	buck := &tdb.Bucket{}
	err := s.Buckets.GetSafe(ctx, dbID, req.Key, buck, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	root, err := getPbRoot(dbID, buck)
	if err != nil {
		return nil, err
	}
	return &pb.RootResponse{
		Root: root,
	}, nil
}

func (s *Service) Links(ctx context.Context, req *pb.LinksRequest) (*pb.LinksResponse, error) {
	log.Debugf("received lists request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	buck := &tdb.Bucket{}
	err := s.Buckets.GetSafe(ctx, dbID, req.Key, buck, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	return s.createLinks(ctx, dbID, buck, cleanPath(req.Path), dbToken)
}

func (s *Service) createLinks(
	ctx context.Context,
	dbID thread.ID,
	buck *tdb.Bucket,
	pth string,
	dbToken thread.Token,
) (*pb.LinksResponse, error) {
	var threadLink, wwwLink, ipnsLink string
	threadLink = fmt.Sprintf("%s/thread/%s/%s/%s", s.GatewayURL, dbID, buckets.CollectionName, buck.Key)
	if s.GatewayBucketsHost != "" {
		parts := strings.Split(s.GatewayURL, "://")
		if len(parts) < 2 {
			return nil, fmt.Errorf("failed to parse gateway URL: %s", s.GatewayURL)
		}
		wwwLink = fmt.Sprintf("%s://%s.%s", parts[0], buck.Key, s.GatewayBucketsHost)
	}
	ipnsLink = fmt.Sprintf("%s/ipns/%s", s.GatewayURL, buck.Key)

	if _, _, ok := buck.GetMetadataForPath(pth, false); !ok {
		return nil, fmt.Errorf("could not resolve path: %s", pth)
	}
	if pth != "" {
		npth, err := inflateFilePath(buck, pth)
		if err != nil {
			return nil, err
		}
		linkKey := buck.GetLinkEncryptionKey()
		if _, err := s.getNodeAtPath(ctx, npth, linkKey); err != nil {
			return nil, err
		}
		pth = "/" + pth
		threadLink += pth
		if wwwLink != "" {
			wwwLink += pth
		}
		ipnsLink += pth
	}
	if buck.IsPrivate() {
		query := "?token=" + string(dbToken)
		threadLink += query
		if wwwLink != "" {
			wwwLink += query
		}
		ipnsLink += query
	}
	return &pb.LinksResponse{
		Url:  threadLink,
		Www:  wwwLink,
		Ipns: ipnsLink,
	}, nil
}

func (s *Service) SetPath(ctx context.Context, req *pb.SetPathRequest) (res *pb.SetPathResponse, err error) {
	log.Debugf("received set path request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	destPath := cleanPath(req.Path)
	bootCid, err := cid.Decode(req.Cid)
	if err != nil {
		return nil, fmt.Errorf("invalid remote cid: %s", err)
	}

	lck := s.Semaphores.Get(buckLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	buck := &tdb.Bucket{}
	if err = s.Buckets.GetSafe(ctx, dbID, req.Key, buck, tdb.WithToken(dbToken)); err != nil {
		return nil, fmt.Errorf("get bucket: %s", err)
	}

	buck.UpdatedAt = time.Now().UnixNano()
	buck.SetMetadataAtPath(destPath, tdb.Metadata{
		UpdatedAt: buck.UpdatedAt,
	})
	buck.UnsetMetadataWithPrefix(destPath + "/")

	if err = s.Buckets.Verify(ctx, dbID, buck, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}

	var linkKey, fileKey []byte
	if buck.IsPrivate() {
		linkKey = buck.GetLinkEncryptionKey()
		fileKey, err = buck.GetFileEncryptionKeyForPath(destPath)
		if err != nil {
			return nil, err
		}
	}

	buckPath := path.New(buck.Path)
	ctx, dirPath, err := s.setPathFromExistingCid(ctx, buck, buckPath, destPath, bootCid, linkKey, fileKey)
	if err != nil {
		return nil, err
	}
	buck.Path = dirPath.String()
	if err = s.Buckets.Save(ctx, dbID, buck, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.SetPathResponse{
		Pinned: s.getPinnedBytes(ctx),
	}, nil
}

// setPathFromExistingCid sets the path with a cid from the network, encrypting with file key if present.
func (s *Service) setPathFromExistingCid(
	ctx context.Context,
	buck *tdb.Bucket,
	buckPath path.Path,
	destPath string,
	bootCid cid.Cid,
	linkKey,
	fileKey []byte,
) (context.Context, path.Resolved, error) {
	var dirPath path.Resolved
	if destPath == "" {
		sn, err := makeSeed(fileKey)
		if err != nil {
			return ctx, nil, fmt.Errorf("generating new seed: %s", err)
		}
		ctx, dirPath, err = s.createBootstrappedPath(ctx, destPath, sn, bootCid, linkKey, fileKey)
		if err != nil {
			return ctx, nil, fmt.Errorf("generating bucket new root: %s", err)
		}
		if buck.IsPrivate() {
			buckPathResolved, err := s.IPFSClient.ResolvePath(ctx, buckPath)
			if err != nil {
				return ctx, nil, fmt.Errorf("resolving path: %s", err)
			}
			ctx, err = s.unpinNodeAndBranch(ctx, buckPathResolved, linkKey)
			if err != nil {
				return ctx, nil, fmt.Errorf("unpinning pinned root: %s", err)
			}
		} else {
			ctx, err = s.unpinPath(ctx, buckPath)
			if err != nil {
				return ctx, nil, fmt.Errorf("updating pinned root: %s", err)
			}
		}
	} else {
		bootPath := path.IpfsPath(bootCid)
		if buck.IsPrivate() {
			n, nodes, err := s.newDirFromExistingPath(ctx, bootPath, destPath, linkKey, fileKey, nil, "")
			if err != nil {
				return ctx, nil, fmt.Errorf("resolving remote path: %s", err)
			}
			ctx, dirPath, err = s.insertNodeAtPath(ctx, n, path.Join(buckPath, destPath), linkKey)
			if err != nil {
				return ctx, nil, fmt.Errorf("updating pinned root: %s", err)
			}
			ctx, err = s.addAndPinNodes(ctx, nodes)
			if err != nil {
				return ctx, nil, err
			}
		} else {
			var err error
			dirPath, err = s.IPFSClient.Object().AddLink(
				ctx,
				buckPath,
				destPath,
				bootPath,
				options.Object.Create(true),
			)
			if err != nil {
				return ctx, nil, fmt.Errorf("adding folder: %s", err)
			}
			ctx, err = s.updateOrAddPin(ctx, buckPath, dirPath)
			if err != nil {
				return ctx, nil, fmt.Errorf("updating pinned root: %s", err)
			}
		}
	}
	return ctx, dirPath, nil
}

// unpinPath unpins path and accounts for sum bytes pinned for context.
func (s *Service) unpinPath(ctx context.Context, path path.Path) (context.Context, error) {
	if err := s.IPFSClient.Pin().Rm(ctx, path); err != nil {
		return ctx, err
	}
	stat, err := s.IPFSClient.Object().Stat(ctx, path)
	if err != nil {
		return ctx, fmt.Errorf("getting size of removed node: %s", err)
	}
	return s.addPinnedBytes(ctx, int64(-stat.CumulativeSize)), nil
}

// addAndPinNodes adds and pins nodes, accounting for sum bytes pinned for context.
func (s *Service) addAndPinNodes(ctx context.Context, nodes []ipld.Node) (context.Context, error) {
	if err := s.IPFSClient.Dag().AddMany(ctx, nodes); err != nil {
		return ctx, err
	}
	return s.pinBlocks(ctx, nodes)
}

func (s *Service) ListPath(ctx context.Context, req *pb.ListPathRequest) (*pb.ListPathResponse, error) {
	log.Debugf("received list path request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	reqPath := cleanPath(req.Path)
	buck, pth, err := s.getBucketPath(ctx, dbID, req.Key, reqPath, dbToken)
	if err != nil {
		return nil, err
	}

	rep, err := s.pathToPb(ctx, dbID, buck, pth, true)
	if err != nil {
		return nil, err
	}
	if pth.String() == buck.Path {
		rep.Item.Name = buck.Name
	}
	return rep, nil
}

func (s *Service) ListIpfsPath(ctx context.Context, req *pb.ListIpfsPathRequest) (*pb.ListIpfsPathResponse, error) {
	log.Debugf("received list ipfs path request")

	pth := path.New(req.Path)
	item, err := s.pathToItem(ctx, nil, pth, true)
	if err != nil {
		return nil, err
	}
	return &pb.ListIpfsPathResponse{Item: item}, nil
}

// pathToItem returns items at path, optionally including one level down of links.
// If key is not nil, the items will be decrypted.
func (s *Service) pathToItem(
	ctx context.Context,
	buck *tdb.Bucket,
	pth path.Path,
	includeNextLevel bool,
) (*pb.PathItem, error) {
	var linkKey []byte
	if buck != nil {
		linkKey = buck.GetLinkEncryptionKey()
	}
	n, err := s.getNodeAtPath(ctx, pth, linkKey)
	if err != nil {
		return nil, err
	}
	return s.nodeToItem(ctx, buck, n, pth.String(), linkKey, false, includeNextLevel)
}

// getNodeAtPath returns the node at path by traversing and potentially decrypting parent nodes.
func (s *Service) getNodeAtPath(ctx context.Context, pth path.Path, key []byte) (ipld.Node, error) {
	if key != nil {
		rp, fp, err := util.ParsePath(pth)
		if err != nil {
			return nil, err
		}
		np, _, r, err := s.getNodesToPath(ctx, rp, fp, key)
		if err != nil {
			return nil, err
		}
		if r != "" {
			return nil, fmt.Errorf("could not resolve path: %s", pth)
		}
		return np[len(np)-1].new, nil
	} else {
		rp, err := s.IPFSClient.ResolvePath(ctx, pth)
		if err != nil {
			return nil, err
		}
		return s.IPFSClient.Dag().Get(ctx, rp.Cid())
	}
}

// resolveNodeAtPath returns the decrypted node at path and whether or not it is a directory.
func (s *Service) resolveNodeAtPath(ctx context.Context, pth path.Resolved, key []byte) (ipld.Node, bool, error) {
	cn, err := s.IPFSClient.ResolveNode(ctx, pth)
	if err != nil {
		return nil, false, err
	}
	return decryptNode(cn, key)
}

// decryptNode returns a decrypted version of node and whether or not it is a directory.
func decryptNode(cn ipld.Node, key []byte) (ipld.Node, bool, error) {
	switch cn := cn.(type) {
	case *dag.RawNode:
		return cn, false, nil // All raw nodes will be leaves
	case *dag.ProtoNode:
		if key == nil {
			return cn, false, nil // Could be a joint, but it's not important to know in the public case
		}
		fn, err := unixfs.FSNodeFromBytes(cn.Data())
		if err != nil {
			return nil, false, err
		}
		if fn.Data() == nil {
			return cn, false, nil // This node is a raw file wrapper
		}
		plain, err := decryptData(fn.Data(), key)
		if err != nil {
			return nil, false, err
		}
		n, err := dag.DecodeProtobuf(plain)
		if err != nil {
			return dag.NewRawNode(plain), false, nil
		}
		n.SetCidBuilder(dag.V1CidPrefix())
		return n, true, nil
	default:
		return nil, false, errInvalidNodeType
	}
}

// decryptData decrypts data with key.
func decryptData(data, key []byte) ([]byte, error) {
	r, err := dcrypto.NewDecrypter(bytes.NewReader(data), key)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

// nodeToItem returns a path item from an IPLD node.
func (s *Service) nodeToItem(
	ctx context.Context,
	buck *tdb.Bucket,
	node ipld.Node,
	pth string,
	key []byte,
	decrypt,
	includeNextLevel bool,
) (*pb.PathItem, error) {
	if decrypt && key != nil {
		var err error
		node, _, err = decryptNode(node, key)
		if err != nil {
			return nil, err
		}
	}
	var pmd *pb.Metadata
	if buck != nil {
		name := cleanPath(strings.TrimPrefix(pth, buck.Path))
		md, _, ok := buck.GetMetadataForPath(name, false)
		if !ok {
			return nil, fmt.Errorf("could not resolve path: %s", pth)
		}
		var err error
		pmd, err = metadataToPb(md)
		if err != nil {
			return nil, err
		}
	}
	stat, err := node.Stat()
	if err != nil {
		return nil, err
	}
	item := &pb.PathItem{
		Cid:      node.Cid().String(),
		Name:     gopath.Base(pth),
		Path:     pth,
		Size:     int64(stat.CumulativeSize),
		Metadata: pmd,
	}
	if pn, ok := node.(*dag.ProtoNode); ok {
		fn, _ := unixfs.FSNodeFromBytes(pn.Data())
		if fn != nil && fn.IsDir() {
			item.IsDir = true
		}
	}
	if item.IsDir {
		for _, l := range node.Links() {
			if l.Name == "" {
				break
			}
			if includeNextLevel {
				p := gopath.Join(pth, l.Name)
				n, err := l.GetNode(ctx, s.IPFSClient.Dag())
				if err != nil {
					return nil, err
				}
				i, err := s.nodeToItem(ctx, buck, n, p, key, true, false)
				if err != nil {
					return nil, err
				}
				item.Items = append(item.Items, i)
			}
			item.ItemsCount++
		}
	}
	return item, nil
}

func parsePath(pth string) (fpth string, err error) {
	if strings.Contains(pth, buckets.SeedName) {
		err = fmt.Errorf("paths containing %s are not allowed", buckets.SeedName)
		return
	}
	return cleanPath(pth), nil
}

func (s *Service) getBucketPath(
	ctx context.Context,
	dbID thread.ID,
	key, pth string,
	token thread.Token,
) (*tdb.Bucket, path.Path, error) {
	buck := &tdb.Bucket{}
	err := s.Buckets.GetSafe(ctx, dbID, key, buck, tdb.WithToken(token))
	if err != nil {
		return nil, nil, err
	}
	npth, err := inflateFilePath(buck, pth)
	return buck, npth, err
}

func cleanPath(pth string) string {
	return strings.TrimPrefix(pth, "/")
}

func inflateFilePath(buck *tdb.Bucket, filePath string) (path.Path, error) {
	npth := path.New(gopath.Join(buck.Path, filePath))
	if err := npth.IsValid(); err != nil {
		return nil, err
	}
	return npth, nil
}

func (s *Service) pathToPb(
	ctx context.Context,
	id thread.ID,
	buck *tdb.Bucket,
	pth path.Path,
	includeNextLevel bool,
) (*pb.ListPathResponse, error) {
	item, err := s.pathToItem(ctx, buck, pth, includeNextLevel)
	if err != nil {
		return nil, err
	}
	root, err := getPbRoot(id, buck)
	if err != nil {
		return nil, err
	}
	return &pb.ListPathResponse{
		Item: item,
		Root: root,
	}, nil
}

func (s *Service) PushPath(server pb.APIService_PushPathServer) (err error) {
	log.Debugf("received push path request")

	dbID, ok := common.ThreadIDFromContext(server.Context())
	if !ok {
		return errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(server.Context())

	storageAvailable := int64(math.MaxInt64)
	owner, ok := buckets.BucketOwnerFromContext(server.Context())
	if ok {
		storageAvailable = owner.StorageAvailable
	}

	req, err := server.Recv()
	if err != nil {
		return err
	}
	var buckKey, headerPath, root string
	switch payload := req.Payload.(type) {
	case *pb.PushPathRequest_Header_:
		buckKey = payload.Header.Key
		headerPath = payload.Header.Path
		root = payload.Header.Root
	default:
		return fmt.Errorf("push bucket path header is required")
	}
	filePath, err := parsePath(headerPath)
	if err != nil {
		return err
	}

	lck := s.Semaphores.Get(buckLock(buckKey))
	lck.Acquire()
	defer lck.Release()

	buck := &tdb.Bucket{}
	err = s.Buckets.GetSafe(server.Context(), dbID, buckKey, buck, tdb.WithToken(dbToken))
	if err != nil {
		return err
	}
	if root != "" && root != buck.Path {
		return status.Error(codes.FailedPrecondition, buckets.ErrNonFastForward.Error())
	}

	buck.UpdatedAt = time.Now().UnixNano()
	buck.SetMetadataAtPath(filePath, tdb.Metadata{
		UpdatedAt: buck.UpdatedAt,
	})
	buck.UnsetMetadataWithPrefix(filePath + "/")

	if err = s.Buckets.Verify(server.Context(), dbID, buck, tdb.WithToken(dbToken)); err != nil {
		return err
	}

	fileKey, err := buck.GetFileEncryptionKeyForPath(filePath)
	if err != nil {
		return err
	}

	buckPath := path.New(buck.Path)
	stat, err := s.IPFSClient.Object().Stat(server.Context(), buckPath)
	if err != nil {
		return fmt.Errorf("get stat of current bucket: %s", err)
	}
	reader, writer := io.Pipe()
	var writerErr error
	assignErr := func(err error) {
		writerErr = err
		_ = writer.CloseWithError(err)
	}
	go func() {
		cummSize := int64(stat.CumulativeSize)
		for {
			req, err := server.Recv()
			if err == io.EOF {
				_ = writer.Close()
				return
			} else if err != nil {
				assignErr(fmt.Errorf("error on receive: %v", err))
				return
			}
			switch payload := req.Payload.(type) {
			case *pb.PushPathRequest_Chunk:
				n, err := writer.Write(payload.Chunk)
				if err != nil {
					assignErr(fmt.Errorf("error writing chunk: %v", err))
					return
				}
				cummSize += int64(n)
				if s.MaxBucketSize > 0 && cummSize > s.MaxBucketSize {
					assignErr(ErrMaxBucketSizeExceeded)
					return
				} else if storageAvailable > 0 && cummSize > storageAvailable {
					assignErr(ErrStorageQuotaExhausted)
					return
				}
			default:
				assignErr(fmt.Errorf("invalid request"))
				return
			}
		}
	}()

	eventCh := make(chan interface{})
	defer close(eventCh)
	chSize := make(chan string)
	go func() {
		for e := range eventCh {
			event, ok := e.(*iface.AddEvent)
			if !ok {
				log.Error("unexpected event type")
				continue
			}
			if event.Path == nil { // This is a progress event
				if err := server.Send(&pb.PushPathResponse{
					Event: &pb.PushPathResponse_Event{
						Name:  event.Name,
						Bytes: event.Bytes,
					},
				}); err != nil {
					log.Errorf("error sending event: %v", err)
				}
			} else {
				chSize <- event.Size // Save size for use in the final response
			}
		}
	}()

	var r io.Reader
	if fileKey != nil {
		r, err = dcrypto.NewEncrypter(reader, fileKey)
		if err != nil {
			return err
		}
	} else {
		r = reader
	}
	newPath, err := s.IPFSClient.Unixfs().Add(
		server.Context(),
		ipfsfiles.NewReaderFile(r),
		options.Unixfs.CidVersion(1),
		options.Unixfs.Pin(false),
		options.Unixfs.Progress(true),
		options.Unixfs.Events(eventCh),
	)
	if writerErr != nil {
		return writerErr
	}
	if err != nil {
		return err
	}
	fn, err := s.IPFSClient.ResolveNode(server.Context(), newPath)
	if err != nil {
		return err
	}

	ctx := server.Context()
	var dirPath path.Resolved
	if buck.IsPrivate() {
		ctx, dirPath, err = s.insertNodeAtPath(ctx, fn, path.Join(buckPath, filePath), buck.GetLinkEncryptionKey())
		if err != nil {
			return err
		}
	} else {
		dirPath, err = s.IPFSClient.Object().AddLink(
			ctx,
			buckPath,
			filePath,
			newPath,
			options.Object.Create(true),
		)
		if err != nil {
			return err
		}
		ctx, err = s.updateOrAddPin(ctx, buckPath, dirPath)
		if err != nil {
			return err
		}
	}

	buck.Path = dirPath.String()
	if err = s.Buckets.Save(ctx, dbID, buck, tdb.WithToken(dbToken)); err != nil {
		return err
	}

	size := <-chSize
	pbroot, err := getPbRoot(dbID, buck)
	if err != nil {
		return err
	}
	if err = server.Send(&pb.PushPathResponse{
		Event: &pb.PushPathResponse_Event{
			Path:   newPath.String(),
			Size:   size,
			Root:   pbroot,
			Pinned: s.getPinnedBytes(ctx),
		},
	}); err != nil {
		return err
	}

	go s.IPNSManager.Publish(dirPath, buck.Key)

	log.Debugf("pushed %s to bucket: %s", filePath, buck.Key)
	return nil
}

// insertNodeAtPath inserts a node at the location of path.
// Key will be required if the path is encrypted.
func (s *Service) insertNodeAtPath(
	ctx context.Context,
	child ipld.Node,
	pth path.Path,
	key []byte,
) (context.Context, path.Resolved, error) {
	// The first step here is find a resolvable list of nodes that point to path.
	rp, fp, err := util.ParsePath(pth)
	if err != nil {
		return ctx, nil, err
	}
	fd, fn := gopath.Split(fp)
	fd = strings.TrimSuffix(fd, "/")
	np, _, r, err := s.getNodesToPath(ctx, rp, fd, key)
	if err != nil {
		return ctx, nil, err
	}
	r = gopath.Join(r, fn)

	// If the remaining path segment is not empty, we need to create each one
	// starting at the other end and walking back up to the deepest node
	// in the node path.
	parts := strings.Split(r, "/")
	news := make([]ipld.Node, len(parts)-1)
	cn := child
	for i := len(parts) - 1; i > 0; i-- {
		n, err := newDirWithNode(cn, parts[i], key)
		if err != nil {
			return ctx, nil, err
		}
		news[i-1] = cn
		cn = n
	}
	np = append(np, pathNode{new: cn, name: parts[0], isJoint: true})

	// Now, we have a full list of nodes to the insert location,
	// but the existing nodes need to be updated and re-encrypted.
	change := make([]ipld.Node, len(np))
	for i := len(np) - 1; i >= 0; i-- {
		change[i] = np[i].new
		if i > 0 {
			p, ok := np[i-1].new.(*dag.ProtoNode)
			if !ok {
				return ctx, nil, dag.ErrNotProtobuf
			}
			if np[i].isJoint {
				xn, err := p.GetLinkedNode(ctx, s.IPFSClient.Dag(), np[i].name)
				if err != nil && !errors.Is(err, dag.ErrLinkNotFound) {
					return ctx, nil, err
				}
				if xn != nil {
					np[i].old = path.IpfsPath(xn.Cid())
					if err := p.RemoveNodeLink(np[i].name); err != nil {
						return ctx, nil, err
					}
				}
			} else {
				xl, err := p.GetNodeLink(np[i].name)
				if err != nil && !errors.Is(err, dag.ErrLinkNotFound) {
					return ctx, nil, err
				}
				if xl != nil {
					if err := p.RemoveNodeLink(np[i].name); err != nil {
						return ctx, nil, err
					}
				}
			}
			if err := p.AddNodeLink(np[i].name, np[i].new); err != nil {
				return ctx, nil, err
			}
			np[i-1].new, err = encryptNode(p, key)
			if err != nil {
				return ctx, nil, err
			}
		}
	}

	// Add all the _new_ nodes, which is the sum of the brand new ones
	// from the missing path segment, and the changed ones from
	// the existing path.
	allNews := append(news, change...)
	if err := s.IPFSClient.Dag().AddMany(ctx, allNews); err != nil {
		return ctx, nil, err
	}
	// Pin brand new nodes
	ctx, err = s.pinBlocks(ctx, news)
	if err != nil {
		return ctx, nil, err
	}

	// Update changed node pins
	for _, n := range np {
		if n.old != nil && n.isJoint {
			ctx, err = s.unpinBranch(ctx, n.old, key)
			if err != nil {
				return ctx, nil, err
			}
		}
		ctx, err = s.updateOrAddPin(ctx, n.old, path.IpfsPath(n.new.Cid()))
		if err != nil {
			return ctx, nil, err
		}
	}
	return ctx, path.IpfsPath(np[0].new.Cid()), nil
}

// unpinBranch walks a the node at path, decrypting (if needed) and unpinning all nodes
func (s *Service) unpinBranch(ctx context.Context, p path.Resolved, key []byte) (context.Context, error) {
	n, _, err := s.resolveNodeAtPath(ctx, p, key)
	if err != nil {
		return ctx, err
	}
	for _, l := range n.Links() {
		if l.Name == "" {
			continue // Data nodes will never be pinned directly
		}
		lp := path.IpfsPath(l.Cid)
		ctx, err = s.unpinPath(ctx, lp)
		if err != nil {
			return ctx, err
		}
		ctx, err = s.unpinBranch(ctx, lp, key)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

type pathNode struct {
	old     path.Resolved
	new     ipld.Node
	name    string
	isJoint bool
}

// getNodesToPath returns a list of pathNodes that point to the path,
// The remaining path segment that was not resolvable is also returned.
func (s *Service) getNodesToPath(
	ctx context.Context,
	base path.Resolved,
	pth string,
	key []byte,
) (nodes []pathNode, isDir bool, remainder string, err error) {
	top, dir, err := s.resolveNodeAtPath(ctx, base, key)
	if err != nil {
		return
	}
	nodes = append(nodes, pathNode{old: base, new: top})
	remainder = pth
	if remainder == "" {
		return
	}
	parts := strings.Split(pth, "/")
	for i := 0; i < len(parts); i++ {
		l := getLink(top.Links(), parts[i])
		if l != nil {
			p := path.IpfsPath(l.Cid)
			top, dir, err = s.resolveNodeAtPath(ctx, p, key)
			if err != nil {
				return
			}
			nodes = append(nodes, pathNode{old: p, new: top, name: parts[i]})
		} else {
			remainder = strings.Join(parts[i:], "/")
			return nodes, dir, remainder, nil
		}
	}
	return nodes, dir, "", nil
}

func getLink(lnks []*ipld.Link, name string) *ipld.Link {
	for _, l := range lnks {
		if l.Name == name {
			return l
		}
	}
	return nil
}

// updateOrAddPin moves the pin at from to to.
// If from is nil, a new pin as placed at to.
func (s *Service) updateOrAddPin(ctx context.Context, from, to path.Path) (context.Context, error) {
	toSize, err := s.dagSize(ctx, to)
	if err != nil {
		return ctx, fmt.Errorf("getting size of destination dag: %s", err)
	}
	if s.MaxBucketSize > 0 && toSize > s.MaxBucketSize {
		return ctx, ErrMaxBucketSizeExceeded
	}

	fromSize, err := s.dagSize(ctx, from)
	if err != nil {
		return ctx, fmt.Errorf("getting size of current dag: %s", err)
	}
	deltaSize := -fromSize + toSize

	// Check context owner's storage allowance
	owner, ok := buckets.BucketOwnerFromContext(ctx)
	if ok && deltaSize > owner.StorageAvailable {
		return ctx, ErrStorageQuotaExhausted
	}

	if from == nil {
		if err := s.IPFSClient.Pin().Add(ctx, to); err != nil {
			return ctx, err
		}
	} else {
		if err := s.IPFSClient.Pin().Update(ctx, from, to); err != nil {
			if err.Error() == pinNotRecursiveMsg {
				return ctx, s.IPFSClient.Pin().Add(ctx, to)
			}
			return ctx, err
		}
	}
	return s.addPinnedBytes(ctx, deltaSize), nil
}

// dagSize returns the cummulative size of root. If root is nil, it returns 0.
func (s *Service) dagSize(ctx context.Context, root path.Path) (int64, error) {
	if root == nil {
		return 0, nil
	}
	stat, err := s.IPFSClient.Object().Stat(ctx, root)
	if err != nil {
		return 0, fmt.Errorf("get stats of pin destination: %s", err)
	}
	return int64(stat.CumulativeSize), nil
}

func (s *Service) PullPath(req *pb.PullPathRequest, server pb.APIService_PullPathServer) error {
	log.Debugf("received pull path request")

	dbID, ok := common.ThreadIDFromContext(server.Context())
	if !ok {
		return errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(server.Context())

	reqPath := cleanPath(req.Path)
	buck, pth, err := s.getBucketPath(server.Context(), dbID, req.Key, reqPath, dbToken)
	if err != nil {
		return err
	}
	fileKey, err := buck.GetFileEncryptionKeyForPath(reqPath)
	if err != nil {
		return err
	}

	var filePath path.Resolved
	if buck.IsPrivate() {
		buckPath, err := util.NewResolvedPath(buck.Path)
		if err != nil {
			return err
		}
		np, isDir, r, err := s.getNodesToPath(server.Context(), buckPath, reqPath, buck.GetLinkEncryptionKey())
		if err != nil {
			return err
		}
		if r != "" {
			return fmt.Errorf("could not resolve path: %s", pth)
		}
		if isDir {
			return fmt.Errorf("node is a directory")
		}
		fn := np[len(np)-1]
		filePath = path.IpfsPath(fn.new.Cid())
	} else {
		filePath, err = s.IPFSClient.ResolvePath(server.Context(), pth)
		if err != nil {
			return err
		}
	}
	node, err := s.IPFSClient.Unixfs().Get(server.Context(), filePath)
	if err != nil {
		return err
	}
	defer node.Close()

	file := ipfsfiles.ToFile(node)
	if file == nil {
		return fmt.Errorf("node is a directory")
	}
	var reader io.Reader
	if fileKey != nil {
		r, err := dcrypto.NewDecrypter(file, fileKey)
		if err != nil {
			return err
		}
		defer r.Close()
		reader = r
	} else {
		reader = file
	}

	buf := make([]byte, chunkSize)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if err := server.Send(&pb.PullPathResponse{
				Chunk: buf[:n],
			}); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) PullIpfsPath(req *pb.PullIpfsPathRequest, server pb.APIService_PullIpfsPathServer) error {
	log.Debugf("received ipfs pull path request")

	reqPath := path.New(req.Path)
	node, err := s.IPFSClient.Unixfs().Get(server.Context(), reqPath)
	if err != nil {
		return err
	}
	defer node.Close()

	file := ipfsfiles.ToFile(node)
	if file == nil {
		return fmt.Errorf("node is a directory")
	}
	buf := make([]byte, chunkSize)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			if err := server.Send(&pb.PullIpfsPathResponse{
				Chunk: buf[:n],
			}); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Remove(ctx context.Context, req *pb.RemoveRequest) (*pb.RemoveResponse, error) {
	log.Debugf("received remove request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	lck := s.Semaphores.Get(buckLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	buck := &tdb.Bucket{}
	err := s.Buckets.GetSafe(ctx, dbID, req.Key, buck, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}

	if err = s.Buckets.Delete(ctx, dbID, buck.Key, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}

	buckPath, err := util.NewResolvedPath(buck.Path)
	if err != nil {
		return nil, err
	}
	linkKey := buck.GetLinkEncryptionKey()
	if linkKey != nil {
		ctx, err = s.unpinNodeAndBranch(ctx, buckPath, linkKey)
		if err != nil {
			return nil, err
		}
	} else {
		ctx, err = s.unpinPath(ctx, buckPath)
		if err != nil {
			return nil, err
		}
	}
	if err = s.IPNSManager.RemoveKey(ctx, buck.Key); err != nil {
		return nil, err
	}

	log.Debugf("removed bucket: %s", buck.Key)
	return &pb.RemoveResponse{
		Pinned: s.getPinnedBytes(ctx),
	}, nil
}

// unpinNodeAndBranch unpins a node and its entire branch, accounting for sum bytes pinned for context.
func (s *Service) unpinNodeAndBranch(ctx context.Context, pth path.Resolved, key []byte) (context.Context, error) {
	ctx, err := s.unpinBranch(ctx, pth, key)
	if err != nil {
		return ctx, err
	}
	return s.unpinPath(ctx, pth)
}

func (s *Service) RemovePath(ctx context.Context, req *pb.RemovePathRequest) (res *pb.RemovePathResponse, err error) {
	log.Debugf("received remove path request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	filePath, err := parsePath(req.Path)
	if err != nil {
		return nil, err
	}

	lck := s.Semaphores.Get(buckLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	buck := &tdb.Bucket{}
	err = s.Buckets.GetSafe(ctx, dbID, req.Key, buck, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	if req.Root != "" && req.Root != buck.Path {
		return nil, status.Error(codes.FailedPrecondition, buckets.ErrNonFastForward.Error())
	}

	buck.UpdatedAt = time.Now().UnixNano()
	buck.UnsetMetadataWithPrefix(filePath)

	if err = s.Buckets.Verify(ctx, dbID, buck, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}

	buckPath := path.New(buck.Path)
	var dirPath path.Resolved
	if buck.IsPrivate() {
		ctx, dirPath, err = s.removeNodeAtPath(
			ctx,
			path.Join(path.New(buck.Path), filePath),
			buck.GetLinkEncryptionKey(),
		)
		if err != nil {
			return nil, err
		}
	} else {
		dirPath, err = s.IPFSClient.Object().RmLink(ctx, buckPath, filePath)
		if err != nil {
			return nil, err
		}
		ctx, err = s.updateOrAddPin(ctx, buckPath, dirPath)
		if err != nil {
			return nil, err
		}
	}

	buck.Path = dirPath.String()
	if err = s.Buckets.Save(ctx, dbID, buck, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}

	go s.IPNSManager.Publish(dirPath, buck.Key)

	log.Debugf("removed %s from bucket: %s", filePath, buck.Key)
	root, err := getPbRoot(dbID, buck)
	if err != nil {
		return nil, err
	}
	return &pb.RemovePathResponse{
		Root:   root,
		Pinned: s.getPinnedBytes(ctx),
	}, nil
}

// removeNodeAtPath removes node at the location of path.
// Key will be required if the path is encrypted.
func (s *Service) removeNodeAtPath(
	ctx context.Context,
	pth path.Path,
	key []byte,
) (context.Context, path.Resolved, error) {
	// The first step here is find a resolvable list of nodes that point to path.
	rp, fp, err := util.ParsePath(pth)
	if err != nil {
		return ctx, nil, err
	}
	np, _, r, err := s.getNodesToPath(ctx, rp, fp, key)
	if err != nil {
		return ctx, nil, err
	}
	// If the remaining path segment is not empty, then we conclude that
	// the node cannot be resolved.
	if r != "" {
		return ctx, nil, fmt.Errorf("could not resolve path: %s", pth)
	}
	np[len(np)-1].isJoint = true

	// Now, we have a full list of nodes to the insert location,
	// but the existing nodes need to be updated and re-encrypted.
	change := make([]ipld.Node, len(np)-1)
	for i := len(np) - 1; i >= 0; i-- {
		if i < len(np)-1 {
			change[i] = np[i].new
		}
		if i > 0 {
			p, ok := np[i-1].new.(*dag.ProtoNode)
			if !ok {
				return ctx, nil, dag.ErrNotProtobuf
			}
			if err := p.RemoveNodeLink(np[i].name); err != nil {
				return ctx, nil, err
			}
			if !np[i].isJoint {
				if err := p.AddNodeLink(np[i].name, np[i].new); err != nil {
					return ctx, nil, err
				}
			}
			np[i-1].new, err = encryptNode(p, key)
			if err != nil {
				return ctx, nil, err
			}
		}
	}

	// Add all the changed nodes
	if err := s.IPFSClient.Dag().AddMany(ctx, change); err != nil {
		return ctx, nil, err
	}
	// Update / remove node pins
	for _, n := range np {
		if n.isJoint {
			ctx, err = s.unpinNodeAndBranch(ctx, n.old, key)
			if err != nil {
				return ctx, nil, err
			}
		} else {
			ctx, err = s.updateOrAddPin(ctx, n.old, path.IpfsPath(n.new.Cid()))
			if err != nil {
				return ctx, nil, err
			}
		}
	}
	return ctx, path.IpfsPath(np[0].new.Cid()), nil
}

func (s *Service) PushPathAccessRoles(
	ctx context.Context,
	req *pb.PushPathAccessRolesRequest,
) (res *pb.PushPathAccessRolesResponse, err error) {
	log.Debugf("received push path access roles request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	reqPath, err := parsePath(req.Path)
	if err != nil {
		return nil, err
	}

	lck := s.Semaphores.Get(buckLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	buck, pth, err := s.getBucketPath(ctx, dbID, req.Key, reqPath, dbToken)
	if err != nil {
		return nil, err
	}
	linkKey := buck.GetLinkEncryptionKey()
	pathNode, err := s.getNodeAtPath(ctx, pth, linkKey)
	if err != nil {
		return nil, err
	}

	var currentFileKeys map[string][]byte
	md, mdPath, ok := buck.GetMetadataForPath(reqPath, false)
	if !ok {
		return nil, fmt.Errorf("could not resolve path: %s", reqPath)
	}
	var target tdb.Metadata
	if mdPath != reqPath { // If the metadata is inherited from a parent, create a new entry
		target = tdb.Metadata{
			Roles: make(map[string]buckets.Role),
		}
	} else {
		target = md
	}
	if buck.IsPrivate() {
		currentFileKeys, err = buck.GetFileEncryptionKeysForPrefix(reqPath)
		if err != nil {
			return nil, err
		}
		newFileKey, err := dcrypto.NewKey()
		if err != nil {
			return nil, err
		}
		target.SetFileEncryptionKey(newFileKey) // Create or refresh the file key
	}

	newRoles, err := buckets.RolesFromPb(req.Roles)
	if err != nil {
		return nil, err
	}
	var changed bool
	for k, r := range newRoles {
		if x, ok := target.Roles[k]; ok && x == r {
			continue
		}
		if r > buckets.None {
			target.Roles[k] = r
		} else {
			delete(target.Roles, k)
		}
		changed = true
	}
	if changed {
		buck.UpdatedAt = time.Now().UnixNano()
		target.UpdatedAt = buck.UpdatedAt
		buck.Metadata[reqPath] = target
		if buck.IsPrivate() {
			if err := buck.RotateFileEncryptionKeysForPrefix(reqPath); err != nil {
				return nil, err
			}
		}

		if err = s.Buckets.Verify(ctx, dbID, buck, tdb.WithToken(dbToken)); err != nil {
			return nil, err
		}

		if buck.IsPrivate() {
			newFileKeys, err := buck.GetFileEncryptionKeysForPrefix(reqPath)
			if err != nil {
				return nil, err
			}
			nmap, err := s.encryptDag(
				ctx,
				s.IPFSClient.Dag(),
				pathNode,
				reqPath,
				linkKey,
				currentFileKeys,
				newFileKeys,
				nil,
				nil,
			)
			if err != nil {
				return nil, err
			}
			nodes := make([]ipld.Node, len(nmap))
			i := 0
			for _, tn := range nmap {
				nodes[i] = tn.node
				i++
			}
			pn := nmap[pathNode.Cid()].node
			var dirPath path.Resolved
			ctx, dirPath, err = s.insertNodeAtPath(ctx, pn, path.Join(path.New(buck.Path), reqPath), linkKey)
			if err != nil {
				return nil, fmt.Errorf("updating pinned root: %s", err)
			}
			ctx, err = s.addAndPinNodes(ctx, nodes)
			if err != nil {
				return nil, err
			}
			buck.Path = dirPath.String()
		}

		if err = s.Buckets.Save(ctx, dbID, buck, tdb.WithToken(dbToken)); err != nil {
			return nil, err
		}
	}
	return &pb.PushPathAccessRolesResponse{
		Pinned: s.getPinnedBytes(ctx),
	}, nil
}

func (s *Service) PullPathAccessRoles(
	ctx context.Context,
	req *pb.PullPathAccessRolesRequest,
) (*pb.PullPathAccessRolesResponse, error) {
	log.Debugf("received pull path access roles request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	reqPath, err := parsePath(req.Path)
	if err != nil {
		return nil, err
	}
	buck, pth, err := s.getBucketPath(ctx, dbID, req.Key, reqPath, dbToken)
	if err != nil {
		return nil, err
	}
	if _, err = s.getNodeAtPath(ctx, pth, buck.GetLinkEncryptionKey()); err != nil {
		return nil, fmt.Errorf("could not resolve path: %s", reqPath)
	}
	md, _, ok := buck.GetMetadataForPath(reqPath, false)
	if !ok {
		return nil, fmt.Errorf("could not resolve path: %s", reqPath)
	}
	roles, err := buckets.RolesToPb(md.Roles)
	if err != nil {
		return nil, err
	}
	return &pb.PullPathAccessRolesResponse{
		Roles: roles,
	}, nil
}

func (s *Service) DefaultArchiveConfig(
	ctx context.Context,
	req *pb.DefaultArchiveConfigRequest,
) (*pb.DefaultArchiveConfigResponse, error) {
	log.Debug("received default archive config request")

	if !s.Buckets.IsArchivingEnabled() {
		return nil, ErrArchivingFeatureDisabled
	}

	ba, err := s.Collections.BucketArchives.GetOrCreate(ctx, req.Key)
	if err != nil {
		return nil, fmt.Errorf("getting bucket archive data: %s", err)
	}
	archiveConfig := ba.DefaultArchiveConfig
	if archiveConfig == nil {
		// The queried BucketArchive was created before we added .DefaultArchiveConfig, so just return the default
		archiveConfig = &defaultDefaultArchiveConfig
	}
	return &pb.DefaultArchiveConfigResponse{
		ArchiveConfig: toPbArchiveConfig(archiveConfig),
	}, nil
}

func (s *Service) SetDefaultArchiveConfig(
	ctx context.Context,
	req *pb.SetDefaultArchiveConfigRequest,
) (*pb.SetDefaultArchiveConfigResponse, error) {
	log.Debug("received set default archive config request")

	if !s.Buckets.IsArchivingEnabled() {
		return nil, ErrArchivingFeatureDisabled
	}

	ba, err := s.Collections.BucketArchives.GetOrCreate(ctx, req.Key)
	if err != nil {
		return nil, fmt.Errorf("getting bucket archive data: %s", err)
	}

	c := fromPbArchiveConfig(req.ArchiveConfig)
	if err := s.validateArchiveConfig(c); err != nil {
		return nil, fmt.Errorf("validating archive config: %s", err)
	}

	ba.DefaultArchiveConfig = c
	err = s.Collections.BucketArchives.Replace(ctx, ba)
	if err != nil {
		return nil, fmt.Errorf("saving default archive config: %s", err)
	}
	return &pb.SetDefaultArchiveConfigResponse{}, nil
}

func (s *Service) Archive(ctx context.Context, req *pb.ArchiveRequest) (*pb.ArchiveResponse, error) {
	log.Debug("received archive request")

	if !s.Buckets.IsArchivingEnabled() {
		return nil, ErrArchivingFeatureDisabled
	}

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, errDBRequired
	}
	dbToken, _ := thread.TokenFromContext(ctx)
	account, _ := mdb.AccountFromContext(ctx)

	lck := s.Semaphores.Get(buckLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	buck := &tdb.Bucket{}
	err := s.Buckets.GetSafe(ctx, dbID, req.Key, buck, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	p, err := util.NewResolvedPath(buck.Path)
	if err != nil {
		return nil, fmt.Errorf("parsing cid path: %s", err)
	}

	createNewUser := func() error {
		ctxAdmin := context.WithValue(ctx, pow.AdminKey, s.PowergateAdminToken)
		res, err := s.PowergateClient.Admin.Users.Create(ctxAdmin)
		if err != nil {
			return fmt.Errorf("creating new powergate integration: %v", err)
		}
		_, err = s.Collections.Accounts.UpdatePowInfo(ctx, account.Owner().Key, &mdb.PowInfo{ID: res.User.Id, Token: res.User.Token})
		if err != nil {
			return fmt.Errorf("updating user/account with new powergate information: %v", err)
		}
		return nil
	}

	tryAgain := fmt.Errorf(
		"new powergate integration created, please try again in 30 seconds to allow time for wallet funding")

	// Case where account/user was created before bucket archives were enabled.
	// create a user for them.
	if account.Owner().PowInfo == nil {
		if err := createNewUser(); err != nil {
			return nil, err
		}
		return nil, tryAgain
	}

	ctxPow := context.WithValue(ctx, pow.AuthKey, account.Owner().PowInfo.Token)

	defConfRes, err := s.PowergateClient.StorageConfig.Default(ctxPow)
	if err != nil {
		if !strings.Contains(err.Error(), "auth token not found") {
			return nil, fmt.Errorf("getting powergate default StorageConfig: %v", err)
		} else {
			// case where the user token is no longer valid because powergate was reset.
			// create a new user for them.
			if err := createNewUser(); err != nil {
				return nil, err
			}
			return nil, tryAgain
		}
	}

	ba, err := s.Collections.BucketArchives.GetOrCreate(ctx, req.Key)
	if err != nil {
		return nil, fmt.Errorf("getting bucket archive data: %s", err)
	}

	var archiveConfig *mdb.ArchiveConfig
	if req.ArchiveConfig != nil {
		archiveConfig = fromPbArchiveConfig(req.ArchiveConfig)
	} else if ba.DefaultArchiveConfig != nil {
		archiveConfig = ba.DefaultArchiveConfig
	} else {
		// The queried BucketArchive was created before we added .DefaultArchiveConfig so just use the default default
		// and defer saving the default until the user calls DefaultArchiveConfig or SetDefaultArchiveConfig.
		archiveConfig = &defaultDefaultArchiveConfig
	}

	if err := s.validateArchiveConfig(archiveConfig); err != nil {
		return nil, fmt.Errorf("validating archive config: %s", err)
	}

	storageConfig := baseArchiveStorageConfig
	storageConfig.Cold.Filecoin = toFilConfig(archiveConfig)
	// Get the address from the default storage config for this user.
	storageConfig.Cold.Filecoin.Address = defConfRes.DefaultStorageConfig.Cold.Filecoin.Address

	// Check that user wallet addr balance is > 0, if not, fail fast.
	balRes, err := s.PowergateClient.Wallet.Balance(ctx, storageConfig.Cold.Filecoin.Address)
	if err != nil {
		return nil, fmt.Errorf("getting powergate wallet address balance: %s", err)
	}
	bal, ok := new(big.Int).SetString(balRes.Balance, 10)
	if !ok {
		return nil, fmt.Errorf("error converting balance %v to big int", balRes.Balance)
	}
	if bal.Cmp(big.NewInt(0)) == 0 {
		return nil, buckets.ErrZeroBalance
	}

	// Archive pushes the current root Cid to the corresponding user of the bucket.
	// The behaviour changes depending on different cases, depending on a previous archive.
	// 0. No previous archive or last one aborted: simply pushes the Cid to Powergate.
	// 1. Last archive exists with the same Cid:
	//   a. Last archive Successful: fails, there's nothing to do.
	//   b. Last archive Executing/Queued: fails, that work already starting and is in progress.
	//   c. Last archive Failed/Canceled: work to do, push again with override flag to try again.
	// 2. Archiving on new Cid: work to do, it will always call Replace(,).
	var jid string
	firstTimeArchive := ba.Archives.Current.JobID == ""
	if firstTimeArchive || ba.Archives.Current.Aborted { // Case 0.
		// On the first archive, we simply push the Cid with
		// the default CidConfig configured at bucket creation.
		res, err := s.PowergateClient.StorageConfig.Apply(
			ctxPow,
			p.Cid().String(),
			pow.WithStorageConfig(storageConfig),
			pow.WithOverride(true),
		)
		if err != nil {
			return nil, fmt.Errorf("pushing config: %s", err)
		}
		jid = res.JobId
	} else {
		oldCid, err := cid.Cast(ba.Archives.Current.Cid)
		if err != nil {
			return nil, fmt.Errorf("parsing old Cid archive: %s", err)
		}

		statusName, found := userPb.JobStatus_name[int32(ba.Archives.Current.Status)]
		if !found {
			return nil, fmt.Errorf("invalid job status %v", ba.Archives.Current.Status)
		}

		st := userPb.JobStatus(ba.Archives.Current.Status)

		if oldCid.Equals(p.Cid()) { // Case 1.
			switch st {
			// Case 1.a.
			case userPb.JobStatus_JOB_STATUS_SUCCESS:
				return nil, fmt.Errorf("the same bucket cid is already archived successfully")
			// Case 1.b.
			case userPb.JobStatus_JOB_STATUS_EXECUTING, userPb.JobStatus_JOB_STATUS_QUEUED:
				return nil, fmt.Errorf("there is an in progress archive")
			// Case 1.c.
			case userPb.JobStatus_JOB_STATUS_FAILED, userPb.JobStatus_JOB_STATUS_CANCELED:
				res, err := s.PowergateClient.StorageConfig.Apply(
					ctxPow,
					p.Cid().String(),
					pow.WithStorageConfig(storageConfig),
					pow.WithOverride(true),
				)
				if err != nil {
					return nil, fmt.Errorf("pushing config: %s", err)
				}
				jid = res.JobId
			default:
				return nil, fmt.Errorf("unexpected current archive status: %s", statusName)
			}
		} else { // Case 2.
			res, err := s.PowergateClient.Data.CidInfo(ctxPow, oldCid.String())
			if err != nil {
				return nil, fmt.Errorf("looking up old storage config: %s", err)
			}
			if len(res.CidInfos) == 0 {
				return nil, fmt.Errorf("no cid info returned")
			}

			if cmp.Equal(&storageConfig, res.CidInfos[0].LatestPushedStorageConfig) {
				// Old storage config is the same as the new so use replace.
				res, err := s.PowergateClient.Data.ReplaceData(ctxPow, oldCid.String(), p.Cid().String())
				if err != nil {
					return nil, fmt.Errorf("replacing cid: %s", err)
				}
				jid = res.JobId
			} else {
				// New storage config, so remove and push.
				_, err = s.PowergateClient.StorageConfig.Apply(
					ctxPow,
					oldCid.String(),
					pow.WithStorageConfig(&userPb.StorageConfig{}),
					pow.WithOverride(true),
				)
				if err != nil {
					return nil, fmt.Errorf("pushing config to disable hot and cold storage: %s", err)
				}
				_, err = s.PowergateClient.StorageConfig.Remove(ctxPow, oldCid.String())
				if err != nil {
					return nil, fmt.Errorf("removing old cid storage: %s", err)
				}
				res, err := s.PowergateClient.StorageConfig.Apply(
					ctxPow,
					p.Cid().String(),
					pow.WithStorageConfig(storageConfig),
					pow.WithOverride(true),
				)
				if err != nil {
					return nil, fmt.Errorf("pushing config: %s", err)
				}
				jid = res.JobId
			}

		}

		// Include the existing archive in history,
		// since we're going to set a new _current_ archive.
		ba.Archives.History = append(ba.Archives.History, ba.Archives.Current)
	}
	ba.Archives.Current = mdb.Archive{
		Cid:       p.Cid().Bytes(),
		CreatedAt: time.Now().Unix(),
		JobID:     jid,
		Status:    int(userPb.JobStatus_JOB_STATUS_QUEUED),
	}
	if err := s.Collections.BucketArchives.Replace(ctx, ba); err != nil {
		return nil, fmt.Errorf("updating bucket archives data: %s", err)
	}

	if err := s.ArchiveTracker.Track(ctx, dbID, dbToken, req.Key, jid, p.Cid(), account.Owner().Key); err != nil {
		return nil, fmt.Errorf("scheduling archive tracking: %s", err)
	}

	log.Debug("archived bucket")
	return &pb.ArchiveResponse{}, nil
}

func (s *Service) Archives(ctx context.Context, req *pb.ArchivesRequest) (*pb.ArchivesResponse, error) {
	ba, err := s.Collections.BucketArchives.GetOrCreate(ctx, req.Key)
	if err != nil {
		return nil, fmt.Errorf("getting bucket archive data: %s", err)
	}
	res := &pb.ArchivesResponse{}
	if ba.Archives.Current.JobID != "" {
		arc, err := toPbArchive(ba.Archives.Current)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "converting to pb archive: %v", err)
		}
		res.Current = arc
	}
	history := make([]*pb.Archive, len(ba.Archives.History))
	for i, item := range ba.Archives.History {
		pbItem, err := toPbArchive(item)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "converting to pb archive: %v", err)
		}
		history[i] = pbItem
	}
	res.History = history
	return res, nil
}

func toPbArchive(archive mdb.Archive) (*pb.Archive, error) {
	c, err := cid.Cast(archive.Cid)
	if err != nil {
		return nil, fmt.Errorf("casting cid: %v", err)
	}
	_, ok := pb.ArchiveStatus_name[int32(archive.Status)]
	if !ok {
		return nil, fmt.Errorf("unknown job status: %v", archive.Status)
	}
	dealInfo := make([]*pb.DealInfo, len(archive.DealInfo))
	for i, info := range archive.DealInfo {
		dealInfo[i] = &pb.DealInfo{
			ActivationEpoch: info.ActivationEpoch,
			DealId:          info.DealID,
			Duration:        info.Duration,
			Message:         info.Message,
			Miner:           info.Miner,
			PieceCid:        info.PieceCID,
			PricePerEpoch:   info.PricePerEpoch,
			ProposalCid:     info.ProposalCid,
			Size:            info.Size,
			StartEpoch:      info.StartEpoch,
			StateId:         info.StateID,
			StateName:       info.StateName,
		}
	}
	return &pb.Archive{
		Aborted:       archive.Aborted,
		AbortedMsg:    archive.AbortedMsg,
		Cid:           c.String(),
		CreatedAt:     archive.CreatedAt,
		DealInfo:      dealInfo,
		FailureMsg:    archive.FailureMsg,
		JobId:         archive.JobID,
		ArchiveStatus: pb.ArchiveStatus(archive.Status),
	}, nil
}

func (s *Service) ArchiveWatch(req *pb.ArchiveWatchRequest, server pb.APIService_ArchiveWatchServer) error {
	log.Debug("received archive watch")

	if !s.Buckets.IsArchivingEnabled() {
		return ErrArchivingFeatureDisabled
	}

	account, _ := mdb.AccountFromContext(server.Context())
	if account.Owner().PowInfo == nil {
		return fmt.Errorf("no powergate info associated with account")
	}

	var err error
	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()
	ch := make(chan string)
	go func() {
		err = s.Buckets.ArchiveWatch(ctx, req.Key, account.Owner().PowInfo.Token, ch)
		close(ch)
	}()
	for s := range ch {
		if err := server.Send(&pb.ArchiveWatchResponse{Msg: s}); err != nil {
			return err
		}
	}
	if err != nil {
		return fmt.Errorf("watching cid logs: %s", err)
	}
	return nil
}

func (s *Service) ArchivesLs(ctx context.Context, req *pb.ArchivesLsRequest) (*pb.ArchivesLsResponse, error) {
	account, _ := mdb.AccountFromContext(ctx)
	if account.Owner().PowInfo == nil {
		return nil, fmt.Errorf("no powergate info associated with account")
	}

	ctx = context.WithValue(ctx, pow.AuthKey, account.Owner().PowInfo.Token)
	r, err := s.PowergateClient.StorageInfo.ListStorageInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting archived cids: %s", err)
	}

	res := &pb.ArchivesLsResponse{
		Archives: make([]*pb.ArchiveLsItem, len(r.StorageInfo)),
	}
	for i, si := range r.StorageInfo {
		props := si.Cold.Filecoin.Proposals
		ali := &pb.ArchiveLsItem{
			Cid:  si.Cid,
			Info: make([]*pb.ArchiveLsItemMetadata, len(props)),
		}
		res.Archives[i] = ali

		for j, p := range props {
			ali.Info[j] = &pb.ArchiveLsItemMetadata{
				DealId: uint64(p.DealId),
			}
		}
	}

	return res, nil
}

func (s *Service) ArchivesImport(ctx context.Context, req *pb.ArchivesImportRequest) (*pb.ArchivesImportResponse, error) {
	account, _ := mdb.AccountFromContext(ctx)
	if account.Owner().PowInfo == nil {
		return nil, fmt.Errorf("no powergate info associated with account")
	}

	var scfg *userPb.StorageConfig

	ctx = context.WithValue(ctx, pow.AuthKey, account.Owner().PowInfo.Token)
	ci, err := s.PowergateClient.Data.CidInfo(ctx, req.Cid)
	var notFound bool
	if err != nil {
		sc, ok := status.FromError(err)
		if !ok || sc.Code() != codes.NotFound {
			return nil, fmt.Errorf("getting current storage information: %s", err)
		}
		notFound = true
	}

	// If deal import is for a new Cid, just use the default Storage Config
	// with both storages disabled and without running any jobs: only deal importing.
	if notFound {
		defConfRes, err := s.PowergateClient.StorageConfig.Default(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting default storage-config: %s", err)
		}
		scfg = defConfRes.DefaultStorageConfig
		scfg.Cold.Enabled = false
		scfg.Hot.Enabled = false

	} else {
		// If deal import is to augment an existing Cid, just use the latest storage config.
		// A Job won't run anyway, so it would only import the deals.
		scfg = ci.CidInfos[0].LatestPushedStorageConfig
	}

	if _, err = s.PowergateClient.StorageConfig.Apply(
		ctx,
		req.Cid,
		pow.WithStorageConfig(scfg),
		pow.WithOverride(true),
		pow.WithImportDealIDs(req.DealIds),
		pow.WithNoExec(true),
	); err != nil {
		return nil, fmt.Errorf("importing deals: %s", err)
	}

	return &pb.ArchivesImportResponse{}, nil
}

func (s *Service) ArchiveRetrievalLs(ctx context.Context, req *pb.ArchiveRetrievalLsRequest) (*pb.ArchiveRetrievalLsResponse, error) {
	account, _ := mdb.AccountFromContext(ctx)
	owner := account.Owner().Key

	rs, err := s.FilRetrieval.GetAllByAccount(owner.String())
	if err != nil {
		return nil, fmt.Errorf("listing retrievals: %s", err)
	}

	res := &pb.ArchiveRetrievalLsResponse{
		Archives: make([]*pb.ArchiveRetrievalLsItem, len(rs)),
	}
	for i, r := range rs {
		res.Archives[i] = &pb.ArchiveRetrievalLsItem{
			Id:           r.JobID,
			Cid:          r.Cid.String(),
			Status:       toPbRetrievalStatus(r.Status),
			FailureCause: r.FailureCause,
			CreatedAt:    r.CreatedAt,
		}
		switch r.Type {
		case retrieval.TypeNewBucket:
			rt := &pb.ArchiveRetrievalLsItem_NewBucket{
				NewBucket: &pb.ArchiveRetrievalLsItemNewBucket{
					Name:    r.Name,
					Private: r.Private,
				},
			}
			res.Archives[i].RetrievalType = rt
		default:
			return nil, fmt.Errorf("unkown retrieval type")
		}
	}

	return res, nil
}

func (s *Service) ArchiveRetrievalLogs(req *pb.ArchiveRetrievalLogsRequest, server pb.APIService_ArchiveRetrievalLogsServer) error {
	account, _ := mdb.AccountFromContext(server.Context())
	owner := account.Owner()
	accKey := owner.Key.String()
	powToken := owner.PowInfo.Token

	var err error
	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()
	ch := make(chan string)
	go func() {
		err = s.FilRetrieval.Logs(ctx, accKey, req.Id, powToken, ch)
		close(ch)
	}()
	for s := range ch {
		if err := server.Send(&pb.ArchiveRetrievalLogsResponse{Msg: s}); err != nil {
			return err
		}
	}
	if err != nil {
		return fmt.Errorf("watching retrieval logs: %s", err)
	}

	return nil
}

func toPbArchiveConfig(config *mdb.ArchiveConfig) *pb.ArchiveConfig {
	var pbConfig *pb.ArchiveConfig
	if config != nil {
		pbConfig = &pb.ArchiveConfig{
			RepFactor:       int32(config.RepFactor),
			DealMinDuration: config.DealMinDuration,
			ExcludedMiners:  config.ExcludedMiners,
			TrustedMiners:   config.TrustedMiners,
			CountryCodes:    config.CountryCodes,
			Renew: &pb.ArchiveRenew{
				Enabled:   config.Renew.Enabled,
				Threshold: int32(config.Renew.Threshold),
			},
			MaxPrice:        config.MaxPrice,
			FastRetrieval:   config.FastRetrieval,
			DealStartOffset: config.DealStartOffset,
		}
	}
	return pbConfig
}

func toPbRetrievalStatus(s retrieval.Status) pb.ArchiveRetrievalStatus {
	switch s {
	case retrieval.StatusQueued:
		return pb.ArchiveRetrievalStatus_QUEUED
	case retrieval.StatusExecuting:
		return pb.ArchiveRetrievalStatus_EXECUTING
	case retrieval.StatusMoveToBucket:
		return pb.ArchiveRetrievalStatus_MOVETOBUCKET
	case retrieval.StatusSuccess:
		return pb.ArchiveRetrievalStatus_SUCCESS
	case retrieval.StatusFailed:
		return pb.ArchiveRetrievalStatus_FAILED
	default:
		return pb.ArchiveRetrievalStatus_UNSPECIFIED
	}
}

func fromPbArchiveConfig(pbConfig *pb.ArchiveConfig) *mdb.ArchiveConfig {
	var config *mdb.ArchiveConfig
	if pbConfig != nil {
		config = &mdb.ArchiveConfig{
			RepFactor:       int(pbConfig.RepFactor),
			DealMinDuration: pbConfig.DealMinDuration,
			ExcludedMiners:  pbConfig.ExcludedMiners,
			TrustedMiners:   pbConfig.TrustedMiners,
			CountryCodes:    pbConfig.CountryCodes,
			MaxPrice:        pbConfig.MaxPrice,
			FastRetrieval:   pbConfig.FastRetrieval,
			DealStartOffset: pbConfig.DealStartOffset,
		}
		if pbConfig.Renew != nil {
			config.Renew = mdb.ArchiveRenew{
				Enabled:   pbConfig.Renew.Enabled,
				Threshold: int(pbConfig.Renew.Threshold),
			}
		}
	}
	return config
}

func toFilConfig(config *mdb.ArchiveConfig) *userPb.FilConfig {
	if config == nil {
		return nil
	}
	return &userPb.FilConfig{
		ReplicationFactor: int64(config.RepFactor),
		CountryCodes:      config.CountryCodes,
		DealMinDuration:   config.DealMinDuration,
		DealStartOffset:   config.DealStartOffset,
		ExcludedMiners:    config.ExcludedMiners,
		FastRetrieval:     config.FastRetrieval,
		MaxPrice:          config.MaxPrice,
		Renew: &userPb.FilRenew{
			Enabled:   config.Renew.Enabled,
			Threshold: int64(config.Renew.Threshold),
		},
		TrustedMiners: config.TrustedMiners,
	}
}

func (s *Service) validateArchiveConfig(c *mdb.ArchiveConfig) error {
	if c.RepFactor > s.MaxBucketArchiveRepFactor {
		return fmt.Errorf("rep factor %d is greater than max allowed of %d", c.RepFactor, s.MaxBucketArchiveRepFactor)
	}
	if c.DealMinDuration < powUtil.MinDealDuration {
		return fmt.Errorf("min deal duration %d is less than the allowed minimum of %d", c.DealMinDuration, powUtil.MinDealDuration)
	}
	if c.DealStartOffset <= 0 {
		return fmt.Errorf("deal start offset of %d is less than required minimum of 1, and really should be higher than 1", c.DealStartOffset)
	}
	return nil
}
