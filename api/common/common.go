package common

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	mbase "github.com/multiformats/go-multibase"
	"github.com/textileio/go-threads/core/thread"
)

type ctxKey string

func NewSessionContext(ctx context.Context, session string) context.Context {
	if session == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("session"), session)
}

func SessionFromContext(ctx context.Context) (string, bool) {
	session, ok := ctx.Value(ctxKey("session")).(string)
	return session, ok
}

func SessionFromMD(ctx context.Context) (session string, ok bool) {
	session = metautils.ExtractIncoming(ctx).Get("x-textile-session")
	if session != "" {
		ok = true
	}
	return
}

func NewOrgNameContext(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("orgName"), name)
}

func OrgNameFromContext(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(ctxKey("orgName")).(string)
	return name, ok
}

func OrgNameFromMD(ctx context.Context) (name string, ok bool) {
	name = metautils.ExtractIncoming(ctx).Get("x-textile-org")
	if name != "" {
		ok = true
	}
	return
}

func NewAPIKeyContext(ctx context.Context, key string) context.Context {
	if key == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("apiKey"), key)
}

func APIKeyFromContext(ctx context.Context) (string, bool) {
	key, ok := ctx.Value(ctxKey("apiKey")).(string)
	return key, ok
}

func APIKeyFromMD(ctx context.Context) (key string, ok bool) {
	key = metautils.ExtractIncoming(ctx).Get("x-textile-api-key")
	if key != "" {
		ok = true
	}
	return
}

func NewAPISigContext(ctx context.Context, sig []byte) context.Context {
	if sig == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("apiSig"), sig)
}

func APISigFromContext(ctx context.Context) ([]byte, bool) {
	sig, ok := ctx.Value(ctxKey("apiSig")).([]byte)
	return sig, ok
}

func APISigFromMD(ctx context.Context) (sig []byte, ok bool) {
	str := metautils.ExtractIncoming(ctx).Get("x-textile-api-sig")
	if str == "" {
		return
	}
	var err error
	_, sig, err = mbase.Decode(str)
	if err != nil {
		return
	}
	return sig, true
}

func NewThreadIDContext(ctx context.Context, id thread.ID) context.Context {
	if !id.Defined() {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("threadID"), id)
}

func ThreadIDFromContext(ctx context.Context) (thread.ID, bool) {
	id, ok := ctx.Value(ctxKey("threadID")).(thread.ID)
	return id, ok
}

func ThreadIDFromMD(ctx context.Context) (id thread.ID, ok bool) {
	str := metautils.ExtractIncoming(ctx).Get("x-textile-thread")
	if str == "" {
		return
	}
	var err error
	id, err = thread.Decode(str)
	if err != nil {
		return id, false
	}
	return id, true
}

func NewThreadNameContext(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("threadName"), name)
}

func ThreadNameFromContext(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(ctxKey("threadName")).(string)
	return name, ok
}

func ThreadNameFromMD(ctx context.Context) (name string, ok bool) {
	name = metautils.ExtractIncoming(ctx).Get("x-textile-thread-name")
	if name != "" {
		ok = true
	}
	return
}

type Credentials struct {
	Secure bool
}

func (c Credentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	session, ok := SessionFromContext(ctx)
	if ok {
		md["x-textile-session"] = session
	}
	orgName, ok := OrgNameFromContext(ctx)
	if ok {
		md["x-textile-org"] = orgName
	}
	apiKey, ok := APIKeyFromContext(ctx)
	if ok {
		md["x-textile-api-key"] = apiKey
	}
	apiSig, ok := APISigFromContext(ctx)
	if ok {
		var err error
		md["x-textile-api-sig"], err = mbase.Encode(mbase.Base64, apiSig)
		if err != nil {
			return nil, err
		}
	}
	threadID, ok := ThreadIDFromContext(ctx)
	if ok {
		md["x-textile-thread"] = threadID.String()
	}
	threadName, ok := ThreadNameFromContext(ctx)
	if ok {
		md["x-textile-thread-name"] = threadName
	}
	threadToken, ok := thread.TokenFromContext(ctx)
	if ok {
		md["authorization"] = "bearer " + string(threadToken)
	}
	return md, nil
}

func (c Credentials) RequireTransportSecurity() bool {
	return c.Secure
}
