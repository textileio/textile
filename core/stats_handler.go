package core

import (
	"context"
	"time"

	tpb "github.com/textileio/go-threads/api/pb"
	hpb "github.com/textileio/textile/v2/api/hubd/pb"
	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc/stats"
)

type StatsHandler struct {
	t *Textile
}

var _ stats.Handler = (*StatsHandler)(nil)

var statsTimeout = time.Second * 10

// HandleRPC accounts for customer usage across services.
func (h *StatsHandler) HandleRPC(ctx context.Context, st stats.RPCStats) {
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
			// @todo: Consolidate these to one IncStats call.
			if rs.egress > 0 {
				if _, err := h.t.bc.IncNetworkEgress(ctx, rs.customerID, rs.egress); err != nil {
					log.Errorf("stats: inc network egress: %v", err)
				}
			}
			if rs.reads > 0 {
				if _, err := h.t.bc.IncInstanceReads(ctx, rs.customerID, rs.reads); err != nil {
					log.Errorf("stats: inc instance reads: %v", err)
				}
			}
			if rs.writes > 0 {
				if _, err := h.t.bc.IncInstanceWrites(ctx, rs.customerID, rs.writes); err != nil {
					log.Errorf("stats: inc instance writes: %v", err)
				}
			}
		}()
	}
}

type statsCtxKey string

type requestStats struct {
	customerID string
	egress     int64
	reads      int64
	writes     int64
}

func (h *StatsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	for _, ignored := range authIgnoredMethods {
		if info.FullMethodName == ignored {
			return ctx
		}
	}
	ctx, _ = h.t.newAuthCtx(ctx, info.FullMethodName, false)
	account, ok := mdb.AccountFromContext(ctx)
	if !ok || account.Owner().CustomerID == "" {
		return ctx
	}
	return context.WithValue(ctx, statsCtxKey("requestStats"), &requestStats{
		customerID: account.Owner().CustomerID,
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
