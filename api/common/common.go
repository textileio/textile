package common

import (
	"context"
	"encoding/hex"

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

func NewOrgContext(ctx context.Context, org string) context.Context {
	return context.WithValue(ctx, ctxKey("org"), org)
}

func OrgFromContext(ctx context.Context) (string, bool) {
	org, ok := ctx.Value(ctxKey("org")).(string)
	return org, ok
}

func NewAppKeyContext(ctx context.Context, key crypto.PrivKey) context.Context {
	return context.WithValue(ctx, ctxKey("appKey"), key)
}

func AppKeyFromContext(ctx context.Context) (crypto.PrivKey, bool) {
	key, ok := ctx.Value(ctxKey("appKey")).(crypto.PrivKey)
	return key, ok
}

func NewAppAddrContext(ctx context.Context, addr ma.Multiaddr) context.Context {
	return context.WithValue(ctx, ctxKey("appAddr"), addr)
}

func AppAddrFromContext(ctx context.Context) (ma.Multiaddr, bool) {
	addr, ok := ctx.Value(ctxKey("appAddr")).(ma.Multiaddr)
	return addr, ok
}

func NewDBContext(ctx context.Context, id thread.ID) context.Context {
	return context.WithValue(ctx, ctxKey("dbID"), id)
}

func DBFromContext(ctx context.Context) (thread.ID, bool) {
	id, ok := ctx.Value(ctxKey("dbID")).(thread.ID)
	return id, ok
}

type Credentials struct {
	Secure bool
}

func (c Credentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	devToken, ok := DevTokenFromContext(ctx)
	if ok {
		md["authorization"] = "bearer " + devToken
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
		md["authorization"] = "bearer " + hex.EncodeToString(b)
	}
	appAddr, ok := AppAddrFromContext(ctx)
	if ok {
		md["x-app-addr"] = appAddr.String()
	}
	dbID, ok := DBFromContext(ctx)
	if ok {
		md["x-db-id"] = dbID.String()
	}
	return md, nil
}

func (c Credentials) RequireTransportSecurity() bool {
	return c.Secure
}
