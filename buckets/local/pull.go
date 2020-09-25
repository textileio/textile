package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/textileio/textile/v2/api/buckets/client"
	"golang.org/x/sync/errgroup"
)

// PullRemote pulls remote files.
// By default, only missing files are pulled. See PathOption for more info.
func (b *Bucket) PullRemote(ctx context.Context, opts ...PathOption) (roots Roots, err error) {
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
	if args.confirm != nil && args.hard && len(diff) > 0 {
		if ok := args.confirm(diff); !ok {
			return roots, ErrAborted
		}
	}

	// Tmp move local modifications and additions if not pulling hard
	if !args.hard {
		for _, c := range diff {
			switch c.Type {
			case dagutils.Mod, dagutils.Add:
				if err := os.Rename(c.Name, c.Name+".buckpatch"); err != nil {
					return roots, err
				}
			}
		}
	}

	bp, err := b.Path()
	if err != nil {
		return
	}
	count, err := b.getPath(ctx, "", bp, diff, args.force, args.events)
	if err != nil {
		return
	}
	if count == 0 {
		return roots, ErrUpToDate
	}

	if err = b.repo.Save(ctx); err != nil {
		return
	}
	rc, err := b.getRemoteRoot(ctx)
	if err != nil {
		return
	}
	if err = b.repo.SetRemotePath("", rc); err != nil {
		return
	}

	// Re-apply local changes if not pulling hard
	if !args.hard {
		for _, c := range diff {
			switch c.Type {
			case dagutils.Mod, dagutils.Add:
				if err := os.Rename(c.Name+".buckpatch", c.Name); err != nil {
					return roots, err
				}
			case dagutils.Remove:
				// If the file was also deleted on the remote,
				// the local deletion will already have been handled by getPath.
				// So, we just ignore the error here.
				_ = os.RemoveAll(c.Name)
			}
		}
	}
	return b.Roots(ctx)
}

func (b *Bucket) getPath(ctx context.Context, pth, dest string, diff []Change, force bool, events chan<- PathEvent) (count int, err error) {
	key := b.Key()
	all, missing, err := b.listPath(ctx, key, pth, dest, force)
	if err != nil {
		return
	}
	count = len(missing)
	rm := make(map[string]string)
	list, err := b.walkPath(dest)
	if err != nil {
		return
	}
loop:
	for _, n := range list {
		for _, r := range all {
			if r.name == n {
				continue loop
			}
		}
		p := strings.TrimPrefix(n, dest+"/")
		rm[p] = n
	}
looop:
	for _, l := range diff {
		for _, r := range all {
			if strings.HasPrefix(r.path, l.Path) {
				continue looop
			}
		}
		if _, ok := rm[l.Path]; !ok {
			rm[l.Path] = l.Name
		}
	}
	count += len(rm)
	if count == 0 {
		return
	}

	if len(missing) > 0 {
		if events != nil {
			events <- PathEvent{
				Path: pth,
				Type: PathStart,
			}
		}
		eg, gctx := errgroup.WithContext(context.Background())
		for _, o := range missing {
			o := o
			eg.Go(func() error {
				if gctx.Err() != nil {
					return nil
				}
				if err := b.getFile(ctx, key, o, events); err != nil {
					return err
				}
				return b.repo.SetRemotePath(o.path, o.cid)
			})
		}
		if err := eg.Wait(); err != nil {
			return count, err
		}
	}
	if len(rm) > 0 {
		for _, r := range rm {
			// The file may have been modified locally, in which case it will have been moved to a patch.
			// So, we just ignore the error here.
			_ = os.RemoveAll(r)
			if events != nil {
				rel, err := filepath.Rel(b.cwd, r)
				if err != nil {
					return count, err
				}
				events <- PathEvent{
					Path: rel,
					Type: FileRemoved,
				}
			}
		}
	}
	if events != nil {
		events <- PathEvent{
			Path: pth,
			Type: PathComplete,
		}
	}
	return count, nil
}

type object struct {
	path string
	name string
	cid  cid.Cid
	size int64
}

func (b *Bucket) listPath(ctx context.Context, key, pth, dest string, force bool) (all, missing []object, err error) {
	rep, err := b.clients.Buckets.ListPath(ctx, key, pth)
	if err != nil {
		return
	}
	if rep.Item.IsDir {
		for _, i := range rep.Item.Items {
			a, m, err := b.listPath(ctx, key, filepath.Join(pth, filepath.Base(i.Path)), dest, force)
			if err != nil {
				return nil, nil, err
			}
			all = append(all, a...)
			missing = append(missing, m...)
		}
	} else {
		name := filepath.Join(dest, pth)
		c, err := cid.Decode(rep.Item.Cid)
		if err != nil {
			return nil, nil, err
		}
		o := object{path: pth, name: name, size: rep.Item.Size, cid: c}
		all = append(all, o)
		if !force {
			c, err := cid.Decode(rep.Item.Cid)
			if err != nil {
				return nil, nil, err
			}
			lc, err := b.repo.HashFile(name)
			if err == nil && lc.Equals(c) { // File exists, skip it
				return all, missing, nil
			} else {
				match, err := b.repo.MatchPath(pth, lc, c)
				if err != nil {
					if !errors.Is(err, ds.ErrNotFound) {
						return nil, nil, err
					}
				} else if match { // File exists, skip it
					return all, missing, nil
				}
			}
		}
		missing = append(missing, o)
	}
	return all, missing, nil
}

func (b *Bucket) getFile(ctx context.Context, key string, o object, events chan<- PathEvent) error {
	if err := os.MkdirAll(filepath.Dir(o.name), os.ModePerm); err != nil {
		return err
	}
	file, err := os.Create(o.name)
	if err != nil {
		return err
	}
	defer file.Close()

	rel, err := filepath.Rel(b.cwd, o.name)
	if err != nil {
		return err
	}

	if events != nil {
		events <- PathEvent{
			Path: rel,
			Cid:  o.cid,
			Type: FileStart,
			Size: o.size,
		}
	}

	progress := make(chan int64)
	go func() {
		for up := range progress {
			if events != nil {
				events <- PathEvent{
					Path:     rel,
					Cid:      o.cid,
					Type:     FileProgress,
					Size:     o.size,
					Progress: up,
				}
			}
		}
	}()
	if err := b.clients.Buckets.PullPath(ctx, key, o.path, file, client.WithProgress(progress)); err != nil {
		return err
	}
	if events != nil {
		events <- PathEvent{
			Path:     rel,
			Cid:      o.cid,
			Type:     FileComplete,
			Size:     o.size,
			Progress: o.size,
		}
	}
	return nil
}
