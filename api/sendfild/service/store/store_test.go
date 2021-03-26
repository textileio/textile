package store

import (
	"context"
	"os"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-ds-mongo/test"
	powutil "github.com/textileio/powergate/v2/util"
	"github.com/textileio/textile/v2/api/sendfild/pb"
	"github.com/textileio/textile/v2/util"
)

const oneFil = int64(1000000000)

var (
	ctx = context.Background()
)

func TestMain(m *testing.M) {
	powutil.AvgBlockTime = time.Millisecond * 100
	logging.SetAllLoggers(logging.LevelError)

	cleanup := test.StartMongoDB()
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

func TestNewTxn(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid", "from", "to", oneFil)
}

func TestGetTxn(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	txn := requireNewTxn(t, ctx, s, "cid", "from", "to", oneFil)
	res := requireGetTxn(t, ctx, s, txn.MessageCid)
	require.Equal(t, txn.MessageCid, res.MessageCid)
	require.Equal(t, txn.From, res.From)
	require.Equal(t, txn.To, res.To)
	require.Equal(t, txn.AmountNanoFil, res.AmountNanoFil)
	require.Equal(t, txn.Id, res.Id)
}

func TestGetAllPendingAll(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "from", "to", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "from", "to", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "from", "to", oneFil)
	res, err := s.GetAllPending(ctx, false)
	require.NoError(t, err)
	require.Len(t, res, 3)
}

func TestGetAllPendingExcludeWaiting(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "from", "to", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "from", "to", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "from", "to", oneFil)
	requireSetWaiting(t, ctx, s, "cid3", true)
	res, err := s.GetAllPending(ctx, true)
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestSetWaiting(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	txn := requireNewTxn(t, ctx, s, "cid1", "from", "to", oneFil)
	require.Equal(t, false, txn.Waiting)
	requireSetWaiting(t, ctx, s, txn.MessageCid, true)
	txn = requireGetTxn(t, ctx, s, txn.MessageCid)
	require.Equal(t, true, txn.Waiting)
	requireSetWaiting(t, ctx, s, txn.MessageCid, false)
	txn = requireGetTxn(t, ctx, s, txn.MessageCid)
	require.Equal(t, false, txn.Waiting)
}

func TestFail(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	txn := requireNewTxn(t, ctx, s, "cid1", "from", "to", oneFil)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_PENDING, txn.MessageState)
	require.Equal(t, false, txn.Waiting)
	requireSetWaiting(t, ctx, s, txn.MessageCid, true)
	txn = requireGetTxn(t, ctx, s, txn.MessageCid)
	require.Equal(t, true, txn.Waiting)
	err := s.Fail(ctx, txn.MessageCid, "oops")
	require.NoError(t, err)
	txn = requireGetTxn(t, ctx, s, txn.MessageCid)
	require.Equal(t, false, txn.Waiting)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_FAILED, txn.MessageState)
	require.Equal(t, false, txn.Waiting)
	require.Equal(t, "oops", txn.FailureMsg)
}

func TestActivate(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	txn := requireNewTxn(t, ctx, s, "cid1", "from", "to", oneFil)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_PENDING, txn.MessageState)
	require.Equal(t, false, txn.Waiting)
	requireSetWaiting(t, ctx, s, txn.MessageCid, true)
	txn = requireGetTxn(t, ctx, s, txn.MessageCid)
	require.Equal(t, true, txn.Waiting)
	err := s.Activate(ctx, txn.MessageCid, txn.MessageCid)
	require.NoError(t, err)
	txn = requireGetTxn(t, ctx, s, txn.MessageCid)
	require.Equal(t, false, txn.Waiting)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_ACTIVE, txn.MessageState)
	require.Equal(t, false, txn.Waiting)
}

func TestList(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "from", "to", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "from", "to", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "from", "to", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{})
	require.NoError(t, err)
	require.Len(t, res, 3)
}

func TestListMessageCids(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	t1 := requireNewTxn(t, ctx, s, "cid1", "from", "to", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "from", "to", oneFil)
	t3 := requireNewTxn(t, ctx, s, "cid3", "from", "to", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{MessageCidsFilter: []string{t1.MessageCid, t3.MessageCid}, Ascending: true})
	require.NoError(t, err)
	require.Len(t, res, 2)
	require.Equal(t, t1.MessageCid, res[0].MessageCid)
	require.Equal(t, t3.MessageCid, res[1].MessageCid)
}

func TestListFrom(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid4", "addr2", "addr1", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{FromFilter: "addr1"})
	require.NoError(t, err)
	require.Len(t, res, 3)
	res, err = s.List(ctx, &pb.ListTxnsRequest{FromFilter: "addr2"})
	require.NoError(t, err)
	require.Len(t, res, 1)
}

func TestListTo(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid4", "addr2", "addr1", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{ToFilter: "addr1"})
	require.NoError(t, err)
	require.Len(t, res, 1)
	res, err = s.List(ctx, &pb.ListTxnsRequest{ToFilter: "addr2"})
	require.NoError(t, err)
	require.Len(t, res, 3)
}

func TestListInvolvingAddress(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr3", oneFil)
	requireNewTxn(t, ctx, s, "cid4", "addr3", "addr2", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{InvolvingAddressFilter: "addr1"})
	require.NoError(t, err)
	require.Len(t, res, 3)
	res, err = s.List(ctx, &pb.ListTxnsRequest{InvolvingAddressFilter: "addr2"})
	require.NoError(t, err)
	require.Len(t, res, 3)
	res, err = s.List(ctx, &pb.ListTxnsRequest{InvolvingAddressFilter: "addr3"})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListAmtGt(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil/2)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil*2)
	res, err := s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil})
	require.NoError(t, err)
	require.Len(t, res, 1)
	res, err = s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil * 2})
	require.NoError(t, err)
	require.Len(t, res, 0)
}

func TestListAmtGteq(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil/2)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil*2)
	res, err := s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil})
	require.NoError(t, err)
	require.Len(t, res, 2)
	res, err = s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil * 3})
	require.NoError(t, err)
	require.Len(t, res, 0)
}

func TestListAmtLt(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil/2)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil*2)
	res, err := s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilLtFilter: oneFil})
	require.NoError(t, err)
	require.Len(t, res, 1)
	res, err = s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilLtFilter: oneFil / 2})
	require.NoError(t, err)
	require.Len(t, res, 0)
}

func TestListAmtLteq(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil/2)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil*2)
	res, err := s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilLteqFilter: oneFil})
	require.NoError(t, err)
	require.Len(t, res, 2)
	res, err = s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilLteqFilter: oneFil / 3})
	require.NoError(t, err)
	require.Len(t, res, 0)
}

func TestListAmtGtLt(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil/2)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil*2)
	res, err := s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil / 2, AmountNanoFilLtFilter: oneFil * 3 / 2})
	require.NoError(t, err)
	require.Len(t, res, 1)
}

func TestListAmtGteqLteq(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil/2)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil*2)
	requireNewTxn(t, ctx, s, "cid4", "addr1", "addr2", oneFil*3)
	res, err := s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil, AmountNanoFilLteqFilter: oneFil * 2})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListAmtGteqLt(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil/2)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil*2)
	res, err := s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil / 2, AmountNanoFilLtFilter: oneFil * 3 / 2})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListAmtGtLteq(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil/2)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil*2)
	requireNewTxn(t, ctx, s, "cid4", "addr1", "addr2", oneFil*3)
	res, err := s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil, AmountNanoFilLteqFilter: oneFil * 2})
	require.NoError(t, err)
	require.Len(t, res, 1)
}

func TestListAmtEq(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil/2)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil*2)
	requireNewTxn(t, ctx, s, "cid4", "addr1", "addr2", oneFil*3)
	res, err := s.List(ctx, &pb.ListTxnsRequest{AmountNanoFilEqFilter: oneFil})
	require.NoError(t, err)
	require.Len(t, res, 1)
}

func TestListMessageState(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr3", oneFil)
	err := s.Activate(ctx, "cid1", "cid1")
	require.NoError(t, err)
	res, err := s.List(ctx, &pb.ListTxnsRequest{MessageStateFilter: pb.MessageState_MESSAGE_STATE_ACTIVE})
	require.NoError(t, err)
	require.Len(t, res, 1)
	res, err = s.List(ctx, &pb.ListTxnsRequest{MessageStateFilter: pb.MessageState_MESSAGE_STATE_PENDING})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListWaiting(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr3", oneFil)
	err := s.SetWaiting(ctx, "cid1", true)
	require.NoError(t, err)
	res, err := s.List(ctx, &pb.ListTxnsRequest{WaitingFilter: pb.WaitingFilter_WAITING_FILTER_NOT_WAITING})
	require.NoError(t, err)
	require.Len(t, res, 2)
	res, err = s.List(ctx, &pb.ListTxnsRequest{WaitingFilter: pb.WaitingFilter_WAITING_FILTER_WAITING})
	require.NoError(t, err)
	require.Len(t, res, 1)
}

func TestListCreatedAfter(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	t1 := requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{CreatedAfter: t1.CreatedAt})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListCreatedBefore(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	t3 := requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{CreatedBefore: t3.CreatedAt})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListCreatedAfterBefore(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	t1 := requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	t3 := requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{CreatedAfter: t1.CreatedAt, CreatedBefore: t3.CreatedAt})
	require.NoError(t, err)
	require.Len(t, res, 1)
}

func TestListUpdatedAfter(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	t1 := requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{UpdatedAfter: t1.UpdatedAt})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListUpdatedBefore(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	t3 := requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{UpdatedBefore: t3.UpdatedAt})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListUpdatedAfterBefore(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	t1 := requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	t3 := requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{UpdatedAfter: t1.UpdatedAt, UpdatedBefore: t3.UpdatedAt})
	require.NoError(t, err)
	require.Len(t, res, 1)
}

func TestListOrder(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil)
	res, err := s.List(ctx, &pb.ListTxnsRequest{Ascending: false})
	require.NoError(t, err)
	require.Len(t, res, 3)
	requireTxnsOrder(t, res, false)
	res, err = s.List(ctx, &pb.ListTxnsRequest{Ascending: true})
	require.NoError(t, err)
	require.Len(t, res, 3)
	requireTxnsOrder(t, res, true)
}

func TestListPaging(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	txFirst := requireNewTxn(t, ctx, s, "cid1", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid4", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid5", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid6", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid7", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid8", "addr1", "addr2", oneFil)
	requireNewTxn(t, ctx, s, "cid9", "addr1", "addr2", oneFil)
	txLast := requireNewTxn(t, ctx, s, "cid10", "addr1", "addr2", oneFil)

	pageResults := func(ascending bool) {
		page := int64(0)
		for {
			req := &pb.ListTxnsRequest{
				CreatedAfter:  txFirst.CreatedAt,
				CreatedBefore: txLast.CreatedAt,
				PageSize:      3,
				Page:          page,
				Ascending:     ascending,
			}
			res, err := s.List(ctx, req)
			require.NoError(t, err)
			if len(res) == 0 {
				break
			}
			requireTxnsOrder(t, res, ascending)
			if page < 2 {
				require.Len(t, res, 3)
			}
			if page == 2 {
				require.Len(t, res, 2)
			}
			page++
		}
		require.Equal(t, int64(3), page)
	}
	pageResults(false)
	pageResults(true)
}

func TestSummary(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	first := requireNewTxn(t, ctx, s, "cid0", "from1", "to1", oneFil)
	requireNewTxn(t, ctx, s, "cid1", "from1", "to1", oneFil)
	requireNewTxn(t, ctx, s, "cid2", "from1", "to1", oneFil)
	requireNewTxn(t, ctx, s, "cid3", "from1", "to1", oneFil)
	requireNewTxn(t, ctx, s, "cid4", "from2", "to1", oneFil)
	requireNewTxn(t, ctx, s, "cid5", "from3", "to2", oneFil*6)
	last := requireNewTxn(t, ctx, s, "cid6", "from1", "to1", oneFil)
	err := s.SetWaiting(ctx, "cid1", true)
	require.NoError(t, err)
	err = s.Activate(ctx, "cid2", "cid2")
	require.NoError(t, err)
	err = s.Fail(ctx, "cid3", "oops")
	require.NoError(t, err)
	res, err := s.Summary(ctx, first.CreatedAt.AsTime(), last.CreatedAt.AsTime())
	require.NoError(t, err)
	require.Equal(t, float64(oneFil*2), res.AvgNanoFilSent)
	require.Equal(t, int64(1), res.CountActive)
	require.Equal(t, int64(1), res.CountFailed)
	require.Equal(t, int64(3), res.CountFromAddrs)
	require.Equal(t, int64(3), res.CountPending)
	require.Equal(t, int64(2), res.CountToAddrs)
	require.Equal(t, int64(5), res.CountTxns)
	require.Equal(t, int64(1), res.CountWaiting)
	require.Equal(t, oneFil*6, res.MaxNanoFilSent)
	require.Equal(t, oneFil, res.MinNanoFilSent)
	require.Equal(t, oneFil*10, res.TotalNanoFilSent)
}

func requireNewTxn(t *testing.T, ctx context.Context, s *Store, cid, from, to string, amt int64) *pb.Txn {
	res, err := s.New(ctx, cid, from, to, amt)
	require.NoError(t, err)
	require.Equal(t, amt, res.AmountNanoFil)
	require.Equal(t, from, res.From)
	require.Equal(t, to, res.To)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_PENDING, res.MessageState)
	require.Equal(t, cid, res.MessageCid)
	return res
}

func requireGetTxn(t *testing.T, ctx context.Context, s *Store, msgCid string) *pb.Txn {
	res, err := s.Get(ctx, msgCid)
	require.NoError(t, err)
	require.Equal(t, msgCid, res.MessageCid)
	return res
}

func requireSetWaiting(t *testing.T, ctx context.Context, s *Store, cid string, waiting bool) {
	err := s.SetWaiting(ctx, cid, waiting)
	require.NoError(t, err)
}

func requireSetup(t *testing.T) (*Store, func()) {
	s, err := New(test.GetMongoUri(), util.MakeToken(12), true)
	require.NoError(t, err)

	cleanup := func() {
		s.Close()
	}

	return s, cleanup
}

func requireTxnsOrder(t *testing.T, txns []*pb.Txn, ascending bool) {
	var last *time.Time
	for _, txn := range txns {
		if last != nil {
			a := *last
			b := txn.CreatedAt.AsTime()
			if ascending {
				a = txn.CreatedAt.AsTime()
				b = *last
			}
			require.True(t, a.After(b))
		}
		t := txn.CreatedAt.AsTime()
		last = &t
	}
}
