package client

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/textile/api"
	pb "github.com/textileio/textile/api/buckets/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
func NewClient(target string, creds credentials.TransportCredentials) (*Client, error) {
	var opts []grpc.DialOption
	auth := tokenAuth{}
	if creds != nil {
		opts = append(opts, grpc.WithTransportCredentials(creds))
		auth.secure = true
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	opts = append(opts, grpc.WithPerRPCCredentials(auth))
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
func (c *Client) ListPath(ctx context.Context, pth string, auth api.Auth) (*pb.ListPathReply, error) {
	return c.c.ListPath(authCtx(ctx, auth), &pb.ListPathRequest{
		Path: pth,
	})
}

// PushPathOptions defines options for pushing a bucket path.
type PushPathOptions struct {
	Progress chan<- int64
}

// PushPathOption specifies an option for pushing a bucket path.
type PushPathOption func(*PushPathOptions)

// WithPushProgress writes progress updates to the given channel.
func WithPushProgress(ch chan<- int64) PushPathOption {
	return func(args *PushPathOptions) {
		args.Progress = ch
	}
}

type pushPathResult struct {
	path path.Resolved
	root path.Path
	err  error
}

// PushPath pushes a file to a bucket path.
// The bucket and any directory paths will be created if they don't exist.
// This will return the resolved path and the bucket's new root path.
func (c *Client) PushPath(
	ctx context.Context, bucketPath string, reader io.Reader, auth api.Auth,
	opts ...PushPathOption) (result path.Resolved, root path.Path, err error) {

	args := &PushPathOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if args.Progress != nil {
		defer close(args.Progress)
	}

	stream, err := c.c.PushPath(authCtx(ctx, auth))
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
				} else if args.Progress != nil {
					args.Progress <- payload.Event.Bytes
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

// PullPathOptions defines options for pulling a bucket path.
type PullPathOptions struct {
	Progress chan<- int64
}

// PullPathOption specifies an option for pulling a bucket path.
type PullPathOption func(*PullPathOptions)

// WithPullProgress writes progress updates to the given channel.
func WithPullProgress(ch chan<- int64) PullPathOption {
	return func(args *PullPathOptions) {
		args.Progress = ch
	}
}

// PullPath pulls the bucket path, writing it to writer if it's a file.
func (c *Client) PullPath(
	ctx context.Context, bucketPath string, writer io.Writer, auth api.Auth, opts ...PullPathOption) error {

	args := &PullPathOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if args.Progress != nil {
		defer close(args.Progress)
	}

	stream, err := c.c.PullPath(authCtx(ctx, auth), &pb.PullPathRequest{
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
		if args.Progress != nil {
			args.Progress <- written
		}
	}
	return nil
}

// RemovePath removes the file or directory at path.
//  files and directories will be unpinned.
// If the resulting bucket is empty, it will also be removed.
func (c *Client) RemovePath(ctx context.Context, pth string, auth api.Auth) error {
	_, err := c.c.RemovePath(authCtx(ctx, auth), &pb.RemovePathRequest{
		Path: pth,
	})
	return err
}

type authKey string

func authCtx(ctx context.Context, auth api.Auth) context.Context {
	ctx = context.WithValue(ctx, authKey("token"), auth.Token)
	if auth.Org != "" {
		ctx = context.WithValue(ctx, authKey("org"), auth.Org)
	}
	return ctx
}

type tokenAuth struct {
	secure bool
}

func (t tokenAuth) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	token, ok := ctx.Value(authKey("token")).(string)
	if ok && token != "" {
		md["authorization"] = "bearer " + token
	}
	org, ok := ctx.Value(authKey("org")).(string)
	if ok && org != "" {
		md["x-org"] = org
	}
	return md, nil
}

func (t tokenAuth) RequireTransportSecurity() bool {
	return t.secure
}
