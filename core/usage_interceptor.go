package core

import (
	"context"
	"fmt"

	grpcm "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/textileio/textile/v2/api/billingd/common"
	"github.com/textileio/textile/v2/buckets"
	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type preFunc func(ctx context.Context, method string) (context.Context, error)
type postFunc func(ctx context.Context, method string) error

func unaryServerInterceptor(pre preFunc, post postFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
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
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
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
	if !ok || account.Owner().CustomerID == "" {
		return ctx, nil
	}
	cus, err := t.bc.GetCustomer(ctx, account.Owner().CustomerID)
	if err != nil {
		return ctx, err
	}
	if err := common.StatusCheck(cus.Status); err != nil {
		return ctx, status.Error(codes.FailedPrecondition, err.Error())
	}
	if !cus.Billable && cus.NetworkEgress.Free == 0 {
		err = fmt.Errorf("network egress exhausted: %v", common.ErrExceedsFreeUnits)
		return ctx, status.Error(codes.ResourceExhausted, err.Error())
	}

	// @todo: Attach egress info that can be used to fail-fast in PullPath?
	switch method {
	case "/api.bucketsd.pb.APIService/Create",
		"/api.bucketsd.pb.APIService/PushPath",
		"/api.bucketsd.pb.APIService/SetPath",
		"/api.bucketsd.pb.APIService/Remove",
		"/api.bucketsd.pb.APIService/RemovePath",
		"/api.bucketsd.pb.APIService/PushPathAccessRoles":
		owner := &buckets.BucketOwner{
			StorageUsed: cus.StoredData.Total,
		}
		if cus.Billable {
			owner.StorageAvailable = -1
		} else {
			owner.StorageAvailable = cus.StoredData.Free
		}
		ctx = buckets.NewBucketOwnerContext(ctx, owner)
	case
		"/threads.pb.API/Verify",
		"/threads.pb.API/Has",
		"/threads.pb.API/Find",
		"/threads.pb.API/FindByID",
		"/threads.pb.API/ReadTransaction",
		"/threads.pb.API/Listen":
		if !cus.Billable && cus.InstanceReads.Free == 0 {
			err = fmt.Errorf("threaddb reads exhausted: %v", common.ErrExceedsFreeUnits)
			return ctx, status.Error(codes.ResourceExhausted, err.Error())
		}
	case "/threads.pb.API/Create",
		"/threads.pb.API/Save",
		"/threads.pb.API/Delete",
		"/threads.pb.API/WriteTransaction":
		if !cus.Billable && cus.InstanceWrites.Free == 0 {
			err = fmt.Errorf("threaddb writes exhausted: %v", common.ErrExceedsFreeUnits)
			return ctx, status.Error(codes.ResourceExhausted, err.Error())
		}
	}
	return ctx, nil
}

func (t *Textile) postUsageFunc(ctx context.Context, method string) error {
	for _, ignored := range authIgnoredMethods {
		if method == ignored {
			return nil
		}
	}
	account, ok := mdb.AccountFromContext(ctx)
	if !ok || account.Owner().CustomerID == "" {
		return nil
	}
	switch method {
	case "/api.bucketsd.pb.APIService/Create",
		"/api.bucketsd.pb.APIService/PushPath",
		"/api.bucketsd.pb.APIService/SetPath",
		"/api.bucketsd.pb.APIService/Remove",
		"/api.bucketsd.pb.APIService/RemovePath",
		"/api.bucketsd.pb.APIService/PushPathAccessRoles":
		owner, ok := buckets.BucketOwnerFromContext(ctx)
		if ok {
			_, err := t.bc.IncStoredData(ctx, account.Owner().CustomerID, owner.StorageDelta)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
