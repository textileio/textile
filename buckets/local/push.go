package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	du "github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/textile/v2/api/bucketsd/client"
)

// PushRemote pushes local files.
// By default, only staged changes are pushed. See PathOption for more info.
func (b *Bucket) PushLocal(ctx context.Context, opts ...PathOption) (roots Roots, err error) {
	b.Lock()
	defer b.Unlock()
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	args := &pathOptions{}
	for _, opt := range opts {
		opt(args)
	}

	diff, err := b.DiffLocal()
	if errors.Is(err, ErrNotABucket) {
		args.force = true
	} else if err != nil {
		return
	}
	bp, err := b.Path()
	if err != nil {
		return roots, err
	}
	if args.force { // Reset the diff to show all files as additions
		var reset []Change
		names, err := b.walkPath(bp)
		if err != nil {
			return roots, err
		}
		for _, n := range names {
			r, err := filepath.Rel(b.cwd, n)
			if err != nil {
				return roots, err
			}
			p := strings.TrimPrefix(n, bp+string(os.PathSeparator))
			reset = append(reset, Change{Type: du.Add, Name: n, Path: p, Rel: r})
		}
		// Add unique additions
	loop:
		for _, c := range reset {
			for _, x := range diff {
				if c.Path == x.Path {
					continue loop
				}
			}
			diff = append(diff, c)
		}
	}
	if len(diff) == 0 {
		return roots, ErrUpToDate
	}
	if args.confirm != nil {
		if ok := args.confirm(diff); !ok {
			return roots, ErrAborted
		}
	}

	r, err := b.Roots(ctx)
	if err != nil {
		return
	}
	xr := path.IpfsPath(r.Remote)
	var rm, add []Change
	if args.events != nil {
		args.events <- PathEvent{
			Path: bp,
			Type: PathStart,
		}
	}
	key := b.Key()
	for _, c := range diff {
		switch c.Type {
		case du.Mod, du.Add:
			add = append(add, c)
		case du.Remove:
			rm = append(rm, c)
		}
	}
	xr, err = b.addFiles(ctx, key, xr, add, args.force, args.events)
	if err != nil {
		return roots, err
	}
	if len(rm) > 0 {
		for _, c := range rm {
			var err error
			xr, err = b.rmFile(ctx, key, xr, c, args.force, args.events)
			if err != nil {
				return roots, err
			}
			if b.repo != nil {
				if err := b.repo.RemovePath(ctx, c.Path); err != nil {
					return roots, err
				}
			}
		}
	}
	if args.events != nil {
		args.events <- PathEvent{
			Path: bp,
			Type: PathComplete,
		}
	}

	if b.repo != nil {
		if err := b.repo.Save(ctx); err != nil {
			return roots, err
		}
		rc, err := b.getRemoteRoot(ctx)
		if err != nil {
			return roots, err
		}
		if err := b.repo.SetRemotePath("", rc); err != nil {
			return roots, err
		}
	}
	return b.Roots(ctx)
}

type pendingFile struct {
	path string
	rel  string
	r    io.ReadCloser
}

func (b *Bucket) addFiles(
	ctx context.Context,
	key string,
	xroot path.Resolved,
	changes []Change,
	force bool,
	events chan<- PathEvent,
) (path.Resolved, error) {
	progress := make(chan int64)
	files := make(map[string]pendingFile)

	opts := []client.Option{client.WithProgress(progress)}
	if !force {
		opts = append(opts, client.WithFastForwardOnly(xroot))
	}
	queue, err := b.clients.Buckets.PushPath(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	for _, c := range changes {
		f, err := os.Open(c.Name)
		if err != nil {
			return nil, err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, err
		}
		size := info.Size()

		file := pendingFile{
			path: c.Path,
			rel:  c.Rel,
			r:    f,
		}
		pth := filepath.ToSlash(c.Path)
		files[pth] = file

		queue.Push(file.path, f, size)
	}
	queue.Close()

	size := queue.Size()
	go func() {
		for up := range progress {
			var u int64
			if up > size {
				u = size
			} else {
				u = up
			}
			if events != nil {
				events <- PathEvent{
					Type:     FileProgress,
					Size:     size,
					Progress: u,
				}
			}
		}
	}()

	var root path.Resolved
	for queue.Next() {
		if queue.Err() != nil {
			return nil, queue.Err()
		}
		file := files[queue.Current.Path]

		if b.repo != nil {
			if err := b.repo.SetRemotePath(file.path, queue.Current.Cid); err != nil {
				return nil, err
			}
		}

		if events != nil {
			events <- PathEvent{
				Path: file.rel,
				Cid:  queue.Current.Cid,
				Type: FileComplete,
				Size: queue.Current.Size,
			}
		}

		root = queue.Current.Root
	}

	return root, nil
}

func (b *Bucket) rmFile(
	ctx context.Context,
	key string,
	xroot path.Resolved,
	c Change,
	force bool,
	events chan<- PathEvent,
) (path.Resolved, error) {
	var opts []client.Option
	if !force {
		opts = append(opts, client.WithFastForwardOnly(xroot))
	}
	root, err := b.clients.Buckets.RemovePath(ctx, key, c.Path, opts...)
	if err != nil {
		if !strings.HasSuffix(err.Error(), "no link by that name") {
			return nil, err
		}
	}
	if events != nil {
		events <- PathEvent{
			Path: c.Rel,
			Type: FileRemoved,
		}
	}
	return root, nil
}
