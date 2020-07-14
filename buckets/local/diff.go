package local

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/logrusorgru/aurora"
	"github.com/textileio/textile/buckets"
	"github.com/textileio/textile/cmd"
)

// Change describes a local bucket change.
type Change struct {
	Type dagutils.ChangeType
	Name string // Absolute file name
	Path string // File name relative to the bucket root
	Rel  string // File name relative to the bucket current working directory
}

// ChangeType returns a string representation of a change type.
func ChangeType(t dagutils.ChangeType) string {
	switch t {
	case dagutils.Mod:
		return "modified:"
	case dagutils.Add:
		return "new file:"
	case dagutils.Remove:
		return "deleted: "
	default:
		return ""
	}
}

// ChangeColor returns an appropriate color for the given change type.
func ChangeColor(t dagutils.ChangeType) func(arg interface{}) aurora.Value {
	switch t {
	case dagutils.Mod:
		return aurora.Yellow
	case dagutils.Add:
		return aurora.Green
	case dagutils.Remove:
		return aurora.Red
	default:
		return nil
	}
}

// DiffLocal returns a list of locally staged bucket file changes.
func (b *Bucket) DiffLocal() ([]Change, error) {
	bp, err := b.Path()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
	defer cancel()
	diff, err := b.repo.Diff(ctx, bp)
	if err != nil {
		return nil, err
	}
	var all []Change
	if len(diff) == 0 {
		return all, nil
	}
	for _, c := range diff {
		fp := filepath.Join(bp, c.Path)
		switch c.Type {
		case dagutils.Mod, dagutils.Add:
			names, err := b.walkPath(fp)
			if err != nil {
				return nil, err
			}
			for _, n := range names {
				p := strings.TrimPrefix(n, bp+"/")
				r, err := filepath.Rel(b.cwd, n)
				if err != nil {
					return nil, err
				}
				all = append(all, Change{Type: c.Type, Name: n, Path: p, Rel: r})
			}
		case dagutils.Remove:
			r, err := filepath.Rel(b.cwd, fp)
			if err != nil {
				return nil, err
			}
			all = append(all, Change{Type: c.Type, Name: fp, Path: c.Path, Rel: r})
		}
	}
	return all, nil
}

func (b *Bucket) walkPath(pth string) (names []string, err error) {
	err = filepath.Walk(pth, func(n string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			f := strings.TrimPrefix(n, pth+"/")
			if Ignore(n) || f == buckets.SeedName || strings.HasPrefix(f, b.conf.Dir) || strings.HasSuffix(f, patchExt) {
				return nil
			}
			names = append(names, n)
		}
		return nil
	})
	if err != nil {
		return
	}
	return names, nil
}
