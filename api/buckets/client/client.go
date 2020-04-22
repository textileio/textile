package client

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	pb "github.com/textileio/textile/api/buckets/pb"
	"google.golang.org/grpc"
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

// ListPath returns information about a bucket path.
func (c *Client) ListPath(ctx context.Context, pth string) (*pb.ListPathReply, error) {
	return c.c.ListPath(ctx, &pb.ListPathRequest{
		Path: pth,
	})
}

type pushPathResult struct {
	path path.Resolved
	root path.Path
	err  error
}

// PushPath pushes a file to a bucket path.
// The bucket and any directory paths will be created if they don't exist.
// This will return the resolved path and the bucket's new root path.
func (c *Client) PushPath(ctx context.Context, bucketPath string, reader io.Reader, opts ...Option) (
	result path.Resolved, root path.Path, err error) {
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
	if err = stream.Send(&pb.PushPathRequest{
		Payload: &pb.PushPathRequest_Header_{
			Header: &pb.PushPathRequest_Header{
				Path: bucketPath,
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
					waitCh <- pushPathResult{
						path: path.IpfsPath(id),
						root: path.New(payload.Event.Root.Path),
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
func (c *Client) PullPath(ctx context.Context, bucketPath string, writer io.Writer, opts ...Option) error {
	args := &options{}
	for _, opt := range opts {
		opt(args)
	}
	if args.progress != nil {
		defer close(args.progress)
	}

	stream, err := c.c.PullPath(ctx, &pb.PullPathRequest{
		Path: bucketPath,
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

// RemovePath removes the file or directory at path.
// Files and directories will be unpinned.
// If the resulting bucket is empty, it will also be removed.
func (c *Client) RemovePath(ctx context.Context, pth string) error {
	_, err := c.c.RemovePath(ctx, &pb.RemovePathRequest{
		Path: pth,
	})
	return err
}
