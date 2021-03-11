package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	grpcm "github.com/grpc-ecosystem/go-grpc-middleware"
	powc "github.com/textileio/powergate/v2/api/client"
	"github.com/textileio/textile/v2/api/billingd/analytics"
	billing "github.com/textileio/textile/v2/api/billingd/client"
	"github.com/textileio/textile/v2/api/billingd/common"
	"github.com/textileio/textile/v2/api/billingd/pb"
	"github.com/textileio/textile/v2/buckets"
	mdb "github.com/textileio/textile/v2/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type preFunc func(ctx context.Context, method string) (context.Context, error)
type postFunc func(ctx context.Context, method string) error

func unaryServerInterceptor(pre preFunc, post postFunc) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		newCtx, err := pre(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		res, err := handler(newCtx, req)
		if err != nil {
			return nil, err
		}
		if err = post(newCtx, info.FullMethod); err != nil {
			return nil, err
		}
		return res, nil
	}
}

func streamServerInterceptor(pre preFunc, post postFunc) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		newCtx, err := pre(stream.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		wrapped := grpcm.WrapServerStream(stream)
		wrapped.WrappedContext = newCtx
		err = handler(srv, wrapped)
		if err != nil {
			return err
		}
		return post(newCtx, info.FullMethod)
	}
}

func (t *Textile) preUsageFunc(ctx context.Context, method string) (context.Context, error) {
	if t.bc == nil {
		return ctx, nil
	}
	for _, ignored := range authIgnoredMethods {
		if method == ignored {
			return ctx, nil
		}
	}
	for _, ignored := range usageIgnoredMethods {
		if method == ignored {
			return ctx, nil
		}
	}
	account, ok := mdb.AccountFromContext(ctx)
	if !ok {
		return ctx, nil
	}
	now := time.Now()

	// Collect new users.
	if account.User != nil && account.User.CreatedAt.IsZero() && account.User.Type == mdb.User {
		var powInfo *mdb.PowInfo
		if t.pc != nil {
			ctxAdmin := context.WithValue(ctx, powc.AdminKey, t.conf.PowergateAdminToken)
			res, err := t.pc.Admin.Users.Create(ctxAdmin)
			if err != nil {
				return ctx, err
			}
			powInfo = &mdb.PowInfo{ID: res.User.Id, Token: res.User.Token}
		}
		user, err := t.collections.Accounts.CreateUser(ctx, account.User.Key, powInfo)
		if err != nil {
			return ctx, err
		}
		ctx = mdb.NewAccountContext(ctx, user, account.Org)
		account, _ = mdb.AccountFromContext(ctx)
	}

	// Collect new customers.
	cus, err := t.bc.GetCustomer(ctx, account.Owner().Key)
	if err != nil {
		if strings.Contains(err.Error(), mongo.ErrNoDocuments.Error()) {
			email, err := t.getAccountCtxEmail(ctx, account)
			if err != nil {
				return ctx, err
			}
			var opts []billing.Option
			if account.Owner().Type == mdb.User {
				key, ok := mdb.APIKeyFromContext(ctx)
				if !ok {
					return ctx, status.Error(codes.PermissionDenied, "Bad API key")
				}
				parent, err := t.collections.Accounts.Get(ctx, key.Owner)
				if err != nil {
					return nil, fmt.Errorf("parent for %s not found: %s", account.Owner().Key, key.Owner)
				}
				email, err := t.getAccountCtxEmail(ctx, mdb.AccountCtxForAccount(parent))
				if err != nil {
					return ctx, err
				}
				opts = append(opts, billing.WithParent(parent.Key, email, parent.Type))
			}
			if _, err := t.bc.CreateCustomer(
				ctx,
				account.Owner().Key,
				email,
				account.Owner().Username,
				account.Owner().Type,
				opts...,
			); err != nil {
				return ctx, err
			}
			cus, err = t.bc.GetCustomer(ctx, account.Owner().Key)
			if err != nil {
				return ctx, err
			}
		} else {
			return ctx, err
		}
	}
	if err := common.StatusCheck(cus.SubscriptionStatus); err != nil {
		return ctx, status.Error(codes.FailedPrecondition, err.Error())
	}

	if usageExhausted(cus, "network_egress", now) {
		err = fmt.Errorf("network egress exhausted: %v", common.ErrExceedsFreeQuota)
		return ctx, status.Error(codes.ResourceExhausted, err.Error())
	}

	// @todo: Attach egress info that can be used to fail-fast in PullPath?
	switch method {
	case "/api.bucketsd.pb.APIService/Create",
		"/api.bucketsd.pb.APIService/PushPath",
		"/api.bucketsd.pb.APIService/PushPaths",
		"/api.bucketsd.pb.APIService/SetPath",
		"/api.bucketsd.pb.APIService/MovePath",
		"/api.bucketsd.pb.APIService/Remove",
		"/api.bucketsd.pb.APIService/RemovePath",
		"/api.bucketsd.pb.APIService/PushPathAccessRoles":
		owner := &buckets.BucketOwner{
			StorageUsed: cus.DailyUsage["stored_data"].Total,
		}
		if cus.Billable {
			// Customer is paying for storage, quota is unbounded
			owner.StorageAvailable = int64(math.MaxInt64)
		} else if now.Unix() < cus.GracePeriodEnd {
			// Customer is in grace period, quota is grace
			owner.StorageAvailable = cus.DailyUsage["stored_data"].Grace
		} else if cus.GracePeriodEnd == 0 {
			// Customer has not started grace period, but there's no way to start the grace period _after_
			// a successful request, so we use grace quota here and let the post usage handler start the
			// grace period in the event the request exceeds the free quota
			owner.StorageAvailable = cus.DailyUsage["stored_data"].Grace
		} else {
			// Customer's grace period has expired, fall back to free quota.
			owner.StorageAvailable = cus.DailyUsage["stored_data"].Free
		}
		ctx = buckets.NewBucketOwnerContext(ctx, owner)
	case
		"/threads.pb.API/Verify",
		"/threads.pb.API/Has",
		"/threads.pb.API/Find",
		"/threads.pb.API/FindByID",
		"/threads.pb.API/ReadTransaction",
		"/threads.pb.API/Listen":
		if usageExhausted(cus, "instance_reads", now) {
			err = fmt.Errorf("threaddb reads exhausted: %v", common.ErrExceedsFreeQuota)
			return ctx, status.Error(codes.ResourceExhausted, err.Error())
		}
	case "/threads.pb.API/Create",
		"/threads.pb.API/Save",
		"/threads.pb.API/Delete",
		"/threads.pb.API/WriteTransaction":
		if usageExhausted(cus, "instance_writes", now) {
			err = fmt.Errorf("threaddb writes exhausted: %v", common.ErrExceedsFreeQuota)
			return ctx, status.Error(codes.ResourceExhausted, err.Error())
		}
	}
	return ctx, nil
}

func usageExhausted(cus *pb.GetCustomerResponse, key string, now time.Time) bool {
	if !cus.Billable && cus.DailyUsage[key].Free == 0 {
		if now.Unix() >= cus.GracePeriodEnd {
			return true // Grace period ended
		} else if cus.DailyUsage[key].Grace == 0 {
			return true // Still in grace period, but reached the hard cap
		}
	}
	return false
}

func (t *Textile) postUsageFunc(ctx context.Context, method string) error {
	if t.bc == nil {
		return nil
	}
	for _, ignored := range authIgnoredMethods {
		if method == ignored {
			return nil
		}
	}
	account, ok := mdb.AccountFromContext(ctx)
	if !ok {
		return nil
	}
	owner, ok := buckets.BucketOwnerFromContext(ctx)
	if !ok {
		return nil
	}
	switch method {
	case "/api.bucketsd.pb.APIService/Create",
		"/api.bucketsd.pb.APIService/PushPath",
		"/api.bucketsd.pb.APIService/PushPaths",
		"/api.bucketsd.pb.APIService/SetPath",
		"/api.bucketsd.pb.APIService/MovePath",
		"/api.bucketsd.pb.APIService/Remove",
		"/api.bucketsd.pb.APIService/RemovePath",
		"/api.bucketsd.pb.APIService/PushPathAccessRoles":
		if _, err := t.bc.IncCustomerUsage(
			ctx,
			account.Owner().Key,
			map[string]int64{
				"stored_data": owner.StorageDelta,
			},
		); err != nil {
			return err
		}
	}

	if t.bc != nil {
		payload := map[string]string{}
		if account.User != nil {
			payload["member"] = account.User.Key.String()
			payload["member_username"] = account.User.Username
			payload["member_email"] = account.User.Email
		}

		switch method {
		case "/api.bucketsd.pb.APIService/Create":
			t.bc.TrackEvent(ctx, account.Owner().Key, account.Owner().Type, true, analytics.BucketCreated, payload)
		case "/api.bucketsd.pb.APIService/Archive":
			t.bc.TrackEvent(ctx, account.Owner().Key, account.Owner().Type, true, analytics.BucketArchiveCreated, payload)
		case "/threads.pb.API/NewDB":
			t.bc.TrackEvent(ctx, account.Owner().Key, account.Owner().Type, true, analytics.ThreadDbCreated, payload)
		}
	}
	return nil
}

func (t *Textile) getAccountCtxEmail(ctx context.Context, account *mdb.AccountCtx) (string, error) {
	if account.User != nil {
		return account.User.Email, nil
	}
	if account.Org == nil {
		return "", errors.New("invalid account context")
	}
	for _, m := range account.Org.Members {
		if m.Role == mdb.OrgOwner {
			owner, err := t.collections.Accounts.Get(ctx, m.Key)
			if err != nil {
				log.Errorf("getting org owner: %v", err)
				continue
			}
			return owner.Email, nil
		}
	}
	return "", errors.New("could not resolve email for org")
}
