package local

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/textile/api/buckets/client"
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
	if err != nil {
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
			p := strings.TrimPrefix(n, bp+"/")
			reset = append(reset, Change{Type: dagutils.Add, Name: n, Path: p, Rel: r})
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
	var rm []Change
	if args.events != nil {
		args.events <- PathEvent{
			Path: bp,
			Type: PathStart,
		}
	}
	key := b.Key()
	for _, c := range diff {
		switch c.Type {
		case dagutils.Mod, dagutils.Add:
			var added path.Resolved
			var err error
			added, xr, err = b.addFile(ctx, key, xr, c, args.force, args.events)
			if err != nil {
				return roots, err
			}
			if err := b.repo.SetRemotePath(c.Path, added.Cid()); err != nil {
				return roots, err
			}
		case dagutils.Remove:
			rm = append(rm, c)
		}
	}
	if args.events != nil {
		args.events <- PathEvent{
			Path: bp,
			Type: PathComplete,
		}
	}
	if len(rm) > 0 {
		for _, c := range rm {
			var err error
			xr, err = b.rmFile(ctx, key, xr, c, args.force, args.events)
			if err != nil {
				return roots, err
			}
			if err := b.repo.RemovePath(ctx, c.Name); err != nil {
				return roots, err
			}
		}
	}

	if err = b.repo.Save(ctx); err != nil {
		return
	}
	rc, err := b.getRemoteRoot(ctx)
	if err != nil {
		return roots, err
	}
	if err = b.repo.SetRemotePath("", rc); err != nil {
		return
	}
	return b.Roots(ctx)
}

func (b *Bucket) addFile(ctx context.Context, key string, xroot path.Resolved, c Change, force bool, events chan<- PathEvent) (added path.Resolved, root path.Resolved, err error) {
	file, err := os.Open(c.Name)
	if err != nil {
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return
	}
	size := info.Size()

	if events != nil {
		events <- PathEvent{
			Path: c.Rel,
			Type: FileStart,
			Size: size,
		}
	}

	progress := make(chan int64)
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
					Path:     c.Rel,
					Type:     FileProgress,
					Size:     size,
					Progress: u,
				}
			}
		}
	}()

	opts := []client.Option{client.WithProgress(progress)}
	if !force {
		opts = append(opts, client.WithFastForwardOnly(xroot))
	}
	added, root, err = b.clients.Buckets.PushPath(ctx, key, c.Path, file, opts...)
	if err != nil {
		return
	} else if events != nil {
		events <- PathEvent{
			Path:     c.Rel,
			Cid:      added.Cid(),
			Type:     FileComplete,
			Size:     size,
			Progress: size,
		}
	}
	return added, root, nil
}

func (b *Bucket) rmFile(ctx context.Context, key string, xroot path.Resolved, c Change, force bool, events chan<- PathEvent) (path.Resolved, error) {
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
