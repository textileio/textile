package core

import (
	"context"
	"time"

	tpb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/core/thread"
	hpb "github.com/textileio/textile/v2/api/hubd/pb"
	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc/stats"
)

type StatsHandler struct {
	t *Textile
}

var _ stats.Handler = (*StatsHandler)(nil)

var statsTimeout = time.Hour

// HandleRPC accounts for customer usage across services.
func (h *StatsHandler) HandleRPC(ctx context.Context, st stats.RPCStats) {
	if h.t.bc == nil {
		return
	}
	switch st := st.(type) {
	case *stats.OutPayload:
		if getStats(ctx) == nil {
			return
		}

		// Handle payload types.
		egress := int64(st.WireLength)
		var reads, writes int64
		var pl interface{}
		switch spl := st.Payload.(type) {
		case *tpb.ReadTransactionReply:
			pl = spl.Option
		case *tpb.WriteTransactionReply:
			pl = spl.Option
		default:
			pl = spl
		}
		switch pl := pl.(type) {

		// Don't charge for usage when customer is trying to add a payment method, cancel subscription, etc.
		case *hpb.GetBillingSessionResponse:
			egress = 0

		// Account for threaddb reads and writes
		case *tpb.CreateReply:
			if pl.InstanceIDs != nil {
				writes = int64(len(pl.InstanceIDs))
			}
		case *tpb.WriteTransactionReply_CreateReply:
			if pl.CreateReply.TransactionError == "" {
				writes = 1
			}
		case *tpb.VerifyReply:
			if pl.TransactionError == "" {
				reads = 1
			}
		case *tpb.WriteTransactionReply_VerifyReply:
			if pl.VerifyReply.TransactionError == "" {
				reads = 1
			}
		case *tpb.SaveReply:
			if pl.TransactionError == "" {
				writes = 1
			}
		case *tpb.WriteTransactionReply_SaveReply:
			if pl.SaveReply.TransactionError == "" {
				writes = 1
			}
		case *tpb.DeleteReply:
			if pl.TransactionError == "" {
				writes = 1
			}
		case *tpb.WriteTransactionReply_DeleteReply:
			if pl.DeleteReply.TransactionError == "" {
				writes = 1
			}
		case *tpb.HasReply:
			if pl.TransactionError == "" {
				reads = 1
			}
		case *tpb.ReadTransactionReply_HasReply:
			if pl.HasReply.TransactionError == "" {
				reads = 1
			}
		case *tpb.WriteTransactionReply_HasReply:
			if pl.HasReply.TransactionError == "" {
				reads = 1
			}
		case *tpb.FindReply:
			if pl.Instances != nil {
				reads = int64(len(pl.Instances))
			}
		case *tpb.ReadTransactionReply_FindReply:
			if pl.FindReply.Instances != nil {
				reads = int64(len(pl.FindReply.Instances))
			}
		case *tpb.WriteTransactionReply_FindReply:
			if pl.FindReply.Instances != nil {
				reads = int64(len(pl.FindReply.Instances))
			}
		case *tpb.FindByIDReply:
			if pl.TransactionError == "" {
				reads = 1
			}
		case *tpb.ReadTransactionReply_FindByIDReply:
			if pl.FindByIDReply.TransactionError == "" {
				reads = 1
			}
		case *tpb.WriteTransactionReply_FindByIDReply:
			if pl.FindByIDReply.TransactionError == "" {
				reads = 1
			}
		case *tpb.ListenReply:
			if pl.Instance != nil {
				reads = 1
			}
		}
		ctx = handleStats(ctx, egress, reads, writes)

	case *stats.End:
		// Record usage
		rs := getStats(ctx)
		if rs == nil {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), statsTimeout)
			defer cancel()
			if rs.egress > 0 || rs.reads > 0 || rs.writes > 0 {
				if _, err := h.t.bc.IncCustomerUsage(
					ctx,
					rs.key,
					map[string]int64{
						"network_egress":  rs.egress,
						"instance_reads":  rs.reads,
						"instance_writes": rs.writes,
					},
				); err != nil {
					log.Errorf("stats: inc customer usage: %v", err)
				}
			}
		}()
	}
}

type statsCtxKey string

type requestStats struct {
	key    thread.PubKey
	egress int64
	reads  int64
	writes int64
}

func (h *StatsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	if h.t.bc == nil {
		return ctx
	}
	for _, ignored := range authIgnoredMethods {
		if info.FullMethodName == ignored {
			return ctx
		}
	}
	token, err := thread.NewTokenFromMD(ctx)
	if err != nil {
		return ctx
	} else {
		ctx = thread.NewTokenContext(ctx, token)
	}
	ctx, err = h.t.newAuthCtx(ctx, info.FullMethodName, false)
	if err != nil {
		return ctx
	}
	account, ok := mdb.AccountFromContext(ctx)
	if !ok {
		return ctx
	}
	return context.WithValue(ctx, statsCtxKey("requestStats"), &requestStats{
		key: account.Owner().Key,
	})
}

func handleStats(ctx context.Context, egress, reads, writes int64) context.Context {
	rs := getStats(ctx)
	if rs == nil {
		return ctx
	}
	rs.egress += egress
	rs.reads += reads
	rs.writes += writes
	return context.WithValue(ctx, statsCtxKey("requestStats"), rs)
}

func getStats(ctx context.Context) *requestStats {
	rs, _ := ctx.Value(statsCtxKey("requestStats")).(*requestStats)
	return rs
}

func (h *StatsHandler) HandleConn(context.Context, stats.ConnStats) {}

func (h *StatsHandler) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}
