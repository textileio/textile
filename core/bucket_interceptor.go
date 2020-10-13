package core

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcm "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/textileio/textile/v2/buckets"
	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc"
)

var (
	// ErrAccountDelinquent indicates the latest charge to the account failed.
	ErrAccountDelinquent = errors.New("payment required")
)

// bucketInterceptor adds context info needed to account for bucket usage.
func (t *Textile) bucketInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		var account *mdb.Account
		if org, ok := mdb.OrgFromContext(stream.Context()); ok {
			account = org
		} else if dev, ok := mdb.DevFromContext(stream.Context()); ok {
			account = dev
		}
		// @todo: Account for users after User -> Account migration
		// else if user, ok := mdb.UserFromContext(ctx); ok {}
		if account == nil || account.CustomerID == "" {
			return handler(srv, stream)
		}
		cus, err := t.bc.GetCustomer(stream.Context(), account.CustomerID)
		if err != nil {
			return err
		}
		if cus.Delinquent {
			return status.Error(codes.FailedPrecondition, ErrAccountDelinquent.Error())
		}
		var newCtx context.Context
		switch info.FullMethod {
		case "/api.buckets.pb.APIService/Create",
			"/api.buckets.pb.APIService/PushPath",
			"/api.buckets.pb.APIService/SetPath",
			"/api.buckets.pb.APIService/Remove",
			"/api.buckets.pb.APIService/RemovePath",
			"/api.buckets.pb.APIService/PushPathAccessRoles":
			usage, err := t.bc.GetStoredData(stream.Context(), account.CustomerID)
			if err != nil {
				return err
			}
			owner := &buckets.BucketOwner{}
			if !cus.Billable {
				// Customer is not billable (no payment source), limit to free quota
				owner.StorageLimitedToQuota = true
				owner.StorageQuota = usage.FreeSize
			}
			newCtx = buckets.NewBucketOwnerContext(stream.Context(), owner)
		default:
			return handler(srv, stream)
		}
		wrapped := grpcm.WrapServerStream(stream)
		wrapped.WrappedContext = newCtx
		return handler(srv, wrapped)
	}
}
