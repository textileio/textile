package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/textile/api/buckets"
	"github.com/textileio/textile/api/common"
)

// collectionHandler renders all instances in a collection.
func (g *Gateway) collectionHandler(c *gin.Context) {
	collection := c.Param("collection")

	ctx, cancel := context.WithTimeout(common.NewSessionContext(context.Background(), g.apiSession), handlerTimeout)
	defer cancel()
	threadID, err := thread.Decode(c.Param("thread"))
	if err != nil {
		renderError(c, http.StatusBadRequest, fmt.Errorf("thread is not valid"))
		return
	}
	ctx = common.NewThreadIDContext(ctx, threadID)
	token := thread.Token(c.Query("token"))
	if token.Defined() {
		ctx = thread.NewTokenContext(ctx, token)
	}

	json := c.Query("json") == "true"
	if collection == buckets.CollectionName && !json {
		g.renderBucket(c, ctx, threadID)
		return
	} else {
		var dummy interface{}
		res, err := g.threads.Find(ctx, threadID, collection, &db.Query{}, &dummy, db.WithTxnToken(token))
		if err != nil {
			renderError(c, http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// instanceHandler renders an instance in a collection.
// If the collection is buckets, the built-in buckets UI in rendered instead.
// This can be overridden with the query param json=true.
func (g *Gateway) instanceHandler(c *gin.Context) {
	collection := c.Param("collection")
	json := c.Query("json") == "true"
	if (collection != buckets.CollectionName || json) && c.Param("path") != "" {
		render404(c)
		return
	}

	ctx, cancel := context.WithTimeout(common.NewSessionContext(context.Background(), g.apiSession), handlerTimeout)
	defer cancel()
	threadID, err := thread.Decode(c.Param("thread"))
	if err != nil {
		renderError(c, http.StatusBadRequest, fmt.Errorf("thread is not valid"))
		return
	}
	ctx = common.NewThreadIDContext(ctx, threadID)
	token := thread.Token(c.Query("token"))
	if token.Defined() {
		ctx = thread.NewTokenContext(ctx, token)
	}

	if collection == buckets.CollectionName && !json {
		g.renderBucketPath(c, ctx, threadID, collection, token)
		return
	} else {
		var res interface{}
		if err := g.threads.FindByID(ctx, threadID, collection, c.Param("id"), &res, db.WithTxnToken(token)); err != nil {
			render404(c)
			return
		}
		c.JSON(http.StatusOK, res)
	}
}
