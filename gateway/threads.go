package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/buckets"
)

// collectionHandler handles collection requests.
func (g *Gateway) collectionHandler(c *gin.Context) {
	threadID, err := thread.Decode(c.Param("thread"))
	if err != nil {
		renderError(c, http.StatusBadRequest, fmt.Errorf("invalid thread ID"))
		return
	}
	g.renderCollection(c, threadID, c.Param("collection"))
}

// renderCollection renders all instances in a collection.
func (g *Gateway) renderCollection(c *gin.Context, threadID thread.ID, collection string) {
	ctx, cancel := context.WithTimeout(common.NewSessionContext(context.Background(), g.apiSession), handlerTimeout)
	defer cancel()
	ctx = common.NewThreadIDContext(ctx, threadID)
	token := thread.Token(c.Query("token"))
	if token.Defined() {
		ctx = thread.NewTokenContext(ctx, token)
	}

	jsn := c.Query("json") == "true"
	if collection == buckets.CollectionName && !jsn {
		g.renderBucket(c, ctx, threadID, token)
		return
	} else {
		var dummy interface{}
		res, err := g.threads.Find(ctx, threadID, collection, &db.Query{}, &dummy, db.WithTxnToken(token))
		if err != nil {
			render404(c)
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// instanceHandler handles collection instance requests.
func (g *Gateway) instanceHandler(c *gin.Context) {
	threadID, err := thread.Decode(c.Param("thread"))
	if err != nil {
		renderError(c, http.StatusBadRequest, fmt.Errorf("invalid thread ID"))
		return
	}
	g.renderInstance(c, threadID, c.Param("collection"), c.Param("id"), c.Param("path"))
}

// renderInstance renders an instance in a collection.
// If the collection is buckets, the built-in buckets UI in rendered instead.
// This can be overridden with the query param json=true.
func (g *Gateway) renderInstance(c *gin.Context, threadID thread.ID, collection, id, pth string) {
	pth = strings.TrimPrefix(pth, "/")
	jsn := c.Query("json") == "true"
	if (collection != buckets.CollectionName || jsn) && pth != "" {
		render404(c)
		return
	}

	ctx, cancel := context.WithTimeout(common.NewSessionContext(context.Background(), g.apiSession), handlerTimeout)
	defer cancel()
	ctx = common.NewThreadIDContext(ctx, threadID)
	token := thread.Token(c.Query("token"))
	if token.Defined() {
		ctx = thread.NewTokenContext(ctx, token)
	}

	if collection == buckets.CollectionName && !jsn {
		g.renderBucketPath(c, ctx, threadID, collection, id, pth, token)
		return
	} else {
		var res interface{}
		if err := g.threads.FindByID(ctx, threadID, collection, id, &res, db.WithTxnToken(token)); err != nil {
			render404(c)
			return
		}
		c.JSON(http.StatusOK, res)
	}
}
