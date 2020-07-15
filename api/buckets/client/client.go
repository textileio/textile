package client

import (
	"context"
	"fmt"
	"io"

	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	// chunkSize for add file requests.
	chunkSize = 1024
)

// Client provides the client api.
type Client struct {
	c    pb.APIClient
	conn *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		c:    pb.NewAPIClient(conn),
		conn: conn,
	}, nil
}

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Init initializes a new bucket.
// The bucket name is only meant to help identify a bucket in a UI and is not unique.
func (c *Client) Init(ctx context.Context, opts ...InitOption) (*pb.InitReply, error) {
	args := &initOptions{}
	for _, opt := range opts {
		opt(args)
	}
	var strCid string
	if args.fromCid.Defined() {
		strCid = args.fromCid.String()
	}
	return c.c.Init(ctx, &pb.InitRequest{
		Name:         args.name,
		Private:      args.private,
		BootstrapCid: strCid,
	})
}

// Root returns the bucket root.
func (c *Client) Root(ctx context.Context, key string) (*pb.RootReply, error) {
	return c.c.Root(ctx, &pb.RootRequest{
		Key: key,
	})
}

// Links returns a list of links that can be used to view the bucket.
func (c *Client) Links(ctx context.Context, key string) (*pb.LinksReply, error) {
	return c.c.Links(ctx, &pb.LinksRequest{
		Key: key,
	})
}

// List returns a list of all bucket roots.
func (c *Client) List(ctx context.Context) (*pb.ListReply, error) {
	return c.c.List(ctx, &pb.ListRequest{})
}

// ListIpfsPath returns items at a particular path in a UnixFS path living in the IPFS network.
func (c *Client) ListIpfsPath(ctx context.Context, pth path.Path) (*pb.ListIpfsPathReply, error) {
	return c.c.ListIpfsPath(ctx, &pb.ListIpfsPathRequest{Path: pth.String()})
}

// ListPath returns information about a bucket path.
func (c *Client) ListPath(ctx context.Context, key, pth string) (*pb.ListPathReply, error) {
	return c.c.ListPath(ctx, &pb.ListPathRequest{
		Key:  key,
		Path: pth,
	})
}

// SetPath set a particular path to an existing IPFS UnixFS DAG.
func (c *Client) SetPath(ctx context.Context, key, pth string, remoteCid cid.Cid) (*pb.SetPathReply, error) {
	return c.c.SetPath(ctx, &pb.SetPathRequest{
		Key:  key,
		Path: pth,
		Cid:  remoteCid.String(),
	})
}

type pushPathResult struct {
	path path.Resolved
	root path.Resolved
	err  error
}

// PushPath pushes a file to a bucket path.
// This will return the resolved path and the bucket's new root path.
func (c *Client) PushPath(ctx context.Context, key, pth string, reader io.Reader, opts ...Option) (result path.Resolved, root path.Resolved, err error) {
	args := &options{}
	for _, opt := range opts {
		opt(args)
	}
	if args.progress != nil {
		defer close(args.progress)
	}

	stream, err := c.c.PushPath(ctx)
	if err != nil {
		return nil, nil, err
	}
	var xr string
	if args.root != nil {
		xr = args.root.String()
	}
	if err = stream.Send(&pb.PushPathRequest{
		Payload: &pb.PushPathRequest_Header_{
			Header: &pb.PushPathRequest_Header{
				Key:  key,
				Path: pth,
				Root: xr,
			},
		},
	}); err != nil {
		return nil, nil, err
	}

	waitCh := make(chan pushPathResult)
	go func() {
		defer close(waitCh)
		for {
			rep, err := stream.Recv()
			if err == io.EOF {
				return
			} else if err != nil {
				waitCh <- pushPathResult{err: err}
				return
			}
			switch payload := rep.Payload.(type) {
			case *pb.PushPathReply_Event_:
				if payload.Event.Path != "" {
					id, err := cid.Parse(payload.Event.Path)
					if err != nil {
						waitCh <- pushPathResult{err: err}
						return
					}
					r, err := util.NewResolvedPath(payload.Event.Root.Path)
					if err != nil {
						waitCh <- pushPathResult{err: err}
						return
					}
					waitCh <- pushPathResult{
						path: path.IpfsPath(id),
						root: r,
					}
				} else if args.progress != nil {
					args.progress <- payload.Event.Bytes
				}
			case *pb.PushPathReply_Error:
				waitCh <- pushPathResult{err: fmt.Errorf(payload.Error)}
				return
			default:
				waitCh <- pushPathResult{err: fmt.Errorf("invalid reply")}
				return
			}
		}
	}()

	buf := make([]byte, chunkSize)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&pb.PushPathRequest{
				Payload: &pb.PushPathRequest_Chunk{
					Chunk: buf[:n],
				},
			}); err == io.EOF {
				var noOp interface{}
				return nil, nil, stream.RecvMsg(noOp)
			} else if err != nil {
				_ = stream.CloseSend()
				return nil, nil, err
			}
		}
		if err == io.EOF {
			break
		} else if err != nil {
			_ = stream.CloseSend()
			return nil, nil, err
		}
	}
	if err = stream.CloseSend(); err != nil {
		return nil, nil, err
	}
	res := <-waitCh
	return res.path, res.root, res.err
}

// PullPath pulls the bucket path, writing it to writer if it's a file.
func (c *Client) PullPath(ctx context.Context, key, pth string, writer io.Writer, opts ...Option) error {
	args := &options{}
	for _, opt := range opts {
		opt(args)
	}
	if args.progress != nil {
		defer close(args.progress)
	}

	stream, err := c.c.PullPath(ctx, &pb.PullPathRequest{
		Key:  key,
		Path: pth,
	})
	if err != nil {
		return err
	}

	var written int64
	for {
		rep, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		n, err := writer.Write(rep.Chunk)
		if err != nil {
			return err
		}
		written += int64(n)
		if args.progress != nil {
			args.progress <- written
		}
	}
	return nil
}

// PullIpfsPath pulls the path from a remote UnixFS dag, writing it to writer if it's a file.
func (c *Client) PullIpfsPath(ctx context.Context, pth path.Path, writer io.Writer, opts ...Option) error {
	args := &options{}
	for _, opt := range opts {
		opt(args)
	}
	if args.progress != nil {
		defer close(args.progress)
	}

	stream, err := c.c.PullIpfsPath(ctx, &pb.PullIpfsPathRequest{
		Path: pth.String(),
	})
	if err != nil {
		return err
	}

	var written int64
	for {
		rep, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		n, err := writer.Write(rep.Chunk)
		if err != nil {
			return err
		}
		written += int64(n)
		if args.progress != nil {
			args.progress <- written
		}
	}
	return nil
}

// Remove removes an entire bucket.
// Files and directories will be unpinned.
func (c *Client) Remove(ctx context.Context, key string) error {
	_, err := c.c.Remove(ctx, &pb.RemoveRequest{
		Key: key,
	})
	return err
}

// RemovePath removes the file or directory at path.
// Files and directories will be unpinned.
func (c *Client) RemovePath(ctx context.Context, key, pth string, opts ...Option) (path.Resolved, error) {
	args := &options{}
	for _, opt := range opts {
		opt(args)
	}
	var xr string
	if args.root != nil {
		xr = args.root.String()
	}
	res, err := c.c.RemovePath(ctx, &pb.RemovePathRequest{
		Key:  key,
		Path: pth,
		Root: xr,
	})
	if err != nil {
		return nil, err
	}
	return util.NewResolvedPath(res.Root.Path)
}

// Archive creates a Filecoin bucket archive via Powergate.
func (c *Client) Archive(ctx context.Context, key string) (*pb.ArchiveReply, error) {
	return c.c.Archive(ctx, &pb.ArchiveRequest{
		Key: key,
	})
}

// ArchiveStatus returns the status of a Filecoin bucket archive.
func (c *Client) ArchiveStatus(ctx context.Context, key string) (*pb.ArchiveStatusReply, error) {
	return c.c.ArchiveStatus(ctx, &pb.ArchiveStatusRequest{
		Key: key,
	})
}

// ArchiveWatch watches status events from a Filecoin bucket archive.
func (c *Client) ArchiveWatch(ctx context.Context, key string, ch chan<- string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := c.c.ArchiveWatch(ctx, &pb.ArchiveWatchRequest{Key: key})
	if err != nil {
		return err
	}
	for {
		reply, err := stream.Recv()
		if err == io.EOF || status.Code(err) == codes.Canceled {
			break
		}
		if err != nil {
			return err
		}
		ch <- reply.Msg
	}
	return nil
}

// ArchiveInfo returns info about a Filecoin bucket archive.
func (c *Client) ArchiveInfo(ctx context.Context, key string) (*pb.ArchiveInfoReply, error) {
	return c.c.ArchiveInfo(ctx, &pb.ArchiveInfoRequest{
		Key: key,
	})
}
