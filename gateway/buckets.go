package gateway

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	assets "github.com/textileio/go-assets"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/buckets"
	bc "github.com/textileio/textile/buckets/collection"
	"github.com/textileio/textile/collections"
)

type fileSystem struct {
	*assets.FileSystem
}

func (f *fileSystem) Exists(prefix, path string) bool {
	pth := strings.TrimPrefix(path, prefix)
	if pth == "/" {
		return false
	}
	_, ok := f.Files[pth]
	return ok
}

func (g *Gateway) renderBucket(c *gin.Context, ctx context.Context, threadID thread.ID) {
	rep, err := g.buckets.List(ctx)
	if err != nil {
		renderError(c, http.StatusBadRequest, err)
		return
	}
	links := make([]link, len(rep.Roots))
	for i, r := range rep.Roots {
		var name string
		if r.Name != "" {
			name = r.Name
		} else {
			name = r.Key
		}
		links[i] = link{
			Name:  name,
			Path:  path.Join("thread", threadID.String(), buckets.CollectionName, r.Key),
			Size:  "",
			Links: "",
		}
	}
	c.HTML(http.StatusOK, "/public/html/unixfs.gohtml", gin.H{
		"Title":   "Index of " + path.Join("/thread", threadID.String(), buckets.CollectionName),
		"Root":    "/",
		"Path":    "",
		"Updated": "",
		"Back":    "",
		"Links":   links,
	})
}

func (g *Gateway) renderBucketPath(c *gin.Context, ctx context.Context, threadID thread.ID, collection, id, pth string, token thread.Token) {
	var buck bc.Bucket
	if err := g.threads.FindByID(ctx, threadID, collection, id, &buck, db.WithTxnToken(token)); err != nil {
		render404(c)
		return
	}
	// @todo: Remove this private bucket handling when the thread ACL is done.
	if buck.GetEncKey() != nil {
		render404(c)
		return
	}
	rep, err := g.buckets.ListPath(ctx, buck.Key, pth)
	if err != nil {
		render404(c)
		return
	}
	if !rep.Item.IsDir {
		if err := g.buckets.PullPath(ctx, buck.Key, pth, c.Writer); err != nil {
			renderError(c, http.StatusInternalServerError, err)
		}
	} else {
		var base string
		if g.subdomains {
			base = buckets.CollectionName
		} else {
			base = path.Join("thread", threadID.String(), buckets.CollectionName)
		}
		var links []link
		for _, item := range rep.Item.Items {
			pth := strings.Replace(item.Path, rep.Root.Path, rep.Root.Key, 1)
			links = append(links, link{
				Name:  item.Name,
				Path:  path.Join(base, pth),
				Size:  byteCountDecimal(item.Size),
				Links: strconv.Itoa(len(item.Items)),
			})
		}
		var name string
		if rep.Root.Name != "" {
			name = rep.Root.Name
		} else {
			name = rep.Root.Key
		}
		root := strings.Replace(rep.Item.Path, rep.Root.Path, name, 1)
		back := path.Dir(path.Join(base, strings.Replace(rep.Item.Path, rep.Root.Path, rep.Root.Key, 1)))
		c.HTML(http.StatusOK, "/public/html/unixfs.gohtml", gin.H{
			"Title":   "Index of /" + root,
			"Root":    "/" + root,
			"Path":    rep.Item.Path,
			"Updated": time.Unix(0, rep.Root.UpdatedAt).String(),
			"Back":    back,
			"Links":   links,
		})
	}
}

type serveBucketFS interface {
	GetThread(ctx context.Context, key string) (thread.ID, error)
	Exists(ctx context.Context, bucket, pth string) (bool, string)
	Write(ctx context.Context, bucket, pth string, writer io.Writer) error
	ValidHost() string
}

type bucketFS struct {
	client  *client.Client
	keys    *collections.IPNSKeys
	session string
	host    string
}

func serveBucket(fs serveBucketFS) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, err := bucketFromHost(c.Request.Host, fs.ValidHost())
		if err != nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
		defer cancel()
		threadID, err := fs.GetThread(ctx, key)
		if err != nil {
			return
		}
		ctx = common.NewThreadIDContext(ctx, threadID)
		token := thread.Token(c.Query("token"))
		if token.Defined() {
			ctx = thread.NewTokenContext(ctx, token)
		}

		exists, target := fs.Exists(ctx, key, c.Request.URL.Path)
		if exists {
			c.Writer.WriteHeader(http.StatusOK)
			ctype := mime.TypeByExtension(filepath.Ext(c.Request.URL.Path))
			if ctype == "" {
				ctype = "application/octet-stream"
			}
			c.Writer.Header().Set("Content-Type", ctype)
			if err := fs.Write(ctx, key, c.Request.URL.Path, c.Writer); err != nil {
				renderError(c, http.StatusInternalServerError, err)
			} else {
				c.Abort()
			}
		} else if target != "" {
			content := path.Join(c.Request.URL.Path, target)
			ctype := mime.TypeByExtension(filepath.Ext(content))
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Header().Set("Content-Type", ctype)
			if err := fs.Write(ctx, key, content, c.Writer); err != nil {
				renderError(c, http.StatusInternalServerError, err)
			} else {
				c.Abort()
			}
		}
	}
}

func (f *bucketFS) GetThread(ctx context.Context, bkey string) (id thread.ID, err error) {
	key, err := f.keys.GetByCid(ctx, bkey)
	if err != nil {
		return
	}
	return key.ThreadID, nil
}

func (f *bucketFS) Exists(ctx context.Context, key, pth string) (ok bool, name string) {
	if key == "" || pth == "/" {
		return
	}
	ctx = common.NewSessionContext(ctx, f.session)
	rep, err := f.client.ListPath(ctx, key, pth)
	if err != nil {
		return
	}
	if rep.Item.IsDir {
		for _, item := range rep.Item.Items {
			if item.Name == "index.html" {
				return false, item.Name
			}
		}
		return
	}
	return true, ""
}

func (f *bucketFS) Write(ctx context.Context, key, pth string, writer io.Writer) error {
	ctx = common.NewSessionContext(ctx, f.session)
	return f.client.PullPath(ctx, key, pth, writer)
}

func (f *bucketFS) ValidHost() string {
	return f.host
}

// renderWWWBucket renders a bucket as a website.
func (g *Gateway) renderWWWBucket(c *gin.Context, key string) {
	ctx, cancel := context.WithTimeout(common.NewSessionContext(context.Background(), g.apiSession), handlerTimeout)
	defer cancel()
	ipnskey, err := g.collections.IPNSKeys.GetByCid(ctx, key)
	if err != nil {
		render404(c)
		return
	}
	ctx = common.NewThreadIDContext(ctx, ipnskey.ThreadID)
	token := thread.Token(c.Query("token"))
	if token.Defined() {
		ctx = thread.NewTokenContext(ctx, token)
	}

	buck := &bc.Bucket{}
	if err := g.threads.FindByID(ctx, ipnskey.ThreadID, buckets.CollectionName, key, &buck, db.WithTxnToken(token)); err != nil {
		render404(c)
		return
	}
	// @todo: Remove this private bucket handling when the thread ACL is done.
	if buck.GetEncKey() != nil {
		render404(c)
		return
	}
	rep, err := g.buckets.ListPath(ctx, buck.Key, "")
	if err != nil {
		renderError(c, http.StatusInternalServerError, err)
		return
	}
	for _, item := range rep.Item.Items {
		if item.Name == "index.html" {
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Header().Set("Content-Type", "text/html")
			if err := g.buckets.PullPath(ctx, buck.Key, item.Name, c.Writer); err != nil {
				renderError(c, http.StatusInternalServerError, err)
			}
			return
		}
	}
	renderError(c, http.StatusNotFound, fmt.Errorf("an index.html file was not found in this bucket"))
}

func bucketFromHost(host, valid string) (key string, err error) {
	parts := strings.SplitN(host, ".", 2)
	hostport := parts[len(parts)-1]
	hostparts := strings.SplitN(hostport, ":", 2)
	if hostparts[0] != valid || valid == "" {
		err = fmt.Errorf("invalid bucket host")
		return
	}
	return parts[0], nil
}
