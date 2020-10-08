package core

import (
	"context"
	"fmt"

	"google.golang.org/grpc/grpclog"

	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
)

type BillingFunc func(ctx context.Context) (context.Context, error)

func unaryBillingInterceptor(fn BillingFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}

func streamBillingInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

		method, _ := grpc.Method(stream.Context())

		switch method {
		case "/api.buckets.pb.APIService/PushPath":
			//switch req := req.(*bpb.PushPathRequest).Payload.(type) {
			//case *bpb.PushPathRequest_Chunk:
			//	size := len(req.Chunk)
			//	log.Warn("chunk %v", size)
			//default:
			//	break
			//}
		default:
			break
		}

		return handler(srv, stream)
	}
}

func (t *Textile) billingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		method, _ := grpc.Method(ctx)

		var account *mdb.Account
		if org, ok := mdb.OrgFromContext(ctx); ok {
			account = org
		} else if dev, ok := mdb.DevFromContext(ctx); ok {
			account = dev
		}

		res, err := handler(ctx, req)
		if err != nil {
			return nil, err
		}

		if account != nil {
			switch method {
			case "/api.buckets.pb.APIService/PushPath":
				//switch req := req.(*bpb.PushPathRequest).Payload.(type) {
				//case *bpb.PushPathRequest_Chunk:
				//	size := len(req.Chunk)
				//	log.Warn("chunk %v", size)
				//default:
				//	break
				//}
			case "/threads.pb.API/FindByID":
				//if res.(*tpb.FindByIDReply).Instance != nil {
				//	count := account.CustomerInfo.InstanceReads.BinCount + 1
				//	if count == 10000 {
				//		item := account.CustomerInfo.InstanceReads.ItemID
				//		if err := billing.ChargeInstanceReads(account.Key, item, 1); err != nil {
				//			return nil, err
				//		}
				//		count = 0
				//	}
				//}
			default:
				break
			}
		}

		return res, nil
	}
}

type BillingHandler struct{}

var _ stats.Handler = (*BillingHandler)(nil)

func (h *BillingHandler) HandleConn(context.Context, stats.ConnStats) {
	// no-op
}

func (h *BillingHandler) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}

func (h *BillingHandler) HandleRPC(_ context.Context, st stats.RPCStats) {
	switch st := st.(type) {
	case *stats.Begin, *stats.OutHeader, *stats.InHeader, *stats.InTrailer, *stats.OutTrailer:
		// do nothing for client
		fmt.Println(st)
	case *stats.OutPayload:
		fmt.Println(st)
	case *stats.InPayload:
		fmt.Println(st)
	case *stats.End:
		fmt.Println(st)
	default:
		grpclog.Infof("unexpected stats: %T", st)
	}
}

func (h *BillingHandler) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context {
	return ctx
}
