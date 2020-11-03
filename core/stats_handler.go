package core

import (
	"context"

	tpb "github.com/textileio/go-threads/api/pb"
	hpb "github.com/textileio/textile/v2/api/hubd/pb"
	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc/stats"
)

type StatsHandler struct {
	t *Textile
}

var _ stats.Handler = (*StatsHandler)(nil)

// HandleRPC accounts for customer usage across services.
func (h *StatsHandler) HandleRPC(ctx context.Context, st stats.RPCStats) {
	switch st := st.(type) {
	case *stats.OutPayload:
		var account *mdb.Account
		if org, ok := mdb.OrgFromContext(ctx); ok {
			account = org
		} else if dev, ok := mdb.DevFromContext(ctx); ok {
			account = dev
		}
		// @todo: Account for users after User -> Account migration
		if account == nil || account.CustomerID == "" {
			return
		}

		recordEgress := true
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
			recordEgress = false

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
		case *tpb.ListenReply:
			if pl.Instance != nil {
				reads = 1
			}
		}

		// Record usage
		go func() {
			if recordEgress {
				if _, err := h.t.bc.IncNetworkEgress(ctx, account.CustomerID, int64(st.WireLength)); err != nil {
					log.Errorf("stats: inc network egress: %v", err)
				}
			}
			if reads > 0 {
				if _, err := h.t.bc.IncInstanceReads(ctx, account.CustomerID, reads); err != nil {
					log.Errorf("stats: inc instance reads: %v", err)
				}
			} else if writes > 0 {
				if _, err := h.t.bc.IncInstanceWrites(ctx, account.CustomerID, writes); err != nil {
					log.Errorf("stats: inc instance writes: %v", err)
				}
			}
		}()
	}
}

func (h *StatsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	for _, ignored := range authIgnoredMethods {
		if info.FullMethodName == ignored {
			return ctx
		}
	}
	ctx, _ = h.t.newAuthCtx(ctx, info.FullMethodName, false)
	return ctx
}

func (h *StatsHandler) HandleConn(context.Context, stats.ConnStats) {}

func (h *StatsHandler) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}
