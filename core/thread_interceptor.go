package core

import (
	"context"
	"errors"

	ma "github.com/multiformats/go-multiaddr"
	dbpb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/core/thread"
	netpb "github.com/textileio/go-threads/net/api/pb"
	"github.com/textileio/textile/v2/api/common"
	mdb "github.com/textileio/textile/v2/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// threadInterceptor monitors for thread creation and deletion.
// Textile tracks threads against dev, org, and user accounts.
// Users must supply a valid API key from a dev/org.
func (t *Textile) threadInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		method, _ := grpc.Method(ctx)
		for _, ignored := range authIgnoredMethods {
			if method == ignored {
				return handler(ctx, req)
			}
		}
		for _, block := range blockMethods {
			if method == block {
				return nil, status.Error(codes.PermissionDenied, "Method is not accessible")
			}
		}
		if sid, ok := common.SessionFromContext(ctx); ok && sid == t.internalHubSession {
			return handler(ctx, req)
		}

		var owner thread.PubKey
		if org, ok := mdb.OrgFromContext(ctx); ok {
			owner = org.Key
		} else if dev, ok := mdb.DevFromContext(ctx); ok {
			owner = dev.Key
		} else if user, ok := mdb.UserFromContext(ctx); ok {
			owner = user.Key
		}

		var newID thread.ID
		var isDB bool
		var err error
		switch method {
		case "/threads.pb.API/NewDB":
			newID, err = thread.Cast(req.(*dbpb.NewDBRequest).DbID)
			if err != nil {
				return nil, err
			}
			isDB = true
		case "/threads.pb.API/NewDBFromAddr":
			addr, err := ma.NewMultiaddrBytes(req.(*dbpb.NewDBFromAddrRequest).Addr)
			if err != nil {
				return nil, err
			}
			newID, err = thread.FromAddr(addr)
			if err != nil {
				return nil, err
			}
			isDB = true
		case "/threads.net.pb.API/CreateThread":
			newID, err = thread.Cast(req.(*netpb.CreateThreadRequest).ThreadID)
			if err != nil {
				return nil, err
			}
		case "/threads.net.pb.API/AddThread":
			addr, err := ma.NewMultiaddrBytes(req.(*netpb.AddThreadRequest).Addr)
			if err != nil {
				return nil, err
			}
			newID, err = thread.FromAddr(addr)
			if err != nil {
				return nil, err
			}
		default:
			// If we're dealing with an existing thread, make sure that the owner
			// owns the thread directly or via an API key.
			threadID, ok := common.ThreadIDFromContext(ctx)
			if ok {
				th, err := t.collections.Threads.Get(ctx, threadID, owner)
				if err != nil && errors.Is(err, mongo.ErrNoDocuments) {
					// Allow non-owners to interact with a limited set of APIs.
					var isAllowed bool
					for _, m := range allowedCrossUserMethods {
						if method == m {
							isAllowed = true
							break
						}
					}
					if !isAllowed {
						return nil, status.Error(codes.PermissionDenied, "User does not own thread")
					}
				} else if err != nil {
					return nil, err
				}
				if th != nil {
					key, _ := mdb.APIKeyFromContext(ctx)
					if key != nil && key.Type == mdb.UserKey {
						// Extra user check for user API keys.
						if key.Key != th.Key {
							return nil, status.Error(codes.PermissionDenied, "Bad API key")
						}
					}
				}
			}
		}

		// Collect the user if we haven't seen them before.
		user, ok := mdb.UserFromContext(ctx)
		if ok && user.CreatedAt.IsZero() {
			var powInfo *mdb.PowInfo
			if t.pc != nil {
				ffsId, ffsToken, err := t.pc.FFS.Create(ctx)
				if err != nil {
					return nil, err
				}
				powInfo = &mdb.PowInfo{ID: ffsId, Token: ffsToken}
			}
			if err := t.collections.Users.Create(ctx, owner, powInfo); err != nil {
				return nil, err
			}
		}
		if !ok && owner != nil {
			// Add the dev/org as the user for the user API.
			ctx = mdb.NewUserContext(ctx, &mdb.User{Key: owner})
		}

		// Preemptively track the new thread ID for the owner.
		// This needs to happen before the request is handled in case there's a conflict
		// with the owner and thread name.
		if newID.Defined() {
			thds, err := t.collections.Threads.ListByOwner(ctx, owner)
			if err != nil {
				return nil, err
			}
			if t.conf.MaxNumberThreadsPerOwner > 0 && len(thds) >= t.conf.MaxNumberThreadsPerOwner {
				return nil, ErrTooManyThreadsPerOwner
			}
			if _, err := t.collections.Threads.Create(ctx, newID, owner, isDB); err != nil {
				return nil, err
			}
		}

		// Track the thread ID marked for deletion.
		var deleteID thread.ID
		switch method {
		case "/threads.pb.API/DeleteDB":
			deleteID, err = thread.Cast(req.(*dbpb.DeleteDBRequest).DbID)
			if err != nil {
				return nil, err
			}
			keys, err := t.collections.IPNSKeys.ListByThreadID(ctx, deleteID)
			if err != nil {
				return nil, err
			}
			if len(keys) != 0 {
				return nil, status.Error(codes.FailedPrecondition, "DB not empty (delete buckets first)")
			}
		case "/threads.net.pb.API/DeleteThread":
			deleteID, err = thread.Cast(req.(*netpb.DeleteThreadRequest).ThreadID)
			if err != nil {
				return nil, err
			}
		}

		// Let the request pass through.
		res, err := handler(ctx, req)
		if err != nil {
			// Clean up the new thread if there was an error.
			if newID.Defined() {
				if err := t.collections.Threads.Delete(ctx, newID, owner); err != nil {
					log.Errorf("error deleting thread %s: %v", newID, err)
				}
			}
			return res, err
		}

		// Clean up the tracked thread if it was deleted.
		if deleteID.Defined() {
			if err := t.collections.Threads.Delete(ctx, deleteID, owner); err != nil {
				return nil, err
			}
		}
		return res, nil
	}
}
