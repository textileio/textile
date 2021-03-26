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

	cleanup := test.StartMongoDB()
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
	txn := requireNewTxn(t, ctx, s, "cid1")
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
	txn := requireNewTxn(t, ctx, s, "cid1")
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

func TestListTxns(t *testing.T) {
	// ToDo: Test this more thuroughtly since it will be mocked in service_test.go
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1")
	requireNewTxn(t, ctx, s, "cid2")
	requireNewTxn(t, ctx, s, "cid3")
	res, err := s.List(ctx, &pb.ListTxnsRequest{})
	require.NoError(t, err)
	require.Len(t, res, 3)
}

// func TestListTxnsMessageCids(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr := requireLotusAddress(t, ctx, lc)
// 	t1 := requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
// 	t3 := requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{MessageCidsFilter: []string{t1.MessageCid, t3.MessageCid}, Ascending: true})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// 	require.Equal(t, t1.MessageCid, res.Txns[0].MessageCid)
// 	require.Equal(t, t3.MessageCid, res.Txns[1].MessageCid)
// }

// func TestListTxnsFrom(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	addr2 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr2.String(), oneFil*2, true)
// 	requireSendFil(t, ctx, c, addr2.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{FromFilter: dAddr.String()})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 3)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{FromFilter: addr2.String()})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// }

// func TestListTxnsTo(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	addr2 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr2.String(), oneFil*2, true)
// 	requireSendFil(t, ctx, c, addr2.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{ToFilter: addr1.String()})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 3)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{ToFilter: addr2.String()})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// }

// func TestListTxnsInvolvingAddress(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	addr2 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr2.String(), oneFil*2, true)
// 	requireSendFil(t, ctx, c, addr2.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{InvolvingAddressFilter: dAddr.String()})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 3)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{InvolvingAddressFilter: addr1.String()})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 3)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{InvolvingAddressFilter: addr2.String()})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// }

// func TestListTxnsAmtGt(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil * 2})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 0)
// }

// func TestListTxnsAmtGteq(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil * 3})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 0)
// }

// func TestListTxnsAmtLt(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilLtFilter: oneFil})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilLtFilter: oneFil / 2})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 0)
// }

// func TestListTxnsAmtLteq(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilLteqFilter: oneFil})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilLteqFilter: oneFil / 3})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 0)
// }

// func TestListTxnsAmtGtLt(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil / 2, AmountNanoFilLtFilter: oneFil * 3 / 2})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// }

// func TestListTxnsAmtGteqLteq(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*3, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil, AmountNanoFilLteqFilter: oneFil * 2})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// }

// func TestListTxnsAmtGteqLt(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil / 2, AmountNanoFilLtFilter: oneFil * 3 / 2})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// }

// func TestListTxnsAmtGtLteq(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*3, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil, AmountNanoFilLteqFilter: oneFil * 2})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// }

// func TestListTxnsAmtEq(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*3, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilEqFilter: oneFil})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// }

// func TestListTxnsMessageState(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx, setupWithSpeed(1000))
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, true)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{MessageStateFilter: pb.MessageState_MESSAGE_STATE_ACTIVE})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{MessageStateFilter: pb.MessageState_MESSAGE_STATE_PENDING})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// }

// func TestListTxnsWaiting(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx, setupWithSpeed(1000))
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, true)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	time.Sleep(time.Millisecond * 100) // Little time to allow the monitoring process to start waiting for all the txns.
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{WaitingFilter: pb.WaitingFilter_WAITING_FILTER_NOT_WAITING})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{WaitingFilter: pb.WaitingFilter_WAITING_FILTER_WAITING})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// }

// func TestListTxnsCreatedAfter(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	t1 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{CreatedAfter: t1.CreatedAt})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// }

// func TestListTxnsCreatedBefore(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	t3 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{CreatedBefore: t3.CreatedAt})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// }

// func TestListTxnsCreatedAfterBefore(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	t1 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	t3 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{CreatedAfter: t1.CreatedAt, CreatedBefore: t3.CreatedAt})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// }

// func TestListTxnsUpdatedAfter(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	t1 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{UpdatedAfter: t1.UpdatedAt})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// }

// func TestListTxnsUpdatedBefore(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	t3 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{UpdatedBefore: t3.UpdatedAt})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 2)
// }

// func TestListTxnsUpdatedAfterBefore(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	t1 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	t3 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{UpdatedAfter: t1.UpdatedAt, UpdatedBefore: t3.UpdatedAt})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 1)
// }

// func TestListTxnsOrder(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{Ascending: false})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 3)
// 	requireTxnsOrder(t, res.Txns, false)
// 	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{Ascending: true})
// 	require.NoError(t, err)
// 	require.Len(t, res.Txns, 3)
// 	requireTxnsOrder(t, res.Txns, true)
// }

// func TestListTxnsPaging(t *testing.T) {
// 	c, lc, dAddr, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	addr1 := requireLotusAddress(t, ctx, lc)
// 	txFirst := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
// 	txLast := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)

// 	pageResults := func(ascending bool) {
// 		page := int64(0)
// 		for {
// 			req := &pb.ListTxnsRequest{
// 				CreatedAfter:  txFirst.CreatedAt,
// 				CreatedBefore: txLast.CreatedAt,
// 				PageSize:      3,
// 				Page:          page,
// 				Ascending:     ascending,
// 			}
// 			res, err := c.ListTxns(ctx, req)
// 			require.NoError(t, err)
// 			if len(res.Txns) == 0 {
// 				break
// 			}
// 			requireTxnsOrder(t, res.Txns, ascending)
// 			if page < 2 {
// 				require.Len(t, res.Txns, 3)
// 			}
// 			if page == 2 {
// 				require.Len(t, res.Txns, 2)
// 			}
// 			page++
// 		}
// 		require.Equal(t, int64(3), page)
// 	}
// 	pageResults(false)
// 	pageResults(true)
// }

func TestSummary(t *testing.T) {
	// ToDo: Test this more thuroughtly since it will be mocked in service_test.go
	s, cleanup := requireSetup(t)
	defer cleanup()
	requireNewTxn(t, ctx, s, "cid1")
	requireNewTxn(t, ctx, s, "cid2")
	requireNewTxn(t, ctx, s, "cid3")
	res, err := s.Summary(ctx, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Equal(t, int64(3), res.CountTxns)
}

func requireNewTxn(t *testing.T, ctx context.Context, s *Store, cid string) *pb.Txn {
	res, err := s.New(ctx, cid, "from", "to", 100)
	require.NoError(t, err)
	require.Equal(t, int64(100), res.AmountNanoFil)
	require.Equal(t, "from", res.From)
	require.Equal(t, "to", res.To)
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
