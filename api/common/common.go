package common

import (
	"context"
	"encoding/base64"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/libp2p/go-libp2p-core/crypto"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/thread"
)

type ctxKey string

func NewDevTokenContext(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ctxKey("devToken"), token)
}

func DevTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(ctxKey("devToken")).(string)
	return token, ok
}

func DevTokenFromMD(ctx context.Context) (dev string, ok bool) {
	dev = metautils.ExtractIncoming(ctx).Get("x-dev")
	if dev != "" {
		ok = true
	}
	return
}

func NewOrgContext(ctx context.Context, org string) context.Context {
	return context.WithValue(ctx, ctxKey("org"), org)
}

func OrgFromContext(ctx context.Context) (string, bool) {
	org, ok := ctx.Value(ctxKey("org")).(string)
	return org, ok
}

func OrgFromMD(ctx context.Context) (org string, ok bool) {
	org = metautils.ExtractIncoming(ctx).Get("x-org")
	if org != "" {
		ok = true
	}
	return
}

func NewAppKeyContext(ctx context.Context, key crypto.PrivKey) context.Context {
	return context.WithValue(ctx, ctxKey("appKey"), key)
}

func AppKeyFromContext(ctx context.Context) (crypto.PrivKey, bool) {
	key, ok := ctx.Value(ctxKey("appKey")).(crypto.PrivKey)
	return key, ok
}

func AppKeyFromMD(ctx context.Context) (key crypto.PrivKey, ok bool) {
	str := metautils.ExtractIncoming(ctx).Get("x-app-key")
	if str == "" {
		return
	}
	data, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return
	}
	key, err = crypto.UnmarshalPrivateKey(data)
	if err != nil {
		return
	}
	return key, true
}

func NewAppAddrContext(ctx context.Context, addr ma.Multiaddr) context.Context {
	return context.WithValue(ctx, ctxKey("appAddr"), addr)
}

func AppAddrFromContext(ctx context.Context) (ma.Multiaddr, bool) {
	addr, ok := ctx.Value(ctxKey("appAddr")).(ma.Multiaddr)
	return addr, ok
}

func AppAddrFromMD(ctx context.Context) (addr ma.Multiaddr, ok bool) {
	str := metautils.ExtractIncoming(ctx).Get("x-app-addr")
	if str == "" {
		return
	}
	var err error
	addr, err = ma.NewMultiaddr(str)
	if err != nil {
		return
	}
	return addr, true
}

func NewDbIDContext(ctx context.Context, id thread.ID) context.Context {
	return context.WithValue(ctx, ctxKey("dbID"), id)
}

func DbIDFromContext(ctx context.Context) (thread.ID, bool) {
	id, ok := ctx.Value(ctxKey("dbID")).(thread.ID)
	return id, ok
}

func DbIDFromMD(ctx context.Context) (id thread.ID, ok bool) {
	str := metautils.ExtractIncoming(ctx).Get("x-db")
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
	devToken, ok := DevTokenFromContext(ctx)
	if ok {
		md["x-dev"] = devToken
	}
	org, ok := OrgFromContext(ctx)
	if ok {
		md["x-org"] = org
	}
	appKey, ok := AppKeyFromContext(ctx)
	if ok {
		b, err := crypto.MarshalPrivateKey(appKey)
		if err != nil {
			return nil, err
		}
		md["x-app-key"] = base64.StdEncoding.EncodeToString(b)
	}
	appAddr, ok := AppAddrFromContext(ctx)
	if ok {
		md["x-app-addr"] = appAddr.String()
	}
	dbID, ok := DbIDFromContext(ctx)
	if ok {
		md["x-db"] = dbID.String()
	}
	return md, nil
}

func (c Credentials) RequireTransportSecurity() bool {
	return c.Secure
}
