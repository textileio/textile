package core

import (
	"context"
	"errors"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/v2/api/common"
	mdb "github.com/textileio/textile/v2/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (t *Textile) authFunc(ctx context.Context) (context.Context, error) {
	method, _ := grpc.Method(ctx)
	for _, ignored := range ignoreMethods {
		if method == ignored {
			return ctx, nil
		}
	}
	for _, block := range blockMethods {
		if method == block {
			return nil, status.Error(codes.PermissionDenied, "Method is not accessible")
		}
	}

	if threadID, ok := common.ThreadIDFromMD(ctx); ok {
		ctx = common.NewThreadIDContext(ctx, threadID)
	}
	if threadName, ok := common.ThreadNameFromMD(ctx); ok {
		ctx = common.NewThreadNameContext(ctx, threadName)
	}
	if threadToken, err := thread.NewTokenFromMD(ctx); err != nil {
		return nil, err
	} else {
		ctx = thread.NewTokenContext(ctx, threadToken)
	}

	sid, ok := common.SessionFromMD(ctx)
	if ok {
		ctx = common.NewSessionContext(ctx, sid)
		if sid == t.internalHubSession {
			return ctx, nil
		}
		session, err := t.collections.Sessions.Get(ctx, sid)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "Invalid session")
		}
		if time.Now().After(session.ExpiresAt) {
			return nil, status.Error(codes.Unauthenticated, "Expired session")
		}
		if err := t.collections.Sessions.Touch(ctx, session.ID); err != nil {
			return nil, err
		}
		ctx = mdb.NewSessionContext(ctx, session)

		dev, err := t.collections.Accounts.Get(ctx, session.Owner)
		if err != nil {
			return nil, status.Error(codes.NotFound, "User not found")
		}
		ctx = mdb.NewDevContext(ctx, dev)

		orgSlug, ok := common.OrgSlugFromMD(ctx)
		if ok {
			isMember, err := t.collections.Accounts.IsMember(ctx, orgSlug, dev.Key)
			if err != nil {
				return nil, err
			}
			if !isMember {
				return nil, status.Error(codes.PermissionDenied, "User is not an org member")
			} else {
				org, err := t.collections.Accounts.GetByUsername(ctx, orgSlug)
				if err != nil {
					return nil, status.Error(codes.NotFound, "Org not found")
				}
				ctx = mdb.NewOrgContext(ctx, org)
				ctx = common.NewOrgSlugContext(ctx, orgSlug)
				ctx = thread.NewTokenContext(ctx, org.Token)
			}
		} else {
			ctx = thread.NewTokenContext(ctx, dev.Token)
		}
	} else if k, ok := common.APIKeyFromMD(ctx); ok {
		key, err := t.collections.APIKeys.Get(ctx, k)
		if err != nil || !key.Valid {
			return nil, status.Error(codes.NotFound, "API key not found or is invalid")
		}
		ctx = common.NewAPIKeyContext(ctx, k)
		if key.Secure {
			msg, sig, ok := common.APISigFromMD(ctx)
			if !ok {
				return nil, status.Error(codes.Unauthenticated, "API key signature required")
			} else {
				ctx = common.NewAPISigContext(ctx, msg, sig)
				if !common.ValidateAPISigContext(ctx, key.Secret) {
					return nil, status.Error(codes.Unauthenticated, "Bad API key signature")
				}
			}
		}
		switch key.Type {
		case mdb.AccountKey:
			acc, err := t.collections.Accounts.Get(ctx, key.Owner)
			if err != nil {
				return nil, status.Error(codes.NotFound, "Account not found")
			}
			switch acc.Type {
			case mdb.Dev:
				ctx = mdb.NewDevContext(ctx, acc)
			case mdb.Org:
				ctx = mdb.NewOrgContext(ctx, acc)
			}
			ctx = thread.NewTokenContext(ctx, acc.Token)
		case mdb.UserKey:
			token, ok := thread.TokenFromContext(ctx)
			if ok {
				var claims jwt.StandardClaims
				if _, _, err = new(jwt.Parser).ParseUnverified(string(token), &claims); err != nil {
					return nil, status.Error(codes.PermissionDenied, "Bad authorization")
				}
				ukey := &thread.Libp2pPubKey{}
				if err = ukey.UnmarshalString(claims.Subject); err != nil {
					return nil, err
				}
				user, err := t.collections.Users.Get(ctx, ukey)
				if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
					return nil, err
				}
				if user == nil {
					// Attach a temp user context that will be accessible in the next interceptor.
					user = &mdb.User{Key: ukey}
				}
				ctx = mdb.NewUserContext(ctx, user)
			} else if method != "/threads.pb.API/GetToken" && method != "/threads.net.pb.API/GetToken" {
				return nil, status.Error(codes.Unauthenticated, "Token required")
			}
		}
		ctx = mdb.NewAPIKeyContext(ctx, key)
	} else {
		return nil, status.Error(codes.Unauthenticated, "Session or API key required")
	}
	return ctx, nil
}

func (t *Textile) noAuthFunc(ctx context.Context) (context.Context, error) {
	if threadID, ok := common.ThreadIDFromMD(ctx); ok {
		ctx = common.NewThreadIDContext(ctx, threadID)
	}
	if threadToken, err := thread.NewTokenFromMD(ctx); err != nil {
		return nil, err
	} else {
		ctx = thread.NewTokenContext(ctx, threadToken)
	}
	return ctx, nil
}
