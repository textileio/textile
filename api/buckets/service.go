package buckets

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	ipfsfiles "github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
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

	buck, seed, err := s.createBucket(ctx, dbID, dbToken, req.Name)
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
		Seed:  seed,
	}, nil
}

func (s *Service) createBucket(ctx context.Context, dbID thread.ID, dbToken thread.Token, name string) (*bucks.Bucket, []byte, error) {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil, nil, err
	}
	pth, err := s.IPFSClient.Unixfs().Add(
		ctx,
		ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
			bucks.SeedName: ipfsfiles.NewBytesFile(seed),
		}),
		options.Unixfs.CidVersion(1),
		options.Unixfs.Pin(true))
	if err != nil {
		return nil, nil, err
	}

	bkey, err := s.IPNSManager.CreateKey(ctx, dbID)
	if err != nil {
		return nil, nil, err
	}
	buck, err := s.Buckets.Create(ctx, dbID, bkey, name, pth, bucks.WithToken(dbToken))
	if err != nil {
		return nil, nil, err
	}
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
	go s.IPNSManager.Publish(pth, buck.Key)
	return buck, seed, nil
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

func (s *Service) pathToItem(ctx context.Context, pth path.Path, followLinks bool) (*pb.ListPathReply_Item, error) {
	rpth, err := s.IPFSClient.ResolvePath(ctx, pth)
	if err != nil {
		return nil, err
	}
	node, err := s.IPFSClient.Unixfs().Get(ctx, rpth)
	if err != nil {
		return nil, err
	}
	defer node.Close()
	return s.nodeToItem(ctx, rpth.Cid(), pth.String(), node, followLinks)
}

func (s *Service) nodeToItem(ctx context.Context, c cid.Cid, pth string, node ipfsfiles.Node, followLinks bool) (*pb.ListPathReply_Item, error) {
	size, err := node.Size()
	if err != nil {
		return nil, err
	}
	item := &pb.ListPathReply_Item{
		Cid:  c.String(),
		Name: filepath.Base(pth),
		Path: pth,
		Size: size,
	}
	switch node := node.(type) {
	case ipfsfiles.Directory:
		item.IsDir = true
		entries := node.Entries()
		for entries.Next() {
			i := &pb.ListPathReply_Item{}
			if followLinks {
				n := entries.Node()
				p := filepath.Join(pth, entries.Name())
				r, err := s.IPFSClient.ResolvePath(ctx, path.New(p))
				if err != nil {
					return nil, err
				}
				i, err = s.nodeToItem(ctx, r.Cid(), p, n, false)
				if err != nil {
					n.Close()
					return nil, err
				}
				n.Close()
			}
			item.Items = append(item.Items, i)
		}
		if err := entries.Err(); err != nil {
			return nil, err
		}
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
	item, err := s.pathToItem(ctx, pth, followLinks)
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

	pth, err := s.IPFSClient.Unixfs().Add(
		server.Context(),
		ipfsfiles.NewReaderFile(reader),
		options.Unixfs.CidVersion(1),
		options.Unixfs.Pin(false),
		options.Unixfs.Progress(true),
		options.Unixfs.Events(eventCh))
	if err != nil {
		return err
	}

	buckPath := path.New(buck.Path)
	dirpth, err := s.IPFSClient.Object().AddLink(server.Context(), buckPath, filePath, pth, options.Object.Create(true))
	if err != nil {
		return err
	}
	if err = s.IPFSClient.Pin().Update(server.Context(), buckPath, dirpth); err != nil {
		if err.Error() == pinNotRecursiveMsg {
			if err = s.IPFSClient.Pin().Add(server.Context(), dirpth); err != nil {
				return err
			}
		} else {
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

func (s *Service) PullPath(req *pb.PullPathRequest, server pb.API_PullPathServer) error {
	log.Debugf("received pull path request")

	dbID, ok := common.ThreadIDFromContext(server.Context())
	if !ok {
		return fmt.Errorf("db required")
	}
	dbToken, _ := thread.TokenFromContext(server.Context())

	_, pth, err := s.getBucketPath(server.Context(), dbID, req.Key, req.Path, dbToken)
	if err != nil {
		return err
	}
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
	if err = s.IPFSClient.Pin().Rm(ctx, path.New(buck.Path)); err != nil {
		return nil, err
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

	buckPath := path.New(buck.Path)
	dirpth, err := s.IPFSClient.Object().RmLink(ctx, buckPath, filePath)
	if err != nil {
		return nil, err
	}
	if err = s.IPFSClient.Pin().Update(ctx, buckPath, dirpth); err != nil {
		if err.Error() == pinNotRecursiveMsg {
			if err = s.IPFSClient.Pin().Add(ctx, dirpth); err != nil {
				return nil, err
			}
		} else {
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
