package local

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ipfs/go-cid"
	pb "github.com/textileio/textile/v2/api/bucketsd/pb"
)

var errEmptyItem = fmt.Errorf("item is empty")

// BucketItem describes an item (file/directory) in a bucket.
type BucketItem struct {
	Cid        cid.Cid      `json:"cid"`
	Name       string       `json:"name"`
	Path       string       `json:"path"`
	Size       int64        `json:"size"`
	IsDir      bool         `json:"is_dir"`
	Items      []BucketItem `json:"items"`
	ItemsCount int          `json:"items_count"`
}

// ListRemotePath returns a list of all bucket items under path.
func (b *Bucket) ListRemotePath(ctx context.Context, pth string) (items []BucketItem, err error) {
	pth = filepath.ToSlash(pth)
	if pth == "." || pth == "/" || pth == "./" {
		pth = ""
	}
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	rep, err := b.clients.Buckets.ListPath(ctx, b.Key(), pth)
	if err != nil {
		return
	}
	if len(rep.Item.Items) > 0 {
		items = make([]BucketItem, len(rep.Item.Items))
		for j, k := range rep.Item.Items {
			ii, err := pbItemToItem(k)
			if err != nil {
				return items, err
			}
			items[j] = ii
		}
	} else if !rep.Item.IsDir {
		items = make([]BucketItem, 1)
		item, err := pbItemToItem(rep.Item)
		if err != nil {
			return items, err
		}
		items[0] = item
	}
	return items, nil
}

func pbItemToItem(pi *pb.PathItem) (item BucketItem, err error) {
	if pi.Cid == "" {
		return item, errEmptyItem
	}
	c, err := cid.Decode(pi.Cid)
	if err != nil {
		return
	}
	items := make([]BucketItem, len(pi.Items))
	for j, k := range pi.Items {
		ii, err := pbItemToItem(k)
		if errors.Is(err, errEmptyItem) {
			items = nil
			break
		} else if err != nil {
			return item, err
		}
		items[j] = ii
	}
	return BucketItem{
		Cid:        c,
		Name:       pi.Name,
		Path:       pi.Path,
		Size:       pi.Size,
		IsDir:      pi.IsDir,
		Items:      items,
		ItemsCount: int(pi.ItemsCount),
	}, nil
}
