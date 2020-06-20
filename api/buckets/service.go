package buckets

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	ipfsfiles "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-unixfs"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/dcrypto"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/powergate/ffs"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/api/common"
	bucks "github.com/textileio/textile/buckets"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/dns"
	"github.com/textileio/textile/ipns"
	"github.com/textileio/textile/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("bucketsapi")

	// ErrArchivingFeatureDisabled indicates an archive was requested with archiving disabled.
	ErrArchivingFeatureDisabled = fmt.Errorf("archiving feature is disabled")
)

const (
	// chunkSize for get file requests.
	chunkSize = 1024 * 32
	// pinNotRecursiveMsg is used to match an IPFS "recursively pinned already" error.
	pinNotRecursiveMsg = "'from' cid was not recursively pinned already"
)

// Service is a gRPC service for buckets.
type Service struct {
	Collections *c.Collections
	Buckets     *bucks.Buckets
	GatewayURL  string
	IPFSClient  iface.CoreAPI
	IPNSManager *ipns.Manager
	DNSManager  *dns.Manager
}

func (s *Service) Init(ctx context.Context, req *pb.InitRequest) (*pb.InitReply, error) {
	log.Debugf("received init request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	var key []byte
	if req.Private {
		var err error
		key, err = dcrypto.NewKey()
		if err != nil {
			return nil, err
		}
	}
	var bootCid cid.Cid
	var err error
	if req.BootstrapCid != "" {
		bootCid, err = cid.Decode(req.BootstrapCid)
		if err != nil {
			return nil, fmt.Errorf("invalid bootstrap cid: %s", err)
		}
	}
	buck, seed, err := s.createBucket(ctx, dbID, dbToken, req.Name, key, bootCid)
	if err != nil {
		return nil, err
	}

	return &pb.InitReply{
		Root: &pb.Root{
			Key:       buck.Key,
			Name:      buck.Name,
			Path:      buck.Path,
			CreatedAt: buck.CreatedAt,
			UpdatedAt: buck.UpdatedAt,
		},
		Links: s.createLinks(dbID, buck),
		Seed:  seed.RawData(),
	}, nil
}

// createBucket returns a new bucket and seed node.
func (s *Service) createBucket(ctx context.Context, dbID thread.ID, dbToken thread.Token, name string, key []byte, bootCid cid.Cid) (buck *bucks.Bucket, seed ipld.Node, err error) {
	// Make a random seed, which ensures a bucket's uniqueness
	seed, err = makeSeed(key)
	if err != nil {
		return
	}

	// Create the bucket directory
	var pth path.Resolved
	if bootCid.Defined() {
		pth, err = s.createBootstrappedPath(ctx, seed, bootCid, key)
		if err != nil {
			return nil, nil, fmt.Errorf("creating prepared bucket: %s", err)
		}
	} else {
		pth, err = s.createPristinePath(ctx, seed, key)
		if err != nil {
			return nil, nil, fmt.Errorf("creating pristine bucket: %s", err)
		}
	}

	// Create a new IPNS key
	bkey, err := s.IPNSManager.CreateKey(ctx, dbID)
	if err != nil {
		return
	}

	// Create the bucket, using the IPNS key as instance ID
	buck, err = s.Buckets.Create(ctx, dbID, bkey, pth, bucks.WithName(name), bucks.WithKey(key), bucks.WithToken(dbToken))
	if err != nil {
		return
	}

	// Add a DNS record if possible
	if s.DNSManager != nil {
		if host, ok := s.getGatewayHost(); ok {
			rec, err := s.DNSManager.NewCNAME(buck.Key, host)
			if err != nil {
				return nil, nil, err
			}
			buck.DNSRecord = rec.ID
			if err = s.Buckets.Save(ctx, dbID, buck, bucks.WithToken(dbToken)); err != nil {
				return nil, nil, err
			}
		}
	}

	// Finally, publish the new bucket's address to the name system
	go s.IPNSManager.Publish(pth, buck.Key)
	return buck, seed, nil
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
		seed, err = encryptData(seed, key)
		if err != nil {
			return nil, err
		}
	}
	return dag.NewRawNode(seed), nil
}

func encryptData(data, key []byte) ([]byte, error) {
	r, err := dcrypto.NewEncrypter(bytes.NewReader(data), key)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(r)
}

// createPristinePath creates an IPFS path which only contains the seed file.
// The returned path will be pinned.
func (s *Service) createPristinePath(ctx context.Context, seed ipld.Node, key []byte) (path.Resolved, error) {
	// Create the initial bucket directory
	n, err := newDirWithNode(seed, bucks.SeedName, key)
	if err != nil {
		return nil, err
	}
	if err := s.addAndPinNodes(ctx, []ipld.Node{n, seed}); err != nil {
		return nil, err
	}
	return path.IpfsPath(n.Cid()), nil
}

// createBootstrapedPath creates an IPFS path which is the bootCid UnixFS DAG,
// with bucks.SeedName seed file added to the root of the DAG. The returned path will
// be pinned.
func (s *Service) createBootstrappedPath(ctx context.Context, seed ipld.Node, bootCid cid.Cid, key []byte) (path.Resolved, error) {
	// Here we have to walk and possibly encrypt the boot path dag
	n, nodes, err := s.newDirFromExistingPath(ctx, path.IpfsPath(bootCid), key, seed, bucks.SeedName)
	if err != nil {
		return nil, err
	}
	if err := s.addAndPinNodes(ctx, nodes); err != nil {
		return nil, err
	}
	return path.IpfsPath(n.Cid()), nil
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

// encryptNode returns the encrypted versoin of node if key is not nil.
func encryptNode(n *dag.ProtoNode, key []byte) (*dag.ProtoNode, error) {
	if key == nil {
		return n, nil
	}
	cipher, err := encryptData(n.RawData(), key)
	if err != nil {
		return nil, err
	}
	en := dag.NodeWithData(unixfs.FilePBData(cipher, uint64(len(cipher))))
	en.SetCidBuilder(dag.V1CidPrefix())
	return en, nil
}

type treeNode struct {
	name string
	node ipld.Node
}

// newDirFromExistingPath returns a new dir based on path.
// If key is not nil, this method recursively walks the path, encrypting files and directories.
// If add is not nil, it will be included in the resulting (possibly encrypted) node under a link named addName.
// This method returns the root node and a list of all new nodes (which also includes the root).
func (s *Service) newDirFromExistingPath(ctx context.Context, pth path.Path, key []byte, add ipld.Node, addName string) (ipld.Node, []ipld.Node, error) {
	rn, err := s.IPFSClient.ResolveNode(ctx, pth)
	if err != nil {
		return nil, nil, err
	}
	top, ok := rn.(*dag.ProtoNode)
	if !ok {
		return nil, nil, dag.ErrNotProtobuf
	}
	if key == nil {
		nodes := []ipld.Node{top}
		if add != nil {
			if err := top.AddNodeLink(addName, add); err != nil {
				return nil, nil, err
			}
			nodes = append(nodes, add)
		}
		return top, nodes, nil
	}

	// Walk the node, encrypting the leaves and directories
	tree := make(map[cid.Cid]*treeNode)
	var addNode *treeNode
	if add != nil {
		addNode = &treeNode{
			name: addName,
			node: add,
		}
	}
	root, err := s.encryptTree(ctx, tree, top, "", key, addNode)
	if err != nil {
		return nil, nil, err
	}

	// Collect new nodes
	nodes := make([]ipld.Node, len(tree))
	i := 0
	for _, tn := range tree {
		nodes[i] = tn.node
		i++
	}
	return root.node, nodes, nil
}

// encryptTree walks all nodes in n, encrypting nodes and re-creating the dag structure.
// add will be added to the encrypted root node if not nil.
// tree will contain a map of all new nodes.
func (s *Service) encryptTree(ctx context.Context, tree map[cid.Cid]*treeNode, root ipld.Node, name string, key []byte, add *treeNode) (*treeNode, error) {
	var tn *treeNode
	switch root.(type) {
	case *dag.RawNode:
		// Encrypt and add the raw node
		data, err := encryptData(root.RawData(), key)
		if err != nil {
			return nil, err
		}
		tn = &treeNode{
			name: name,
			node: dag.NewRawNode(data),
		}
	case *dag.ProtoNode:
		var isLeaf bool
		var leaves []*treeNode
		// Follow each link, wrapping up and encrypting files and dirs
	loop:
		for _, l := range root.Links() {
			if l.Name == "" {
				isLeaf = true
				break loop // We have discovered a raw node wrapper
			}
			ln, err := l.GetNode(ctx, s.IPFSClient.Dag())
			if err != nil {
				return nil, err
			}
			ltn, err := s.encryptTree(ctx, tree, ln, l.Name, key, nil)
			if err != nil {
				return nil, err
			}
			leaves = append(leaves, ltn)
		}
		if !isLeaf {
			// We're done with these links, wrap up the new dir
			dir := unixfs.EmptyDirNode()
			dir.SetCidBuilder(dag.V1CidPrefix())
			for _, l := range leaves {
				if err := dir.AddNodeLink(l.name, l.node); err != nil {
					return nil, err
				}
			}
			if add != nil {
				if err := dir.AddNodeLink(add.name, add.node); err != nil {
					return nil, err
				}
				tree[add.node.Cid()] = add
			}
			cn, err := encryptNode(dir, key)
			if err != nil {
				return nil, err
			}
			tn = &treeNode{
				name: name,
				node: cn,
			}
			break
		}

		// Encrypt and add the file node
		fn, err := s.IPFSClient.Unixfs().Get(ctx, path.IpfsPath(root.Cid()))
		if err != nil {
			return nil, err
		}
		defer fn.Close()
		file := ipfsfiles.ToFile(fn)
		if file == nil {
			return nil, fmt.Errorf("node is a directory")
		}
		r, err := dcrypto.NewEncrypter(file, key)
		if err != nil {
			return nil, err
		}
		pth, err := s.IPFSClient.Unixfs().Add(
			ctx,
			ipfsfiles.NewReaderFile(r),
			options.Unixfs.CidVersion(1),
			options.Unixfs.Pin(false))
		if err != nil {
			return nil, err
		}
		cn, err := s.IPFSClient.ResolveNode(ctx, pth)
		if err != nil {
			return nil, err
		}
		tn = &treeNode{
			name: name,
			node: cn,
		}
	default:
		return nil, fmt.Errorf("cannot get node with unhandled type")
	}
	tree[root.Cid()] = tn
	return tn, nil
}

func (s *Service) addAndPinNodes(ctx context.Context, nodes []ipld.Node) error {
	if err := s.IPFSClient.Dag().AddMany(ctx, nodes); err != nil {
		return err
	}
	return s.IPFSClient.Dag().Pinning().AddMany(ctx, nodes)
}

func (s *Service) Root(ctx context.Context, req *pb.RootRequest) (*pb.RootReply, error) {
	log.Debugf("received root request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	buck, err := s.Buckets.Get(ctx, dbID, req.Key, bucks.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	return &pb.RootReply{
		Root: &pb.Root{
			Key:       buck.Key,
			Name:      buck.Name,
			Path:      buck.Path,
			CreatedAt: buck.CreatedAt,
			UpdatedAt: buck.UpdatedAt,
		},
	}, nil
}

func (s *Service) Links(ctx context.Context, req *pb.LinksRequest) (*pb.LinksReply, error) {
	log.Debugf("received lists request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	buck, err := s.Buckets.Get(ctx, dbID, req.Key, bucks.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	return s.createLinks(dbID, buck), nil
}

func (s *Service) createLinks(dbID thread.ID, buck *bucks.Bucket) *pb.LinksReply {
	var threadLink, wwwLink, ipnsLink string
	threadLink = fmt.Sprintf("%s/thread/%s/%s/%s", s.GatewayURL, dbID, bucks.CollectionName, buck.Key)
	if s.DNSManager != nil && s.DNSManager.Domain != "" {
		parts := strings.Split(s.GatewayURL, "://")
		if len(parts) < 2 {
			return nil
		}
		scheme := parts[0]
		wwwLink = fmt.Sprintf("%s://%s.%s", scheme, buck.Key, s.DNSManager.Domain)
	}
	ipnsLink = fmt.Sprintf("%s/ipns/%s", s.GatewayURL, buck.Key)
	return &pb.LinksReply{
		URL:  threadLink,
		WWW:  wwwLink,
		IPNS: ipnsLink,
	}
}

func (s *Service) SetPath(ctx context.Context, req *pb.SetPathRequest) (*pb.SetPathReply, error) {
	log.Debugf("received set path request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	buck, err := s.Buckets.Get(ctx, dbID, req.Key, bucks.WithToken(dbToken))
	if err != nil {
		return nil, fmt.Errorf("get bucket: %s", err)
	}
	buckPath := path.New(buck.Path)
	encKey := buck.GetEncKey()

	remoteCid, err := cid.Decode(req.Cid)
	if err != nil {
		return nil, fmt.Errorf("invalid remote cid: %s", err)
	}
	remotePath := path.IpfsPath(remoteCid)

	var dirpth path.Resolved
	if req.Path == "" {
		sn, err := makeSeed(encKey)
		if err != nil {
			return nil, fmt.Errorf("generating new seed: %s", err)
		}
		dirpth, err = s.createBootstrappedPath(ctx, sn, remoteCid, encKey)
		if err != nil {
			return nil, fmt.Errorf("generating bucket new root: %s", err)
		}
		if encKey != nil {
			if err = s.unpinNodeAndBranch(ctx, dirpth, encKey); err != nil {
				return nil, fmt.Errorf("unpinning pinned root: %s", err)
			}
		} else {
			if err = s.updateOrAddPin(ctx, buckPath, dirpth); err != nil {
				return nil, fmt.Errorf("updating pinned root: %s", err)
			}
		}
	} else {
		if encKey != nil {
			n, nodes, err := s.newDirFromExistingPath(ctx, remotePath, encKey, nil, "")
			if err != nil {
				return nil, fmt.Errorf("resolving remote path: %s", err)
			}
			dirpth, err = s.insertNodeAtPath(ctx, n, path.Join(buckPath, req.Path), encKey)
			if err != nil {
				return nil, fmt.Errorf("updating pinned root: %s", err)
			}
			if err := s.addAndPinNodes(ctx, nodes); err != nil {
				return nil, err
			}
		} else {
			dirpth, err = s.IPFSClient.Object().AddLink(ctx, buckPath, req.Path, remotePath, options.Object.Create(true))
			if err != nil {
				return nil, fmt.Errorf("adding folder: %s", err)
			}
			if err = s.updateOrAddPin(ctx, buckPath, dirpth); err != nil {
				return nil, fmt.Errorf("updating pinned root: %s", err)
			}
		}
	}

	buck.Path = dirpth.String()
	buck.UpdatedAt = time.Now().UnixNano()
	if err = s.Buckets.Save(ctx, dbID, buck, bucks.WithToken(dbToken)); err != nil {
		return nil, fmt.Errorf("saving new bucket state: %s", err)
	}

	return &pb.SetPathReply{}, nil
}

func (s *Service) List(ctx context.Context, _ *pb.ListRequest) (*pb.ListReply, error) {
	log.Debugf("received list request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	list, err := s.Buckets.List(ctx, dbID, bucks.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	roots := make([]*pb.Root, len(list))
	for i, buck := range list {
		roots[i] = &pb.Root{
			Key:       buck.Key,
			Name:      buck.Name,
			Path:      buck.Path,
			CreatedAt: buck.CreatedAt,
			UpdatedAt: buck.UpdatedAt,
		}
	}
	return &pb.ListReply{Roots: roots}, nil
}

func (s *Service) ListPath(ctx context.Context, req *pb.ListPathRequest) (*pb.ListPathReply, error) {
	log.Debugf("received list path request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	buck, pth, err := s.getBucketPath(ctx, dbID, req.Key, req.Path, dbToken)
	if err != nil {
		return nil, err
	}
	rep, err := s.pathToPb(ctx, buck, pth, true)
	if err != nil {
		return nil, err
	}
	if pth.String() == buck.Path {
		rep.Item.Name = buck.Name
	}
	return rep, nil
}

func (s *Service) ListIpfsPath(ctx context.Context, req *pb.ListIpfsPathRequest) (*pb.ListIpfsPathReply, error) {
	log.Debugf("received list ipfs path request")

	pth := path.New(req.Path)
	item, err := s.pathToItem(ctx, pth, true, nil)
	if err != nil {
		return nil, err
	}
	return &pb.ListIpfsPathReply{Item: item}, nil
}

// pathToItem returns items at path, optionally including one level down of links.
// If key is not nil, the items will be decrypted.
func (s *Service) pathToItem(ctx context.Context, pth path.Path, followLinks bool, key []byte) (*pb.ListPathItem, error) {
	rp, fp, err := util.ParsePath(pth)
	if err != nil {
		return nil, err
	}
	np, r, err := s.getNodesToPath(ctx, rp, fp, key)
	if err != nil {
		return nil, err
	}
	if r != "" {
		return nil, fmt.Errorf("could not resolve path: %s", pth)
	}
	n := np[len(np)-1]
	return s.nodeToItem(ctx, n.new, pth.String(), followLinks)
}

// getNodeAtPath returns the decrypted node at path.
func (s *Service) getNodeAtPath(ctx context.Context, pth path.Resolved, key []byte) (ipld.Node, error) {
	cn, err := s.IPFSClient.ResolveNode(ctx, pth)
	if err != nil {
		return nil, err
	}
	switch cn.(type) {
	case *dag.RawNode:
		return cn, nil // All raw nodes will be leaves
	case *dag.ProtoNode:
		if key == nil {
			return cn, nil
		}
		fn, err := unixfs.FSNodeFromBytes(cn.(*dag.ProtoNode).Data())
		if err != nil {
			return nil, err
		}
		if fn.Data() == nil {
			return cn, nil // This node is a raw file wrapper
		}
		plain, err := decryptData(fn.Data(), key)
		if err != nil {
			return nil, err
		}
		n, err := dag.DecodeProtobuf(plain)
		if err != nil {
			return dag.NewRawNode(plain), nil
		}
		n.SetCidBuilder(dag.V1CidPrefix())
		return n, nil
	default:
		return nil, fmt.Errorf("cannot get node with unhandled type")
	}
}

func decryptData(data, key []byte) ([]byte, error) {
	r, err := dcrypto.NewDecrypter(bytes.NewReader(data), key)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

func (s *Service) nodeToItem(ctx context.Context, node ipld.Node, pth string, followLinks bool) (*pb.ListPathItem, error) {
	stat, err := node.Stat()
	if err != nil {
		return nil, err
	}
	item := &pb.ListPathItem{
		Cid:  node.Cid().String(),
		Name: filepath.Base(pth),
		Path: pth,
		Size: int64(stat.CumulativeSize),
	}
	for _, l := range node.Links() {
		if l.Name == "" {
			break
		}
		item.IsDir = true
		i := &pb.ListPathItem{}
		if followLinks {
			p := filepath.Join(pth, l.Name)
			n, err := l.GetNode(ctx, s.IPFSClient.Dag())
			if err != nil {
				return nil, err
			}
			i, err = s.nodeToItem(ctx, n, p, false)
			if err != nil {
				return nil, err
			}
		}
		item.Items = append(item.Items, i)
	}
	return item, nil
}

func parsePath(pth string) (fpth string, err error) {
	if strings.Contains(pth, bucks.SeedName) {
		err = fmt.Errorf("paths containing %s are not allowed", bucks.SeedName)
		return
	}
	fpth = strings.TrimPrefix(pth, "/")
	return
}

func (s *Service) getBucketPath(ctx context.Context, dbID thread.ID, key, pth string, token thread.Token) (*bucks.Bucket, path.Path, error) {
	filePath := strings.TrimPrefix(pth, "/")
	buck, err := s.Buckets.Get(ctx, dbID, key, bucks.WithToken(token))
	if err != nil {
		return nil, nil, err
	}
	if buck == nil {
		return nil, nil, status.Error(codes.NotFound, "bucket not found")
	}
	npth, err := inflateFilePath(buck, filePath)
	return buck, npth, err
}

func inflateFilePath(buck *bucks.Bucket, filePath string) (path.Path, error) {
	npth := path.New(filepath.Join(buck.Path, filePath))
	if err := npth.IsValid(); err != nil {
		return nil, err
	}
	return npth, nil
}

func (s *Service) pathToPb(ctx context.Context, buck *bucks.Bucket, pth path.Path, followLinks bool) (*pb.ListPathReply, error) {
	item, err := s.pathToItem(ctx, pth, followLinks, buck.GetEncKey())
	if err != nil {
		return nil, err
	}
	return &pb.ListPathReply{
		Item: item,
		Root: &pb.Root{
			Key:       buck.Key,
			Name:      buck.Name,
			Path:      buck.Path,
			CreatedAt: buck.CreatedAt,
			UpdatedAt: buck.UpdatedAt,
		},
	}, nil
}

func (s *Service) PushPath(server pb.API_PushPathServer) error {
	log.Debugf("received push path request")

	dbID, ok := common.ThreadIDFromContext(server.Context())
	if !ok {
		return fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(server.Context())

	req, err := server.Recv()
	if err != nil {
		return err
	}
	var key, headerPath, root string
	switch payload := req.Payload.(type) {
	case *pb.PushPathRequest_Header_:
		key = payload.Header.Key
		headerPath = payload.Header.Path
		root = payload.Header.Root
	default:
		return fmt.Errorf("push bucket path header is required")
	}
	filePath, err := parsePath(headerPath)
	if err != nil {
		return err
	}
	buck, err := s.Buckets.Get(server.Context(), dbID, key, bucks.WithToken(dbToken))
	if err != nil {
		return err
	}
	if root != "" && root != buck.Path {
		return status.Error(codes.FailedPrecondition, bucks.ErrNonFastForward.Error())
	}
	encKey := buck.GetEncKey()

	sendEvent := func(event *pb.PushPathReply_Event) error {
		return server.Send(&pb.PushPathReply{
			Payload: &pb.PushPathReply_Event_{
				Event: event,
			},
		})
	}

	sendErr := func(err error) {
		if err2 := server.Send(&pb.PushPathReply{
			Payload: &pb.PushPathReply_Error{
				Error: err.Error(),
			},
		}); err2 != nil {
			log.Errorf("error sending error: %v (%v)", err, err2)
		}
	}

	reader, writer := io.Pipe()
	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		for {
			req, err := server.Recv()
			if err == io.EOF {
				_ = writer.Close()
				return
			} else if err != nil {
				sendErr(fmt.Errorf("error on receive: %v", err))
				_ = writer.CloseWithError(err)
				return
			}
			switch payload := req.Payload.(type) {
			case *pb.PushPathRequest_Chunk:
				if _, err := writer.Write(payload.Chunk); err != nil {
					sendErr(fmt.Errorf("error writing chunk: %v", err))
					return
				}
			default:
				sendErr(fmt.Errorf("invalid request"))
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
				if err := sendEvent(&pb.PushPathReply_Event{
					Name:  event.Name,
					Bytes: event.Bytes,
				}); err != nil {
					log.Errorf("error sending event: %v", err)
				}
			} else {
				chSize <- event.Size // Save size for use in the final response
			}
		}
	}()

	var r io.Reader
	if encKey != nil {
		r, err = dcrypto.NewEncrypter(reader, encKey)
		if err != nil {
			return err
		}
	} else {
		r = reader
	}

	pth, err := s.IPFSClient.Unixfs().Add(
		server.Context(),
		ipfsfiles.NewReaderFile(r),
		options.Unixfs.CidVersion(1),
		options.Unixfs.Pin(false),
		options.Unixfs.Progress(true),
		options.Unixfs.Events(eventCh))
	if err != nil {
		return err
	}
	fn, err := s.IPFSClient.ResolveNode(server.Context(), pth)
	if err != nil {
		return err
	}

	buckPath := path.New(buck.Path)
	var dirpth path.Resolved
	if encKey != nil {
		dirpth, err = s.insertNodeAtPath(server.Context(), fn, path.Join(buckPath, filePath), encKey)
		if err != nil {
			return err
		}
	} else {
		dirpth, err = s.IPFSClient.Object().AddLink(server.Context(), buckPath, filePath, pth, options.Object.Create(true))
		if err != nil {
			return err
		}
		if err = s.updateOrAddPin(server.Context(), buckPath, dirpth); err != nil {
			return err
		}
	}

	buck.Path = dirpth.String()
	buck.UpdatedAt = time.Now().UnixNano()
	if err = s.Buckets.Save(server.Context(), dbID, buck, bucks.WithToken(dbToken)); err != nil {
		return err
	}

	size := <-chSize
	if err = sendEvent(&pb.PushPathReply_Event{
		Path: pth.String(),
		Size: size,
		Root: &pb.Root{
			Key:       buck.Key,
			Name:      buck.Name,
			Path:      buck.Path,
			CreatedAt: buck.CreatedAt,
			UpdatedAt: buck.UpdatedAt,
		},
	}); err != nil {
		return err
	}

	go s.IPNSManager.Publish(dirpth, buck.Key)

	log.Debugf("pushed %s to bucket: %s", filePath, buck.Key)
	return nil
}

// insertNodeAtPath inserts a node at the location of path.
// Key will be required if the path is encrypted.
func (s *Service) insertNodeAtPath(ctx context.Context, child ipld.Node, pth path.Path, key []byte) (path.Resolved, error) {
	// The first step here is find a resolvable list of nodes that point to path.
	rp, fp, err := util.ParsePath(pth)
	if err != nil {
		return nil, err
	}
	fd, fn := filepath.Split(fp)
	fd = strings.TrimSuffix(fd, "/")
	np, r, err := s.getNodesToPath(ctx, rp, fd, key)
	if err != nil {
		return nil, err
	}
	r = filepath.Join(r, fn)

	// If the remaining path segment is not empty, we need to create each one
	// starting at the other end and walking back up to the deepest node
	// in the node path.
	parts := strings.Split(r, "/")
	news := make([]ipld.Node, len(parts)-1)
	cn := child
	for i := len(parts) - 1; i > 0; i-- {
		n, err := newDirWithNode(cn, parts[i], key)
		if err != nil {
			return nil, err
		}
		news[i-1] = cn
		cn = n
	}
	np = append(np, childNode{new: cn, name: parts[0], isLeaf: true})

	// Now, we have a full list of nodes to the insert location,
	// but the existing nodes need to be updated and re-encrypted.
	change := make([]ipld.Node, len(np))
	for i := len(np) - 1; i >= 0; i-- {
		change[i] = np[i].new
		if i > 0 {
			p, ok := np[i-1].new.(*dag.ProtoNode)
			if !ok {
				return nil, dag.ErrNotProtobuf
			}
			if np[i].isLeaf {
				xn, err := p.GetLinkedNode(ctx, s.IPFSClient.Dag(), np[i].name)
				if err != nil && !errors.Is(err, dag.ErrLinkNotFound) {
					return nil, err
				}
				if xn != nil {
					np[i].old = path.IpfsPath(xn.Cid())
					if err := p.RemoveNodeLink(np[i].name); err != nil {
						return nil, err
					}
				}
			} else {
				xl, err := p.GetNodeLink(np[i].name)
				if err != nil && !errors.Is(err, dag.ErrLinkNotFound) {
					return nil, err
				}
				if xl != nil {
					if err := p.RemoveNodeLink(np[i].name); err != nil {
						return nil, err
					}
				}
			}
			if err := p.AddNodeLink(np[i].name, np[i].new); err != nil {
				return nil, err
			}
			np[i-1].new, err = encryptNode(p, key)
			if err != nil {
				return nil, err
			}
		}
	}

	// Add all the _new_ nodes, which is the sum of the branch new ones
	// from the missing path segment, and the changed ones from
	// the existing path.
	allNews := append(news, change...)
	if err := s.IPFSClient.Dag().AddMany(ctx, allNews); err != nil {
		return nil, err
	}
	// Pin brand new nodes
	if err := s.IPFSClient.Dag().Pinning().AddMany(ctx, news); err != nil {
		return nil, err
	}
	// Update changed node pins
	for _, n := range np {
		if n.old != nil && n.isLeaf {
			if err := s.unpinBranch(ctx, n.old, key); err != nil {
				return nil, err
			}
		}
		if err = s.updateOrAddPin(ctx, n.old, path.IpfsPath(n.new.Cid())); err != nil {
			return nil, err
		}
	}
	return path.IpfsPath(np[0].new.Cid()), nil
}

// unpinBranch walks a the node at path, decrypting (if needed) and unpinning all nodes
func (s *Service) unpinBranch(ctx context.Context, p path.Resolved, key []byte) error {
	n, err := s.getNodeAtPath(ctx, p, key)
	if err != nil {
		return err
	}
	for _, l := range n.Links() {
		if l.Name == "" {
			continue // Data nodes will never be pinned directly
		}
		lp := path.IpfsPath(l.Cid)
		if err := s.IPFSClient.Pin().Rm(ctx, lp); err != nil {
			return err
		}
		if err = s.unpinBranch(ctx, lp, key); err != nil {
			return err
		}
	}
	return nil
}

type childNode struct {
	old    path.Resolved
	new    ipld.Node
	name   string
	isLeaf bool
}

// getNodesToPath returns a list of childNodes that point to the path,
// The remaining path segment that was not resolvable is also returned.
func (s *Service) getNodesToPath(ctx context.Context, base path.Resolved, pth string, key []byte) (nodes []childNode, remainder string, err error) {
	top, err := s.getNodeAtPath(ctx, base, key)
	if err != nil {
		return
	}
	nodes = append(nodes, childNode{old: base, new: top})
	remainder = pth
	if remainder == "" {
		return
	}
	parts := strings.Split(pth, "/")
	for i := 0; i < len(parts); i++ {
		l := getLink(top.Links(), parts[i])
		if l != nil {
			p := path.IpfsPath(l.Cid)
			top, err = s.getNodeAtPath(ctx, p, key)
			if err != nil {
				return
			}
			nodes = append(nodes, childNode{old: p, new: top, name: parts[i]})
			if i == len(parts)-1 {
				remainder = ""
			} else {
				remainder = strings.Join(parts[i+1:], "/")
			}
		} else {
			return nodes, remainder, nil
		}
	}
	return
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
func (s *Service) updateOrAddPin(ctx context.Context, from, to path.Path) error {
	if from == nil {
		if err := s.IPFSClient.Pin().Add(ctx, to); err != nil {
			return err
		}
	} else {
		if err := s.IPFSClient.Pin().Update(ctx, from, to); err != nil {
			if err.Error() == pinNotRecursiveMsg {
				if err = s.IPFSClient.Pin().Add(ctx, to); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	return nil
}

func (s *Service) PullPath(req *pb.PullPathRequest, server pb.API_PullPathServer) error {
	log.Debugf("received pull path request")

	dbID, ok := common.ThreadIDFromContext(server.Context())
	if !ok {
		return fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(server.Context())

	buck, pth, err := s.getBucketPath(server.Context(), dbID, req.Key, req.Path, dbToken)
	if err != nil {
		return err
	}
	encKey := buck.GetEncKey()
	buckPath, err := util.NewResolvedPath(buck.Path)
	if err != nil {
		return err
	}
	np, r, err := s.getNodesToPath(server.Context(), buckPath, req.Path, encKey)
	if err != nil {
		return err
	}
	if r != "" {
		return fmt.Errorf("could not resolve path: %s", pth)
	}
	fn := np[len(np)-1]
	fpth := path.IpfsPath(fn.new.Cid())

	node, err := s.IPFSClient.Unixfs().Get(server.Context(), fpth)
	if err != nil {
		return err
	}
	defer node.Close()
	file := ipfsfiles.ToFile(node)
	if file == nil {
		return fmt.Errorf("node is a directory")
	}

	var reader io.Reader
	if encKey != nil {
		r, err := dcrypto.NewDecrypter(file, encKey)
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
			if err := server.Send(&pb.PullPathReply{
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

func (s *Service) PullIpfsPath(req *pb.PullIpfsPathRequest, server pb.API_PullIpfsPathServer) error {
	log.Debugf("received ipfs pull path request")

	pth := path.New(req.Path)
	node, err := s.IPFSClient.Unixfs().Get(server.Context(), pth)
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
			if err := server.Send(&pb.PullIpfsPathReply{
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

func (s *Service) Remove(ctx context.Context, req *pb.RemoveRequest) (*pb.RemoveReply, error) {
	log.Debugf("received remove request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	buck, err := s.Buckets.Get(ctx, dbID, req.Key, bucks.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	encKey := buck.GetEncKey()
	buckPath, err := util.NewResolvedPath(buck.Path)
	if err != nil {
		return nil, err
	}
	if encKey != nil {
		if err = s.unpinNodeAndBranch(ctx, buckPath, encKey); err != nil {
			return nil, err
		}
	} else {
		if err = s.IPFSClient.Pin().Rm(ctx, buckPath); err != nil {
			return nil, err
		}
	}
	if err = s.Buckets.Delete(ctx, dbID, buck.Key, bucks.WithToken(dbToken)); err != nil {
		return nil, err
	}
	if err = s.IPNSManager.RemoveKey(ctx, buck.Key); err != nil {
		return nil, err
	}
	if buck.DNSRecord != "" && s.DNSManager != nil {
		if err = s.DNSManager.DeleteRecord(buck.DNSRecord); err != nil {
			return nil, err
		}
	}

	log.Debugf("removed bucket: %s", buck.Key)
	return &pb.RemoveReply{}, nil
}

func (s *Service) unpinNodeAndBranch(ctx context.Context, pth path.Resolved, key []byte) error {
	if err := s.unpinBranch(ctx, pth, key); err != nil {
		return err
	}
	return s.IPFSClient.Pin().Rm(ctx, pth)
}

func (s *Service) RemovePath(ctx context.Context, req *pb.RemovePathRequest) (*pb.RemovePathReply, error) {
	log.Debugf("received remove path request")

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	filePath, err := parsePath(req.Path)
	if err != nil {
		return nil, err
	}
	buck, err := s.Buckets.Get(ctx, dbID, req.Key, bucks.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	if req.Root != "" && req.Root != buck.Path {
		return nil, status.Error(codes.FailedPrecondition, bucks.ErrNonFastForward.Error())
	}

	encKey := buck.GetEncKey()
	buckPath := path.New(buck.Path)
	var dirpth path.Resolved
	if encKey != nil {
		dirpth, err = s.removeNodeAtPath(ctx, path.Join(path.New(buck.Path), filePath), encKey)
		if err != nil {
			return nil, err
		}
	} else {
		dirpth, err = s.IPFSClient.Object().RmLink(ctx, buckPath, filePath)
		if err != nil {
			return nil, err
		}
		if err = s.updateOrAddPin(ctx, buckPath, dirpth); err != nil {
			return nil, err
		}
	}

	buck.Path = dirpth.String()
	buck.UpdatedAt = time.Now().UnixNano()
	if err = s.Buckets.Save(ctx, dbID, buck, bucks.WithToken(dbToken)); err != nil {
		return nil, err
	}

	go s.IPNSManager.Publish(dirpth, buck.Key)

	log.Debugf("removed %s from bucket: %s", filePath, buck.Key)
	return &pb.RemovePathReply{
		Root: &pb.Root{
			Key:       buck.Key,
			Name:      buck.Name,
			Path:      buck.Path,
			CreatedAt: buck.CreatedAt,
			UpdatedAt: buck.UpdatedAt,
		},
	}, nil
}

// removeNodeAtPath removes node at the location of path.
// Key will be required if the path is encrypted.
func (s *Service) removeNodeAtPath(ctx context.Context, pth path.Path, key []byte) (path.Resolved, error) {
	// The first step here is find a resolvable list of nodes that point to path.
	rp, fp, err := util.ParsePath(pth)
	if err != nil {
		return nil, err
	}
	np, r, err := s.getNodesToPath(ctx, rp, fp, key)
	if err != nil {
		return nil, err
	}
	// If the remaining path segment is not empty, then we conclude that
	// the node cannot be resolved.
	if r != "" {
		return nil, fmt.Errorf("could not resolve path: %s", pth)
	}
	np[len(np)-1].isLeaf = true

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
				return nil, dag.ErrNotProtobuf
			}
			if err := p.RemoveNodeLink(np[i].name); err != nil {
				return nil, err
			}
			if !np[i].isLeaf {
				if err := p.AddNodeLink(np[i].name, np[i].new); err != nil {
					return nil, err
				}
			}
			np[i-1].new, err = encryptNode(p, key)
			if err != nil {
				return nil, err
			}
		}
	}

	// Add all the changed nodes
	if err := s.IPFSClient.Dag().AddMany(ctx, change); err != nil {
		return nil, err
	}
	// Update / remove node pins
	for _, n := range np {
		if n.isLeaf {
			if err = s.unpinNodeAndBranch(ctx, n.old, key); err != nil {
				return nil, err
			}
		} else {
			if err = s.updateOrAddPin(ctx, n.old, path.IpfsPath(n.new.Cid())); err != nil {
				return nil, err
			}
		}
	}
	return path.IpfsPath(np[0].new.Cid()), nil
}

func (s *Service) Archive(ctx context.Context, req *pb.ArchiveRequest) (*pb.ArchiveReply, error) {
	log.Debug("received archive request")

	if !s.Buckets.IsArchivingEnabled() {
		return nil, ErrArchivingFeatureDisabled
	}

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	buck, err := s.Buckets.Get(ctx, dbID, req.Key, bucks.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	p, err := util.NewResolvedPath(buck.Path)
	if err != nil {
		return nil, fmt.Errorf("parsing cid path: %s", err)
	}
	if err := s.Buckets.Archive(ctx, dbID, req.GetKey(), p.Cid(), bucks.WithToken(dbToken)); err != nil {
		return nil, fmt.Errorf("archiving bucket %s: %s", req.GetKey(), err)
	}
	log.Debug("archived bucket")
	return &pb.ArchiveReply{}, nil
}

func (s *Service) ArchiveWatch(req *pb.ArchiveWatchRequest, server pb.API_ArchiveWatchServer) error {
	log.Debug("received archive watch")

	if !s.Buckets.IsArchivingEnabled() {
		return ErrArchivingFeatureDisabled
	}

	var err error
	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()
	ch := make(chan string)
	go func() {
		err = s.Buckets.ArchiveWatch(ctx, req.GetKey(), ch)
		close(ch)
	}()
	for s := range ch {
		if err := server.Send(&pb.ArchiveWatchReply{Msg: s}); err != nil {
			return err
		}
	}
	if err != nil {
		return fmt.Errorf("watching cid logs: %s", err)
	}
	return nil
}

func (s *Service) ArchiveStatus(ctx context.Context, req *pb.ArchiveStatusRequest) (*pb.ArchiveStatusReply, error) {
	log.Debug("received archive status")

	if !s.Buckets.IsArchivingEnabled() {
		return nil, ErrArchivingFeatureDisabled
	}

	jstatus, failedMsg, err := s.Buckets.ArchiveStatus(ctx, req.Key)
	if err != nil {
		return nil, fmt.Errorf("getting status from last archive: %s", err)
	}
	var st pb.ArchiveStatusReply_Status
	switch jstatus {
	case ffs.Success:
		st = pb.ArchiveStatusReply_Done
	case ffs.Queued, ffs.Executing:
		st = pb.ArchiveStatusReply_Executing
	case ffs.Failed:
		st = pb.ArchiveStatusReply_Failed
	case ffs.Canceled:
		st = pb.ArchiveStatusReply_Canceled
	default:
		return nil, fmt.Errorf("unknown job status %d", jstatus)
	}

	log.Debug("finished archive status")
	return &pb.ArchiveStatusReply{
		Key:       req.Key,
		Status:    st,
		FailedMsg: failedMsg,
	}, nil
}

func (s *Service) ArchiveInfo(ctx context.Context, req *pb.ArchiveInfoRequest) (*pb.ArchiveInfoReply, error) {
	log.Debug("received archive info")

	if !s.Buckets.IsArchivingEnabled() {
		return nil, ErrArchivingFeatureDisabled
	}

	dbID, ok := common.ThreadIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	buck, err := s.Buckets.Get(ctx, dbID, req.Key, bucks.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	currentArchive := buck.Archives.Current
	if currentArchive.Cid == "" {
		return nil, bucks.ErrNoCurrentArchive
	}

	deals := make([]*pb.ArchiveInfoReply_Archive_Deal, len(currentArchive.Deals))
	for i, d := range currentArchive.Deals {
		deals[i] = &pb.ArchiveInfoReply_Archive_Deal{
			ProposalCid: d.ProposalCid,
			Miner:       d.Miner,
		}
	}
	log.Debug("finished archive info")
	return &pb.ArchiveInfoReply{
		Key: req.Key,
		Archive: &pb.ArchiveInfoReply_Archive{
			Cid:   currentArchive.Cid,
			Deals: deals,
		},
	}, nil
}

func (s *Service) getGatewayHost() (host string, ok bool) {
	parts := strings.SplitN(s.GatewayURL, "//", 2)
	if len(parts) > 1 {
		return parts[1], true
	}
	return
}
