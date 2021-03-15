package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	assets "github.com/textileio/go-assets"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/textile/v2/api/bucketsd/client"
	bucketsclient "github.com/textileio/textile/v2/api/bucketsd/client"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/buckets"
	mdb "github.com/textileio/textile/v2/mongodb"
	tdb "github.com/textileio/textile/v2/threaddb"
	"github.com/textileio/textile/v2/util"
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

func (g *Gateway) renderBucket(c *gin.Context, ctx context.Context, threadID thread.ID, token thread.Token) {
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
		p := path.Join("thread", threadID.String(), buckets.CollectionName, r.Key)
		if token.Defined() {
			p += "?token=" + string(token)
		}
		links[i] = link{
			Name:  name,
			Path:  p,
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
	var buck tdb.Bucket
	if err := g.threads.FindByID(ctx, threadID, collection, id, &buck, db.WithTxnToken(token)); err != nil {
		render404(c)
		return
	}
	rep, err := g.buckets.ListPath(ctx, buck.Key, pth)
	if err != nil {
		render404(c)
		return
	}
	if !rep.Item.IsDir {
		contentType, r, err := getContentTypeFromPullPath(ctx, g.buckets, buck.Key, pth)
		if err != nil {
			render404(c)
			return
		}
		c.Writer.Header().Set("Content-Type", contentType)
		io.Copy(c.Writer, r)
	} else {
		var base string
		if g.subdomains {
			base = buckets.CollectionName
		} else {
			base = path.Join("thread", threadID.String(), buckets.CollectionName)
		}
		var links []link
		for _, item := range rep.Item.Items {
			pth := path.Join(base, strings.Replace(item.Path, rep.Root.Path, rep.Root.Key, 1))
			if token.Defined() {
				pth += "?token=" + string(token)
			}
			links = append(links, link{
				Name:  item.Name,
				Path:  pth,
				Size:  util.ByteCountDecimal(item.Size),
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
		if token.Defined() {
			back += "?token=" + string(token)
		}
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
	GetPathWithContentType(ctx context.Context, bucket, pth string) (string, io.Reader, error)
	ValidHost() string
}

type bucketFS struct {
	client  *client.Client
	keys    *mdb.IPNSKeys
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
			contentType, r, err := fs.GetPathWithContentType(ctx, key, c.Request.URL.Path)
			if err != nil {
				renderError(c, http.StatusInternalServerError, err)
			} else {
				c.Writer.Header().Set("Content-Type", contentType)
				io.Copy(c.Writer, r)
				c.Abort()
			}
		} else if target != "" {
			content := path.Join(c.Request.URL.Path, target)
			c.Writer.WriteHeader(http.StatusOK)
			contentType, r, err := fs.GetPathWithContentType(ctx, key, content)
			if err != nil {
				renderError(c, http.StatusInternalServerError, err)
			} else {
				c.Writer.Header().Set("Content-Type", contentType)
				io.Copy(c.Writer, r)
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

func (f *bucketFS) GetPathWithContentType(ctx context.Context, key, pth string) (string, io.Reader, error) {
	ctx = common.NewSessionContext(ctx, f.session)
	contentType, r, err := getContentTypeFromPullPath(ctx, f.client, key, pth)
	if err != nil {
		return "", nil, err
	}
	return contentType, r, nil
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

	buck := &tdb.Bucket{}
	if err := g.threads.FindByID(ctx, ipnskey.ThreadID, buckets.CollectionName, key, &buck, db.WithTxnToken(token)); err != nil {
		render404(c)
		return
	}
	rep, err := g.buckets.ListPath(ctx, buck.Key, "")
	if err != nil {
		render404(c)
		return
	}
	for _, item := range rep.Item.Items {
		if item.Name == "index.html" {
			c.Writer.WriteHeader(http.StatusOK)
			contentType, r, err := getContentTypeFromPullPath(ctx, g.buckets, buck.Key, item.Name)
			if err != nil {
				render404(c)
			}
			c.Writer.Header().Set("Content-Type", contentType)
			io.Copy(c.Writer, r)
			return
		}
	}
	renderError(c, http.StatusNotFound, fmt.Errorf("an index.html file was not found in this bucket"))
}

func getContentTypeFromPullPath(ctx context.Context, client *bucketsclient.Client, buckKey, pth string) (string, io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if err := client.PullPath(ctx, buckKey, pth, pw); err != nil {
			log.Errorf("pulling path: %s", err)
		}
	}()

	contentType, r, err := detectReaderContentType(pr)
	if err != nil {
		return "", nil, fmt.Errorf("detecting content-type: %s", err)
	}

	return contentType, r, nil
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
