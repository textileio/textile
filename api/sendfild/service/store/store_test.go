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

var (
	ctx = context.Background()
)

func TestMain(m *testing.M) {
	powutil.AvgBlockTime = time.Millisecond * 100
	logging.SetAllLoggers(logging.LevelError)

	cleanup := func() {}
	if os.Getenv("SKIP_SERVICES") != "true" {
		cleanup = test.StartMongoDB()
	}
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

func TestNewTxn(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid")
}

func TestGetTxn(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	txn := requireNewTxn(t, ctx, s, "cid")
	txnCid := requireLatestCid(t, txn)
	res := requireGetTxn(t, ctx, s, txnCid)
	resCid := requireLatestCid(t, res)
	require.Equal(t, txnCid, resCid)
	require.Equal(t, txn.From, res.From)
	require.Equal(t, txn.To, res.To)
	require.Equal(t, txn.AmountNanoFil, res.AmountNanoFil)
	require.Equal(t, txn.ID, res.ID)
}

func TestGetAllPendingAll(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1")
	requireNewTxn(t, ctx, s, "cid2")
	requireNewTxn(t, ctx, s, "cid3")
	res, err := s.GetAllPending(ctx, false)
	require.NoError(t, err)
	require.Len(t, res, 3)
}

func TestGetAllPendingExcludeWaiting(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1")
	requireNewTxn(t, ctx, s, "cid2")
	requireNewTxn(t, ctx, s, "cid3")
	requireSetWaiting(t, ctx, s, "cid3", true)
	res, err := s.GetAllPending(ctx, true)
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestSetWaiting(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	txn := requireNewTxn(t, ctx, s, "cid1")
	require.Equal(t, false, txn.Waiting)
	c := requireLatestCid(t, txn)
	requireSetWaiting(t, ctx, s, c, true)
	txn = requireGetTxn(t, ctx, s, c)
	require.Equal(t, true, txn.Waiting)
	requireSetWaiting(t, ctx, s, c, false)
	txn = requireGetTxn(t, ctx, s, c)
	require.Equal(t, false, txn.Waiting)
}

func TestFailTxn(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	txn := requireNewTxn(t, ctx, s, "cid1")
	require.Equal(t, pb.MessageState_MESSAGE_STATE_PENDING, txn.MessageState)
	require.Equal(t, false, txn.Waiting)
	c := requireLatestCid(t, txn)
	requireSetWaiting(t, ctx, s, c, true)
	txn = requireGetTxn(t, ctx, s, c)
	require.Equal(t, true, txn.Waiting)
	err := s.FailTxn(ctx, c, "oops")
	require.NoError(t, err)
	txn = requireGetTxn(t, ctx, s, c)
	require.Equal(t, false, txn.Waiting)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_FAILED, txn.MessageState)
	require.Equal(t, false, txn.Waiting)
	require.Equal(t, "oops", txn.FailureMsg)
}

func TestActivateTxn(t *testing.T) {
	s, cleanup := requireSetup(t)
	defer cleanup()
	txn := requireNewTxn(t, ctx, s, "cid1")
	require.Equal(t, pb.MessageState_MESSAGE_STATE_PENDING, txn.MessageState)
	require.Equal(t, false, txn.Waiting)
	c := requireLatestCid(t, txn)
	requireSetWaiting(t, ctx, s, c, true)
	txn = requireGetTxn(t, ctx, s, c)
	require.Equal(t, true, txn.Waiting)
	err := s.ActivateTxn(ctx, c, c)
	require.NoError(t, err)
	txn = requireGetTxn(t, ctx, s, c)
	require.Equal(t, false, txn.Waiting)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_ACTIVE, txn.MessageState)
	require.Equal(t, false, txn.Waiting)
}

func TestListTxns(t *testing.T) {
	// This is more thoroughly tested with all filters in service_test.go.
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1")
	requireNewTxn(t, ctx, s, "cid2")
	requireNewTxn(t, ctx, s, "cid3")
	res, err := s.ListTxns(ctx, &pb.ListTxnsRequest{})
	require.NoError(t, err)
	require.Len(t, res, 3)
}

func TestGenerateSummary(t *testing.T) {
	// This is more thoroughly tested in service_test.go.
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1")
	requireNewTxn(t, ctx, s, "cid2")
	requireNewTxn(t, ctx, s, "cid3")
	res, err := s.GenerateSummary(ctx, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, res.All, 1)
	require.Equal(t, int64(3), res.All[0].Count)
}

func requireNewTxn(t *testing.T, ctx context.Context, s *Store, cid string) *Txn {
	res, err := s.NewTxn(ctx, cid, "from", "to", 100)
	require.NoError(t, err)
	require.Equal(t, int64(100), res.AmountNanoFil)
	require.Equal(t, "from", res.From)
	require.Equal(t, "to", res.To)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_PENDING, res.MessageState)
	c := requireLatestCid(t, res)
	require.Equal(t, cid, c)
	return res
}

func requireGetTxn(t *testing.T, ctx context.Context, s *Store, msgCid string) *Txn {
	res, err := s.GetTxn(ctx, msgCid)
	require.NoError(t, err)
	resCid := requireLatestCid(t, res)
	require.Equal(t, msgCid, resCid)
	return res
}

func requireSetWaiting(t *testing.T, ctx context.Context, s *Store, cid string, waiting bool) {
	err := s.SetWaiting(ctx, cid, waiting)
	require.NoError(t, err)
}

func requireLatestCid(t *testing.T, txn *Txn) string {
	c, err := txn.LatestMsgCid()
	require.NoError(t, err)
	return c.Cid
}

func requireSetup(t *testing.T) (*Store, func()) {
	s, err := New(test.GetMongoUri(), util.MakeToken(12), true)
	require.NoError(t, err)

	cleanup := func() {
		s.Close()
	}

	return s, cleanup
}
