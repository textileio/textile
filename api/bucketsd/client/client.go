package client

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	pb "github.com/textileio/textile/v2/api/bucketsd/pb"
	"github.com/textileio/textile/v2/buckets"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	// chunkSize for add file requests.
	chunkSize = 1024 * 32
)

var (
	// ErrPushPathQueueClosed indicates the push path is or was closed.
	ErrPushPathQueueClosed = errors.New("push path queue is closed")
)

// Client provides the client api.
type Client struct {
	c    pb.APIServiceClient
	conn *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		c:    pb.NewAPIServiceClient(conn),
		conn: conn,
	}, nil
}

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Create initializes a new bucket.
// The bucket name is only meant to help identify a bucket in a UI and is not unique.
func (c *Client) Create(ctx context.Context, opts ...CreateOption) (*pb.CreateResponse, error) {
	args := &createOptions{}
	for _, opt := range opts {
		opt(args)
	}
	var strCid string
	if args.fromCid.Defined() {
		strCid = args.fromCid.String()
	}
	return c.c.Create(ctx, &pb.CreateRequest{
		Name:         args.name,
		Private:      args.private,
		BootstrapCid: strCid,
		Unfreeze:     args.unfreeze,
	})
}

// Root returns the bucket root.
func (c *Client) Root(ctx context.Context, key string) (*pb.RootResponse, error) {
	return c.c.Root(ctx, &pb.RootRequest{
		Key: key,
	})
}

// Links returns a list of bucket path URL links.
func (c *Client) Links(ctx context.Context, key, pth string) (*pb.LinksResponse, error) {
	return c.c.Links(ctx, &pb.LinksRequest{
		Key:  key,
		Path: filepath.ToSlash(pth),
	})
}

// List returns a list of all bucket roots.
func (c *Client) List(ctx context.Context) (*pb.ListResponse, error) {
	return c.c.List(ctx, &pb.ListRequest{})
}

// ListIpfsPath returns items at a particular path in a UnixFS path living in the IPFS network.
func (c *Client) ListIpfsPath(ctx context.Context, pth path.Path) (*pb.ListIpfsPathResponse, error) {
	return c.c.ListIpfsPath(ctx, &pb.ListIpfsPathRequest{Path: pth.String()})
}

// ListPath returns information about a bucket path.
func (c *Client) ListPath(ctx context.Context, key, pth string) (*pb.ListPathResponse, error) {
	return c.c.ListPath(ctx, &pb.ListPathRequest{
		Key:  key,
		Path: filepath.ToSlash(pth),
	})
}

// SetPath set a particular path to an existing IPFS UnixFS DAG.
func (c *Client) SetPath(ctx context.Context, key, pth string, remoteCid cid.Cid) (*pb.SetPathResponse, error) {
	return c.c.SetPath(ctx, &pb.SetPathRequest{
		Key:  key,
		Path: filepath.ToSlash(pth),
		Cid:  remoteCid.String(),
	})
}

// MovePath moves a particular path to another path in the existing IPFS UnixFS DAG.
func (c *Client) MovePath(ctx context.Context, key, pth string, dest string) error {
	_, err := c.c.MovePath(ctx, &pb.MovePathRequest{
		Key:      key,
		FromPath: filepath.ToSlash(pth),
		ToPath:   filepath.ToSlash(dest),
	})
	return err
}

type pushPathResult struct {
	path path.Resolved
	root path.Resolved
	err  error
}

// PushPath pushes a file to a bucket path.
// This will return the resolved path and the bucket's new root path.
func (c *Client) PushPath(
	ctx context.Context,
	key, pth string,
	reader io.Reader,
	opts ...Option,
) (result path.Resolved, root path.Resolved, err error) {
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
				Path: filepath.ToSlash(pth),
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
			if rep.Event.Path != "" {
				id, err := cid.Parse(rep.Event.Path)
				if err != nil {
					waitCh <- pushPathResult{err: err}
					return
				}
				r, err := util.NewResolvedPath(rep.Event.Root.Path)
				if err != nil {
					waitCh <- pushPathResult{err: err}
					return
				}
				waitCh <- pushPathResult{
					path: path.IpfsPath(id),
					root: r,
				}
			} else if args.progress != nil {
				args.progress <- rep.Event.Bytes
			}
		}
	}()

	end := func(err error) error {
		if err := stream.CloseSend(); err != nil {
			return err
		}
		return err
	}

	buf := make([]byte, chunkSize)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&pb.PushPathRequest{
				Payload: &pb.PushPathRequest_Chunk{
					Chunk: buf[:n],
				},
			}); err == io.EOF {
				break
			} else if err != nil {
				return nil, nil, end(err)
			}
		} else if err == io.EOF {
			break
		} else if err != nil {
			return nil, nil, end(err)
		}
	}
	if err := end(nil); err != nil {
		return nil, nil, err
	}
	res := <-waitCh
	return res.path, res.root, res.err
}

// PushPathsResult contains the result of a Push.
type PushPathsResult struct {
	Path   string
	Cid    cid.Cid
	Size   int64
	Pinned int64
	Root   path.Resolved

	err error
}

// PushPathsQueue handles PushPath input and output.
type PushPathsQueue struct {
	// Current contains the current push result.
	Current PushPathsResult

	q           []pushPath
	len         int
	inCh        chan pushPath
	inWaitCh    chan struct{}
	outCh       chan PushPathsResult
	started     bool
	stopped     bool
	closeFunc   func() error
	closed      bool
	closeWaitCh chan struct{}

	size     int64
	complete int64

	lk sync.Mutex
	wg sync.WaitGroup
}

type pushPath struct {
	path string
	r    func() (io.ReadCloser, error)
}

// AddFile adds a file to the queue.
// pth is the location relative to the bucket root at which to insert the file, e.g., "/path/to/mybone.jpg".
// name is the location of the file on the local filesystem, e.g., "/Users/clyde/Downloads/mybone.jpg".
func (c *PushPathsQueue) AddFile(pth, name string) error {
	c.lk.Lock()
	defer c.lk.Unlock()

	if c.closed {
		return ErrPushPathQueueClosed
	}
	if c.started {
		return errors.New("cannot call AddFile after Next")
	}

	f, err := os.Open(name)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}

	atomic.AddInt64(&c.size, info.Size())
	f.Close()
	c.q = append(c.q, pushPath{
		path: filepath.ToSlash(pth),
		r: func() (io.ReadCloser, error) {
			return os.Open(name)
		},
	})
	return nil
}

// AddReader adds a reader to the queue.
// pth is the location relative to the bucket root at which to insert the file, e.g., "/path/to/mybone.jpg".
// r is the reader to read from.
// size is the size of the reader. Use of the WithProgress option is not recommended if the reader size is unknown.
func (c *PushPathsQueue) AddReader(pth string, r io.Reader, size int64) error {
	c.lk.Lock()
	defer c.lk.Unlock()

	if c.closed {
		return ErrPushPathQueueClosed
	}
	if c.started {
		return errors.New("cannot call AddReader after Next")
	}

	atomic.AddInt64(&c.size, size)
	c.q = append(c.q, pushPath{
		path: filepath.ToSlash(pth),
		r:    func() (io.ReadCloser, error) { return ioutil.NopCloser(r), nil },
	})
	return nil
}

// Size returns the queue size in bytes.
func (c *PushPathsQueue) Size() int64 {
	return atomic.LoadInt64(&c.size)
}

// Complete returns the portion of the queue size that has been pushed.
func (c *PushPathsQueue) Complete() int64 {
	return atomic.LoadInt64(&c.complete)
}

// Next blocks while the queue is open, returning true when a result is ready.
// Use Current to access the result.
func (c *PushPathsQueue) Next() (ok bool) {
	c.lk.Lock()
	if c.closed {
		c.lk.Unlock()
		return false
	}

	if !c.started {
		c.started = true
		c.len = len(c.q)
		c.start()
	}
	if c.len == 0 {
		c.lk.Unlock()
		return false
	}

	c.lk.Unlock()
	select {
	case r, k := <-c.outCh:
		if !k {
			return false
		}
		c.lk.Lock()
		c.len--
		c.Current = r
		c.lk.Unlock()
		return true
	}
}

func (c *PushPathsQueue) start() {
	go func() {
		defer close(c.inWaitCh)
		for {
			c.lk.Lock()
			if c.closed {
				c.q = nil
				c.lk.Unlock()
				return
			}
			if len(c.q) == 0 {
				c.lk.Unlock()
				return
			}
			var p pushPath
			p, c.q = c.q[0], c.q[1:]
			c.lk.Unlock()
			c.inCh <- p
		}

	}()
}

func (c *PushPathsQueue) stop() {
	c.lk.Lock()
	defer c.lk.Unlock()
	c.stopped = true
}

// Err returns the current queue error.
// Call this method before checking the value of Current.
func (c *PushPathsQueue) Err() error {
	c.lk.Lock()
	defer c.lk.Unlock()
	return c.Current.err
}

// Close the queue.
// Failure to close may lead to unpredictable bucket state.
func (c *PushPathsQueue) Close() error {
	c.lk.Lock()
	if c.closed {
		c.lk.Unlock()
		return nil
	}
	c.closed = true
	c.lk.Unlock()

	<-c.inWaitCh
	close(c.inCh)

	c.lk.Lock()
	wait := !c.stopped
	c.lk.Unlock()
	if err := c.closeFunc(); err != nil {
		return err
	}
	if wait {
		<-c.closeWaitCh
	}
	return nil
}

// PushPaths returns a queue that can be used to push multiple files and readers to bucket paths.
// See PushPathQueue.AddFile and PushPathsQueue.AddReader for more.
func (c *Client) PushPaths(ctx context.Context, key string, opts ...Option) (*PushPathsQueue, error) {
	args := &options{}
	for _, opt := range opts {
		opt(args)
	}

	stream, err := c.c.PushPaths(ctx)
	if err != nil {
		return nil, err
	}
	var xr string
	if args.root != nil {
		xr = args.root.String()
	}

	if err := stream.Send(&pb.PushPathsRequest{
		Payload: &pb.PushPathsRequest_Header_{
			Header: &pb.PushPathsRequest_Header{
				Key:  key,
				Root: xr,
			},
		},
	}); err != nil {
		return nil, err
	}

	q := &PushPathsQueue{
		inCh:     make(chan pushPath),
		inWaitCh: make(chan struct{}),
		outCh:    make(chan PushPathsResult),
		closeFunc: func() error {
			return stream.CloseSend()
		},
		closeWaitCh: make(chan struct{}),
	}

	go func() {
		defer q.stop()
		for {
			rep, err := stream.Recv()
			if err == io.EOF {
				select {
				case q.closeWaitCh <- struct{}{}:
				default:
				}
				return
			} else if err != nil {
				if strings.Contains(err.Error(), "STREAM_CLOSED") {
					err = ErrPushPathQueueClosed
				}
				q.outCh <- PushPathsResult{err: err}
				return
			}

			id, err := cid.Parse(rep.Cid)
			if err != nil {
				q.outCh <- PushPathsResult{err: err}
				return
			}
			root, err := util.NewResolvedPath(rep.Root.Path)
			if err != nil {
				q.outCh <- PushPathsResult{err: err}
				return
			}
			q.outCh <- PushPathsResult{
				Path:   rep.Path,
				Cid:    id,
				Size:   rep.Size,
				Pinned: rep.Pinned,
				Root:   root,
			}
		}
	}()

	sendChunk := func(c *pb.PushPathsRequest_Chunk) bool {
		q.lk.Lock()
		if q.closed {
			q.lk.Unlock()
			return false
		}
		q.lk.Unlock()

		if err := stream.Send(&pb.PushPathsRequest{
			Payload: &pb.PushPathsRequest_Chunk_{
				Chunk: c,
			},
		}); err == io.EOF {
			return false // error is waiting to be received with stream.Recv above
		} else if err != nil {
			q.outCh <- PushPathsResult{err: err}
			return false
		}
		atomic.AddInt64(&q.complete, int64(len(c.Data)))

		q.lk.Lock()
		if !q.closed && args.progress != nil {
			args.progress <- atomic.LoadInt64(&q.complete)
		}
		q.lk.Unlock()
		return true
	}

	go func() {
		for p := range q.inCh {
			r, err := p.r()
			if err != nil {
				q.outCh <- PushPathsResult{err: err}
				break
			}
			buf := make([]byte, chunkSize)
			for {
				n, err := r.Read(buf)
				c := &pb.PushPathsRequest_Chunk{
					Path: p.path,
				}
				if n > 0 {
					c.Data = make([]byte, n)
					copy(c.Data, buf[:n])
					if ok := sendChunk(c); !ok {
						break
					}
				} else if err == io.EOF {
					sendChunk(c)
					break
				} else if err != nil {
					q.outCh <- PushPathsResult{err: err}
					break
				}
			}
			r.Close()
		}
	}()

	return q, nil
}

// PullPath pulls the bucket path, writing it to writer if it's a file.
func (c *Client) PullPath(ctx context.Context, key, pth string, writer io.Writer, opts ...Option) error {
	args := &options{}
	for _, opt := range opts {
		opt(args)
	}

	pth = filepath.ToSlash(pth)
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
		Path: filepath.ToSlash(pth),
		Root: xr,
	})
	if err != nil {
		return nil, err
	}
	return util.NewResolvedPath(res.Root.Path)
}

// PushPathAccessRoles updates path access roles by merging the pushed roles with existing roles.
// roles is a map of string marshaled public keys to path roles. A non-nil error is returned
// if the map keys are not unmarshalable to public keys.
// To delete a role for a public key, set its value to buckets.None.
func (c *Client) PushPathAccessRoles(ctx context.Context, key, pth string, roles map[string]buckets.Role) error {
	pbroles, err := buckets.RolesToPb(roles)
	if err != nil {
		return err
	}
	_, err = c.c.PushPathAccessRoles(ctx, &pb.PushPathAccessRolesRequest{
		Key:   key,
		Path:  filepath.ToSlash(pth),
		Roles: pbroles,
	})
	return err
}

// PullPathAccessRoles returns access roles for a path.
func (c *Client) PullPathAccessRoles(ctx context.Context, key, pth string) (map[string]buckets.Role, error) {
	res, err := c.c.PullPathAccessRoles(ctx, &pb.PullPathAccessRolesRequest{
		Key:  key,
		Path: filepath.ToSlash(pth),
	})
	if err != nil {
		return nil, err
	}
	return buckets.RolesFromPb(res.Roles)
}

// DefaultArchiveConfig gets the default archive config for the specified Bucket.
func (c *Client) DefaultArchiveConfig(ctx context.Context, key string) (*pb.ArchiveConfig, error) {
	res, err := c.c.DefaultArchiveConfig(ctx, &pb.DefaultArchiveConfigRequest{Key: key})
	if err != nil {
		return nil, err
	}
	return res.ArchiveConfig, nil
}

// SetDefaultArchiveConfig sets the default archive config for the specified Bucket.
func (c *Client) SetDefaultArchiveConfig(ctx context.Context, key string, config *pb.ArchiveConfig) error {
	req := &pb.SetDefaultArchiveConfigRequest{
		Key:           key,
		ArchiveConfig: config,
	}
	_, err := c.c.SetDefaultArchiveConfig(ctx, req)
	return err
}

// Archive creates a Filecoin bucket archive via Powergate.
func (c *Client) Archive(ctx context.Context, key string, opts ...ArchiveOption) error {
	req := &pb.ArchiveRequest{
		Key: key,
	}
	for _, opt := range opts {
		opt(req)
	}
	_, err := c.c.Archive(ctx, req)
	return err
}

// Archives returns information about current and historical archives.
func (c *Client) Archives(ctx context.Context, key string) (*pb.ArchivesResponse, error) {
	return c.c.Archives(ctx, &pb.ArchivesRequest{Key: key})
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
