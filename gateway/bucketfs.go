package gateway

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/collections"
)

type serveBucketFileSystem interface {
	GetThread(ctx context.Context, key string) (thread.ID, error)
	Exists(ctx context.Context, bucket, pth string) (bool, string)
	Write(ctx context.Context, bucket, pth string, writer io.Writer) error
	ValidHost() string
}

type bucketFileSystem struct {
	client  *client.Client
	keys    *collections.IPNSKeys
	session string
	host    string
}

// serveBucket returns a middleware handler that serves files in a bucket.
func serveBucket(fs serveBucketFileSystem) gin.HandlerFunc {
	return func(c *gin.Context) {
		bkey, err := bucketFromHost(c.Request.Host, fs.ValidHost())
		if err != nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
		defer cancel()
		threadID, err := fs.GetThread(ctx, bkey)
		if err != nil {
			return
		}
		ctx = common.NewThreadIDContext(ctx, threadID)
		token := thread.Token(c.Query("token"))
		if token.Defined() {
			ctx = thread.NewTokenContext(ctx, token)
		}

		exists, target := fs.Exists(ctx, bkey, c.Request.URL.Path)
		if exists {
			c.Writer.WriteHeader(http.StatusOK)
			ctype := mime.TypeByExtension(filepath.Ext(c.Request.URL.Path))
			if ctype == "" {
				ctype = "application/octet-stream"
			}
			c.Writer.Header().Set("Content-Type", ctype)
			if err := fs.Write(ctx, bkey, c.Request.URL.Path, c.Writer); err != nil {
				renderError(c, http.StatusInternalServerError, err)
			} else {
				c.Abort()
			}
		} else if target != "" {
			content := path.Join(c.Request.URL.Path, target)
			ctype := mime.TypeByExtension(filepath.Ext(content))
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Header().Set("Content-Type", ctype)
			if err := fs.Write(ctx, bkey, content, c.Writer); err != nil {
				renderError(c, http.StatusInternalServerError, err)
			} else {
				c.Abort()
			}
		}
	}
}

func (f *bucketFileSystem) GetThread(ctx context.Context, bkey string) (id thread.ID, err error) {
	key, err := f.keys.GetByCid(ctx, bkey)
	if err != nil {
		return
	}
	return key.ThreadID, nil
}

func (f *bucketFileSystem) Exists(ctx context.Context, key, pth string) (ok bool, name string) {
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

func (f *bucketFileSystem) Write(ctx context.Context, key, pth string, writer io.Writer) error {
	ctx = common.NewSessionContext(ctx, f.session)
	return f.client.PullPath(ctx, key, pth, writer)
}

func (f *bucketFileSystem) ValidHost() string {
	return f.host
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
