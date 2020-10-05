package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/textileio/go-threads/core/thread"
	powc "github.com/textileio/powergate/api/client"
	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func powInterceptor(serviceName string, allowedMethods []string, serviceDesc *desc.ServiceDescriptor, stub *grpcdynamic.Stub, pc *powc.Client, c *mdb.Collections) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		methodParts := strings.Split(info.FullMethod, "/")
		if len(methodParts) != 3 {
			return nil, status.Errorf(codes.Internal, "error parsing method string %s", info.FullMethod)
		}
		methodServiceName := methodParts[1]
		methodName := methodParts[2]

		if methodServiceName != serviceName {
			return handler(ctx, req)
		}

		if pc == nil {
			return nil, status.Error(codes.Internal, "Powergate isn't enabled in Hub")
		}

		isAllowedMethod := false
		for _, method := range allowedMethods {
			if method == methodName {
				isAllowedMethod = true
				break
			}
		}

		if !isAllowedMethod {
			return nil, status.Errorf(codes.PermissionDenied, "method not allowed: %s", info.FullMethod)
		}

		var powInfo *mdb.PowInfo
		var owner thread.PubKey
		var isAccount bool
		if org, ok := mdb.OrgFromContext(ctx); ok {
			powInfo = org.PowInfo
			owner = org.Key
			isAccount = true
		} else if dev, ok := mdb.DevFromContext(ctx); ok {
			powInfo = dev.PowInfo
			owner = dev.Key
			isAccount = true
		} else if user, ok := mdb.UserFromContext(ctx); ok {
			powInfo = user.PowInfo
			owner = user.Key
			isAccount = false
		}

		createNewFFS := func() error {
			id, token, err := pc.FFS.Create(ctx)
			if err != nil {
				return fmt.Errorf("creating new powergate integration: %v", err)
			}
			if isAccount {
				_, err = c.Accounts.UpdatePowInfo(ctx, owner, &mdb.PowInfo{ID: id, Token: token})
			} else {
				_, err = c.Users.UpdatePowInfo(ctx, owner, &mdb.PowInfo{ID: id, Token: token})
			}
			if err != nil {
				return fmt.Errorf("updating user/account with new powergate information: %v", err)
			}
			return nil
		}

		tryAgain := fmt.Errorf("powergate newly integrated into your account, please try again in 30 seconds to allow time for setup to complete")

		// case where account/user was created before powergate was enabled.
		// create a ffs instance for them.
		if powInfo == nil {
			if err := createNewFFS(); err != nil {
				return nil, err
			}
			return nil, tryAgain
		}

		ffsCtx := context.WithValue(ctx, powc.AuthKey, powInfo.Token)

		methodDesc := serviceDesc.FindMethodByName(methodName)
		if methodDesc == nil {
			return nil, status.Errorf(codes.Internal, "no method found for %s", methodName)
		}

		res, err := stub.InvokeRpc(ffsCtx, methodDesc, req.(proto.Message))
		if err != nil {
			if !strings.Contains(err.Error(), "auth token not found") {
				return nil, err
			} else {
				// case where the ffs token is no longer valid because powergate was reset.
				// create a new ffs instance for them.
				if err := createNewFFS(); err != nil {
					return nil, err
				}
				return nil, tryAgain
			}
		}
		return res, nil
	}
}

func createPowStub(target string) (*grpcdynamic.Stub, error) {
	pgConn, err := powc.CreateClientConn(target)
	if err != nil {
		return &grpcdynamic.Stub{}, err
	}
	s := grpcdynamic.NewStubWithMessageFactory(pgConn, dynamic.NewMessageFactoryWithDefaults())
	return &s, nil
}
