package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/textile/v2/api/bucketsd/client"
	"golang.org/x/sync/errgroup"
)

// AddRemoteCid stages the Unixfs dag at cid in the local bucket,
// optionally allowing the caller to select a merge strategy.
func (b *Bucket) AddRemoteCid(ctx context.Context, c cid.Cid, dest string, opts ...AddOption) error {
	b.Lock()
	defer b.Unlock()
	ctx, err := b.context(ctx)
	if err != nil {
		return err
	}
	args := &addOptions{}
	for _, opt := range opts {
		opt(args)
	}
	return b.mergeIpfsPath(ctx, path.IpfsPath(c), dest, args.merge, args.events)
}

func (b *Bucket) mergeIpfsPath(
	ctx context.Context,
	ipfsBasePth path.Path,
	dest string,
	merge SelectMergeFunc,
	events chan<- Event,
) error {
	ok, err := b.containsPath(dest)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("destination %s is not in bucket path", dest)
	}

	folderReplace, toAdd, err := b.listMergePath(ctx, ipfsBasePth, "", dest, merge)
	if err != nil {
		return err
	}

	// Remove all the folders that were decided to be replaced.
	for _, fr := range folderReplace {
		if err := os.RemoveAll(fr); err != nil {
			return err
		}
	}

	// Add files that are missing, or were decided to be overwritten.
	if len(toAdd) > 0 {
		progress := handleAllPullProgress(toAdd, events)
		defer close(progress)

		eg, gctx := errgroup.WithContext(ctx)
		for _, o := range toAdd {
			o := o
			eg.Go(func() error {
				if gctx.Err() != nil {
					return nil
				}
				if err := os.Remove(o.path); err != nil && !os.IsNotExist(err) {
					return err
				}
				trimmedDest := strings.TrimPrefix(o.path, dest)
				return b.getIpfsFile(
					gctx,
					path.Join(ipfsBasePth, trimmedDest),
					o.path,
					o.size,
					o.cid,
					events,
					progress,
				)
			})
		}
		if err := eg.Wait(); err != nil {
			return err
		}
	}
	return nil
}

// listMergePath walks the local bucket and the remote IPFS UnixFS DAG asking
// the client if wants to (replace, merge, ignore) matching folders, and if wants
// to (overwrite, ignore) matching files. Any non-matching files or folders in the
// IPFS UnixFS DAG will be added locally.
// The first return value is a slice of paths of folders that were decided to be
// replaced completely (not merged). The second return value are a list of files
// that should be added locally. If one of them exist, can be understood that should
// be overwritten.
func (b *Bucket) listMergePath(
	ctx context.Context,
	ipfsBasePth path.Path,
	ipfsRelPath, dest string,
	merge SelectMergeFunc,
) ([]string, []object, error) {
	// List remote IPFS UnixFS path level
	rep, err := b.clients.Buckets.ListIpfsPath(ctx, path.Join(ipfsBasePth, ipfsRelPath))
	if err != nil {
		return nil, nil, err
	}

	// If its a dir, ask if should be ignored, replaced, or merged.
	if rep.Item.IsDir {
		var replacedFolders []string
		var toAdd []object

		var folderExists bool

		localFolderPath := filepath.Join(dest, ipfsRelPath)
		if _, err := os.Stat(localFolderPath); err == nil {
			folderExists = true
		}

		if folderExists && merge != nil {
			ms, err := merge(fmt.Sprintf("Merge strategy for  %s", localFolderPath), true)
			if err != nil {
				return nil, nil, err
			}
			switch ms {
			case Skip:
				return nil, nil, nil
			case Merge:
				break
			case Replace:
				replacedFolders = append(replacedFolders, localFolderPath)
				merge = nil
			}
		}
		for _, i := range rep.Item.Items {
			nestFolderReplace, nestAdd, err := b.listMergePath(
				ctx,
				ipfsBasePth,
				filepath.Join(ipfsRelPath, i.Name),
				dest,
				merge,
			)
			if err != nil {
				return nil, nil, err
			}
			replacedFolders = append(replacedFolders, nestFolderReplace...)
			toAdd = append(toAdd, nestAdd...)
		}
		return replacedFolders, toAdd, nil
	}

	// If it's a file and it exists, confirm whether or not it should be overwritten.
	pth := filepath.Join(dest, ipfsRelPath)
	if _, err := os.Stat(pth); err == nil && merge != nil {
		ms, err := merge(fmt.Sprintf("Overwrite  %s", pth), false)
		if err != nil {
			return nil, nil, err
		}
		switch ms {
		case Skip:
			return nil, nil, nil
		case Merge:
			return nil, nil, fmt.Errorf("cannot merge files")
		case Replace:
			break
		}
	} else if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}

	c, err := cid.Decode(rep.Item.Cid)
	if err != nil {
		return nil, nil, err
	}
	o := object{path: pth, name: rep.Item.Name, size: rep.Item.Size, cid: c}
	return nil, []object{o}, nil
}

func (b *Bucket) getIpfsFile(
	ctx context.Context,
	ipfsPath path.Path,
	filePath string,
	size int64,
	c cid.Cid,
	events chan<- Event,
	progress chan<- int64,
) error {
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	prog, finish := handlePullProgress(progress, size)
	defer finish()
	if err := b.clients.Buckets.PullIpfsPath(ctx, ipfsPath, file, client.WithProgress(prog)); err != nil {
		return err
	}

	if events != nil {
		events <- Event{
			Type:     EventFileComplete,
			Path:     filePath,
			Cid:      c,
			Size:     size,
			Complete: size,
		}
	}

	return nil
}
