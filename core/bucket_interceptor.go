package core

import (
	"context"

	grpcm "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/textileio/textile/v2/buckets"
	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc"
)

func (t *Textile) bucketInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		var account *mdb.Account
		if org, ok := mdb.OrgFromContext(stream.Context()); ok {
			account = org
		} else if dev, ok := mdb.DevFromContext(stream.Context()); ok {
			account = dev
		}
		// else if user, ok := mdb.UserFromContext(ctx); ok {}
		if account == nil || account.CustomerID == "" {
			return handler(srv, stream)
		}

		var newCtx context.Context
		switch info.FullMethod {
		case "/api.buckets.pb.APIService/PushPath":
			usage, err := t.bc.GetPeriodUsage(stream.Context(), account.CustomerID)
			if err != nil {
				return err
			}
			// @todo: Usage should return a bool indicating if the push should be allowed,
			//        e.g., an invoice charge failed, etc.

			newCtx = buckets.NewBucketOwnerContext(stream.Context(), &buckets.BucketOwner{
				ID:               account.CustomerID,
				StorageTotalSize: usage.StoredData.TotalSize,
			})
		default:
			return handler(srv, stream)
		}
		wrapped := grpcm.WrapServerStream(stream)
		wrapped.WrappedContext = newCtx
		return handler(srv, wrapped)
	}
}
