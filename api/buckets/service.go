package buckets

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	ipfsfiles "github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	fc "github.com/textileio/filecoin/api/client"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/api"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/dns"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var log = logging.Logger("buckets")

const (
	// bucketSeedName is the name of the seed file used to ensure buckets are unique
	bucketSeedName = ".textilebucketseed"
	// chunkSize for get file requests.
	chunkSize = 1024 * 32
)

// Service is a gRPC service for buckets.
type Service struct {
	Buckets *Buckets

	IPFSClient     iface.CoreAPI
	FilecoinClient *fc.Client

	GatewayUrl string

	DNSManager *dns.Manager
}

func (s *Service) ListPath(ctx context.Context, req *pb.ListPathRequest) (*pb.ListPathReply, error) {
	log.Debugf("received list bucket path request")

	storeID, ok := api.StoreFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("store required")
	}

	req.Path = strings.TrimSuffix(req.Path, "/")
	if req.Path == "" { // List top-level buckets
		bucks, err := s.Buckets.List(ctx, storeID)
		if err != nil {
			return nil, err
		}
		items := make([]*pb.ListPathReply_Item, len(bucks))
		for i, buck := range bucks {
			items[i], err = s.pathToItem(ctx, path.New(buck.Path), false)
			if err != nil {
				return nil, err
			}
			items[i].Name = buck.Name
		}

		return &pb.ListPathReply{
			Item: &pb.ListPathReply_Item{
				IsDir: true,
				Items: items,
			},
		}, nil
	}

	buck, pth, err := s.getBucketPath(ctx, storeID, req.Path)
	if err != nil {
		return nil, err
	}
	return s.pathToPb(ctx, buck, pth, true)
}

func (s *Service) pathToItem(ctx context.Context, pth path.Path, followLinks bool) (*pb.ListPathReply_Item, error) {
	node, err := s.IPFSClient.Unixfs().Get(ctx, pth)
	if err != nil {
		return nil, err
	}
	defer node.Close()
	return nodeToItem(pth.String(), node, followLinks)
}

func nodeToItem(pth string, node ipfsfiles.Node, followLinks bool) (*pb.ListPathReply_Item, error) {
	size, err := node.Size()
	if err != nil {
		return nil, err
	}
	item := &pb.ListPathReply_Item{
		Name: filepath.Base(pth),
		Path: pth,
		Size: size,
	}
	switch node := node.(type) {
	case ipfsfiles.Directory:
		item.IsDir = true
		entries := node.Entries()
		for entries.Next() {
			if entries.Name() == bucketSeedName {
				continue
			}
			i := &pb.ListPathReply_Item{}
			if followLinks {
				n := entries.Node()
				i, err = nodeToItem(filepath.Join(pth, entries.Name()), n, false)
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

func parsePath(pth string) (buck, name string, err error) {
	if strings.Contains(pth, bucketSeedName) {
		err = fmt.Errorf("paths containing %s are not allowed", bucketSeedName)
		return
	}
	pth = strings.TrimPrefix(pth, "/")
	parts := strings.SplitN(pth, "/", 2)
	buck = parts[0]
	if len(parts) > 1 {
		name = parts[1]
	}
	return
}

func (s *Service) getBucketPath(ctx context.Context, storeID thread.ID, pth string) (*Bucket, path.Path, error) {
	buckName, filePath, err := parsePath(pth)
	if err != nil {
		return nil, nil, err
	}
	buck, err := s.Buckets.Get(ctx, storeID, buckName)
	if err != nil {
		return nil, nil, err
	}
	if buck == nil {
		return nil, nil, status.Error(codes.NotFound, "bucket not found")
	}
	npth, err := inflateFilePath(buck, filePath)
	return buck, npth, err
}

func inflateFilePath(buck *Bucket, filePath string) (path.Path, error) {
	npth := path.New(filepath.Join(buck.Path, filePath))
	if err := npth.IsValid(); err != nil {
		return nil, err
	}
	return npth, nil
}

func (s *Service) pathToPb(
	ctx context.Context, buck *Bucket, pth path.Path, followLinks bool) (*pb.ListPathReply, error) {
	item, err := s.pathToItem(ctx, pth, followLinks)
	if err != nil {
		return nil, err
	}
	return &pb.ListPathReply{
		Item: item,
		Root: &pb.Root{
			Name:      buck.Name,
			Path:      buck.Path,
			CreatedAt: buck.CreatedAt,
			UpdatedAt: buck.UpdatedAt,
		},
	}, nil
}

func (s *Service) PushPath(server pb.API_PushPathServer) error {
	log.Debugf("received push bucket path request")

	storeID, ok := api.StoreFromContext(server.Context())
	if !ok {
		return fmt.Errorf("store required")
	}

	req, err := server.Recv()
	if err != nil {
		return err
	}
	var headerPath string
	switch payload := req.Payload.(type) {
	case *pb.PushPathRequest_Header_:
		headerPath = payload.Header.Path
	default:
		return fmt.Errorf("push bucket path header is required")
	}
	buckName, filePath, err := parsePath(headerPath)
	if err != nil {
		return err
	}
	buck, err := s.Buckets.Get(server.Context(), storeID, buckName)
	if err != nil {
		return err
	}
	if buck == nil {
		buck, err = s.createBucket(server.Context(), storeID, buckName)
		if err != nil {
			return err
		}
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

	var size string
	eventCh := make(chan interface{})
	defer close(eventCh)
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
				size = event.Size // Save size for use in the final response
			}
		}
	}()

	pth, err := s.IPFSClient.Unixfs().Add(
		server.Context(),
		ipfsfiles.NewReaderFile(reader),
		options.Unixfs.Pin(false),
		options.Unixfs.Progress(true),
		options.Unixfs.Events(eventCh))
	if err != nil {
		return err
	}

	buckPath := path.New(buck.Path)
	dirpth, err := s.IPFSClient.Object().
		AddLink(server.Context(), buckPath, filePath, pth, options.Object.Create(true))
	if err != nil {
		return err
	}
	if err = s.IPFSClient.Pin().Update(server.Context(), buckPath, dirpth); err != nil {
		return err
	}

	buck.Path = dirpth.String()
	buck.UpdatedAt = time.Now().UnixNano()
	if err = s.Buckets.Save(server.Context(), storeID, buck); err != nil {
		return err
	}

	if err = sendEvent(&pb.PushPathReply_Event{
		Path: pth.String(),
		Size: size,
		Root: &pb.Root{
			Name:      buck.Name,
			Path:      buck.Path,
			CreatedAt: buck.CreatedAt,
			UpdatedAt: buck.UpdatedAt,
		},
	}); err != nil {
		return err
	}

	log.Debugf("pushed %s to bucket: %s", filePath, buck.Name)
	return nil
}

func (s *Service) createBucket(ctx context.Context, storeID thread.ID, name string) (*Bucket, error) {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}

	pth, err := s.IPFSClient.Unixfs().Add(
		ctx,
		ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
			bucketSeedName: ipfsfiles.NewBytesFile(seed),
		}),
		options.Unixfs.Pin(true))
	if err != nil {
		return nil, err
	}

	if s.DNSManager != nil {
		parts := strings.SplitN(s.GatewayUrl, "//", 2)
		if len(parts) > 1 {
			if _, err := s.DNSManager.NewCNAME(name, parts[1]); err != nil {
				return nil, err
			}
		}
	}

	return s.Buckets.Create(ctx, storeID, pth, name)
}

func (s *Service) PullPath(req *pb.PullPathRequest, server pb.API_PullPathServer) error {
	log.Debugf("received pull bucket path request")

	storeID, ok := api.StoreFromContext(server.Context())
	if !ok {
		return fmt.Errorf("store required")
	}

	_, pth, err := s.getBucketPath(server.Context(), storeID, req.Path)
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

func (s *Service) RemovePath(ctx context.Context, req *pb.RemovePathRequest) (*pb.RemovePathReply, error) {
	log.Debugf("received remove bucket path request")

	storeID, ok := api.StoreFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("store required")
	}

	buckName, filePath, err := parsePath(req.Path)
	if err != nil {
		return nil, err
	}
	buck, err := s.Buckets.Get(ctx, storeID, buckName)
	if err != nil {
		return nil, err
	}
	buckPath := path.New(buck.Path)

	var dirpth path.Resolved
	var linkCnt int
	if filePath != "" {
		dirpth, err = s.IPFSClient.Object().RmLink(ctx, buckPath, filePath)
		if err != nil {
			return nil, err
		}

		links, err := s.IPFSClient.Unixfs().Ls(ctx, dirpth)
		if err != nil {
			return nil, err
		}
		for range links {
			linkCnt++
		}
	}

	if linkCnt > 1 { // Account for the seed file
		if err = s.IPFSClient.Pin().Update(ctx, buckPath, dirpth); err != nil {
			return nil, err
		}

		buck.Path = dirpth.String()
		buck.UpdatedAt = time.Now().UnixNano()
		if err = s.Buckets.Save(ctx, storeID, buck); err != nil {
			return nil, err
		}
		log.Debugf("removed %s from bucket: %s", filePath, buck.Name)
	} else {
		if err = s.IPFSClient.Pin().Rm(ctx, buckPath); err != nil {
			return nil, err
		}

		if err = s.Buckets.Delete(ctx, storeID, buck.ID); err != nil {
			return nil, err
		}
		log.Debugf("removed bucket: %s", buck.Name)
	}

	return &pb.RemovePathReply{}, nil
}
