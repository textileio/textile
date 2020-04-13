package common

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/libp2p/go-libp2p-core/crypto"
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

func SessionFromMD(ctx context.Context) (dev string, ok bool) {
	dev = metautils.ExtractIncoming(ctx).Get("x-textile-session")
	if dev != "" {
		ok = true
	}
	return
}

func NewOrgNameContext(ctx context.Context, org string) context.Context {
	if org == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("org"), org)
}

func OrgNameFromContext(ctx context.Context) (string, bool) {
	org, ok := ctx.Value(ctxKey("org")).(string)
	return org, ok
}

func OrgNameFromMD(ctx context.Context) (name string, ok bool) {
	name = metautils.ExtractIncoming(ctx).Get("x-textile-org")
	if name != "" {
		ok = true
	}
	return
}

func NewAPIKeyContext(ctx context.Context, key crypto.PubKey) context.Context {
	if key == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("apiKey"), key)
}

func APIKeyFromContext(ctx context.Context) (crypto.PubKey, bool) {
	key, ok := ctx.Value(ctxKey("apiKey")).(crypto.PubKey)
	return key, ok
}

func APIKeyFromMD(ctx context.Context) (key crypto.PubKey, ok bool) {
	str := metautils.ExtractIncoming(ctx).Get("x-textile-api-key")
	if str == "" {
		return
	}
	_, data, err := mbase.Decode(str)
	if err != nil {
		return
	}
	key, err = crypto.UnmarshalPublicKey(data)
	if err != nil {
		return
	}
	return key, true
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
		b, err := crypto.MarshalPublicKey(apiKey)
		if err != nil {
			return nil, err
		}
		md["x-textile-api-key"], err = mbase.Encode(mbase.Base64, b)
		if err != nil {
			return nil, err
		}
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
	threadToken, ok := thread.TokenFromContext(ctx)
	if ok {
		md["authorization"] = "bearer " + string(threadToken)
	}
	return md, nil
}

func (c Credentials) RequireTransportSecurity() bool {
	return c.Secure
}
