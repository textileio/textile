package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	powc "github.com/textileio/powergate/v2/api/client"
	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// powInterceptor handles hubd-proxied Powergate requests.
func powInterceptor(
	serviceName string,
	allowedMethods []string,
	serviceDesc *desc.ServiceDescriptor,
	stub *grpcdynamic.Stub,
	pc *powc.Client,
	powAdminToken string,
	c *mdb.Collections,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
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

		account, ok := mdb.AccountFromContext(ctx)
		if !ok {
			// Should not happen at this point in the interceptor chain
			return nil, status.Errorf(codes.FailedPrecondition, "account is required")
		}

		createNewUser := func() error {
			ctxAdmin := context.WithValue(ctx, powc.AdminKey, powAdminToken)
			res, err := pc.Admin.Users.Create(ctxAdmin)
			if err != nil {
				return fmt.Errorf("creating new powergate integration: %v", err)
			}
			_, err = c.Accounts.UpdatePowInfo(ctx, account.Owner().Key, &mdb.PowInfo{
				ID:    res.User.Id,
				Token: res.User.Token,
			})
			if err != nil {
				return fmt.Errorf("updating user/account with new powergate information: %v", err)
			}
			return nil
		}

		tryAgain := fmt.Errorf("powergate newly integrated into your account, " +
			"please try again in 30 seconds to allow time for setup to complete")

		// Case where account/user was created before powergate was enabled.
		// create a user for them.
		if account.Owner().PowInfo == nil {
			if err := createNewUser(); err != nil {
				return nil, err
			}
			return nil, tryAgain
		}

		powCtx := context.WithValue(ctx, powc.AuthKey, account.Owner().PowInfo.Token)

		methodDesc := serviceDesc.FindMethodByName(methodName)
		if methodDesc == nil {
			return nil, status.Errorf(codes.Internal, "no method found for %s", methodName)
		}

		res, err := stub.InvokeRpc(powCtx, methodDesc, req.(proto.Message))
		if err != nil {
			if !strings.Contains(err.Error(), "auth token not found") {
				return nil, err
			} else {
				// case where the auth token is no longer valid because powergate was reset.
				// create a new user for them.
				if err := createNewUser(); err != nil {
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
