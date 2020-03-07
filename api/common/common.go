package common

import (
	"context"

	"github.com/textileio/go-threads/core/thread"
)

type ctxKey string

func NewStoreContext(ctx context.Context, id thread.ID) context.Context {
	return context.WithValue(ctx, ctxKey("store"), id)
}

func StoreFromContext(ctx context.Context) (thread.ID, bool) {
	id, ok := ctx.Value(ctxKey("store")).(thread.ID)
	return id, ok
}
