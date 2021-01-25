package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	du "github.com/ipfs/go-merkledag/dagutils"
	"github.com/textileio/textile/v2/api/bucketsd/client"
	"github.com/textileio/textile/v2/buckets"
	"golang.org/x/sync/errgroup"
)

// MaxPullConcurrency is the maximum number of files that can be pulled concurrently.
var MaxPullConcurrency = 10

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
	if errors.Is(err, ErrNotABucket) {
		args.force = true
	} else if err != nil {
		return
	}
	if args.confirm != nil && args.hard && len(diff) > 0 {
		if ok := args.confirm(diff); !ok {
			return roots, ErrAborted
		}
	}

	// Stash local modifications and additions if not pulling hard
	if !args.hard {
		if err := stashChanges(diff); err != nil {
			return roots, err
		}
	}

	bp, err := b.Path()
	if err != nil {
		return
	}
	changes, err := b.getPath(ctx, "", bp, diff, args.force, args.events)
	if err != nil {
		return
	}
	if changes == 0 {
		return roots, ErrUpToDate
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

	// Re-apply local changes if not pulling hard
	if !args.hard {
		if err := applyChanges(diff); err != nil {
			return roots, err
		}
	}
	return b.Roots(ctx)
}

func (b *Bucket) getPath(
	ctx context.Context,
	pth, dest string,
	diff []Change,
	force bool,
	events chan<- Event,
) (changes int, err error) {
	all, missing, err := b.listPath(ctx, pth, dest, force)
	if err != nil {
		return
	}
	remove := make(map[string]string)
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
		p := strings.TrimPrefix(n, dest+string(os.PathSeparator))
		remove[p] = n
	}
looop:
	for _, l := range diff {
		for _, r := range all {
			if strings.HasPrefix(r.path, l.Path) {
				continue looop
			}
		}
		if _, ok := remove[l.Path]; !ok {
			remove[l.Path] = l.Name
		}
	}

	return b.handleChanges(ctx, missing, remove, events)
}

func (b *Bucket) handleChanges(
	ctx context.Context,
	missing []object,
	remove map[string]string,
	events chan<- Event,
) (count int, err error) {
	count = len(missing)
	count += len(remove)
	if count == 0 {
		return
	}

	if len(missing) > 0 {
		progress := handleAllPullProgress(missing, events)
		defer close(progress)

		eg, gctx := errgroup.WithContext(context.Background())
		lim := make(chan struct{}, MaxPullConcurrency)
		for _, o := range missing {
			lim <- struct{}{}
			o := o
			eg.Go(func() error {
				defer func() { <-lim }()
				if gctx.Err() != nil {
					return nil
				}
				return b.getFile(ctx, b.Key(), o, events, progress)
			})
		}
		for i := 0; i < cap(lim); i++ {
			lim <- struct{}{}
		}
		if err := eg.Wait(); err != nil {
			return count, err
		}
	}
	if len(remove) > 0 {
		for p, n := range remove {
			// The file may have been modified locally, in which case it will have been moved to a patch.
			// So, we just ignore the error here.
			_ = os.RemoveAll(n)
			if events != nil {
				rel, err := filepath.Rel(b.cwd, n)
				if err != nil {
					return count, err
				}
				events <- Event{
					Type: EventFileRemoved,
					Path: rel,
				}
			}

			if b.repo != nil {
				if err := b.repo.RemovePath(ctx, p); err != nil {
					return count, err
				}
			}
		}
	}
	return count, nil
}

func (b *Bucket) diffPath(
	ctx context.Context,
	pth, dest string,
	ignoreDeletions bool,
) (diff []Change, missing []object, remove map[string]string, err error) {
	all, missing, err := b.listPath(ctx, pth, dest, false)
	if err != nil {
		return
	}
	remove = make(map[string]string)
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
		p := strings.TrimPrefix(n, dest+string(os.PathSeparator))
		r, err := filepath.Rel(b.cwd, n)
		if err != nil {
			return nil, nil, nil, err
		}
		diff = append(diff, Change{Type: du.Add, Name: n, Path: p, Rel: r})
		remove[p] = n
	}
	for _, o := range missing {
		if o.path == buckets.SeedName {
			continue
		}
		var ct du.ChangeType
		if _, err = os.Stat(o.name); err == nil {
			ct = du.Mod
		} else if os.IsNotExist(err) {
			if ignoreDeletions {
				continue
			}
			ct = du.Remove
		} else {
			return
		}
		r, err := filepath.Rel(b.cwd, o.name)
		if err != nil {
			return nil, nil, nil, err
		}
		diff = append(diff, Change{Type: ct, Name: o.name, Path: o.path, Rel: r})
	}
	return diff, missing, remove, nil
}

type object struct {
	path string
	name string
	cid  cid.Cid
	size int64
}

func (b *Bucket) listPath(
	ctx context.Context,
	pth, dest string,
	force bool,
) (all, missing []object, err error) {
	rep, err := b.clients.Buckets.ListPath(ctx, b.Key(), pth)
	if err != nil {
		return
	}
	if rep.Item.IsDir {
		for _, i := range rep.Item.Items {
			a, m, err := b.listPath(ctx, filepath.Join(pth, filepath.Base(i.Path)), dest, force)
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
		if !force && b.repo != nil {
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

func (b *Bucket) getFile(ctx context.Context, key string, o object, events chan<- Event, progress chan<- int64) error {
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

	prog, finish := handlePullProgress(progress, o.size)
	defer finish()
	if err := b.clients.Buckets.PullPath(ctx, key, o.path, file, client.WithProgress(prog)); err != nil {
		return err
	}

	if b.repo != nil {
		if err := b.repo.SetRemotePath(o.path, o.cid); err != nil {
			return err
		}
	}

	if events != nil {
		events <- Event{
			Type: EventFileComplete,
			Path: rel,
			Cid:  o.cid,
			Size: o.size,
		}
	}
	return nil
}

func handlePullProgress(global chan<- int64, fsize int64) (chan<- int64, func()) {
	progress := make(chan int64)
	var current int64
	var done bool
	var lk sync.Mutex
	go func() {
		for p := range progress {
			lk.Lock()
			if !done {
				diff := p - current
				global <- diff
				current = p
			}
			lk.Unlock()
		}
	}()
	doneFn := func() {
		lk.Lock()
		global <- fsize - current
		done = true
		close(progress)
		lk.Unlock()
	}
	return progress, doneFn
}

func handleAllPullProgress(os []object, events chan<- Event) chan<- int64 {
	var total int64
	for _, o := range os {
		total += o.size
	}
	progress := make(chan int64)
	go func() {
		var complete int64
		for p := range progress {
			complete += p
			if events != nil {
				events <- Event{
					Type:     EventProgress,
					Size:     total,
					Complete: complete,
				}
			}
		}
	}()
	return progress
}
