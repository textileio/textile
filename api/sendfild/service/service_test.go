package service

import (
	"context"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api/apistruct"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-ds-mongo/test"
	"github.com/textileio/powergate/v2/lotus"
	"github.com/textileio/powergate/v2/tests"
	powutil "github.com/textileio/powergate/v2/util"
	pb "github.com/textileio/textile/v2/api/sendfild/pb"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const (
	bufSize = 1024 * 1024
	oneFil  = 1000000000
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

func TestRestartWaiting(t *testing.T) {
	cb, lc, dAddr, cleanupLotus := requireSetupLotus(t, ctx, setupWithSpeed(1000))
	defer cleanupLotus()

	c, cleanupService := requireSetupService(t, ctx, cb, setupWithDbName("restart_waiting"))
	addr := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, true)
	requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
	time.Sleep(time.Millisecond * 100) // Little time to allow the monitoring process to start waiting for all the txns.
	res, err := c.Summary(ctx, &pb.SummaryRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(3), res.CountWaiting)
	require.Equal(t, int64(3), res.CountPending)
	require.Equal(t, int64(1), res.CountActive)
	cleanupService()
	c, cleanupService = requireSetupService(t, ctx, cb, setupWithDbName("restart_waiting"))
	defer cleanupService()
	time.Sleep(time.Millisecond * 100) // Little time to allow the monitoring process to start waiting for all the txns.
	res, err = c.Summary(ctx, &pb.SummaryRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(3), res.CountWaiting)
	require.Equal(t, int64(3), res.CountPending)
	require.Equal(t, int64(1), res.CountActive)
}

func TestSendFil(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr := requireLotusAddress(t, ctx, lc)
	txn := requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_PENDING, txn.MessageState)
}

func TestSendFilWait(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr := requireLotusAddress(t, ctx, lc)
	txn := requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, true)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_ACTIVE, txn.MessageState)
}

func TestSendFilWaitTimeout(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx, setupWithSpeed(3000), setupWithMessageWaitTimeout(time.Second))
	defer cleanup()
	addr := requireLotusAddress(t, ctx, lc)
	_, err := c.SendFil(ctx, &pb.SendFilRequest{From: dAddr.String(), To: addr.String(), AmountNanoFil: oneFil, Wait: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "waiting for txn status timed out, but txn is still processing, query txn again if needed")
}

func TestGetTxn(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx, setupWithSpeed(1000))
	defer cleanup()
	addr := requireLotusAddress(t, ctx, lc)
	txn := requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
	res, err := c.GetTxn(ctx, &pb.GetTxnRequest{MessageCid: txn.MessageCid, Wait: false})
	require.NoError(t, err)
	require.Equal(t, txn.MessageCid, res.Txn.MessageCid)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_PENDING, res.Txn.MessageState)
}

func TestGetTxnWait(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr := requireLotusAddress(t, ctx, lc)
	txn1 := requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
	res, err := c.GetTxn(ctx, &pb.GetTxnRequest{MessageCid: txn1.MessageCid, Wait: true})
	require.NoError(t, err)
	require.Equal(t, txn1.MessageCid, res.Txn.MessageCid)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_ACTIVE, res.Txn.MessageState)
}

func TestGetTxnWaitTimeout(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx, setupWithSpeed(3000), setupWithMessageWaitTimeout(time.Second))
	defer cleanup()
	addr := requireLotusAddress(t, ctx, lc)
	txn1 := requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
	_, err := c.GetTxn(ctx, &pb.GetTxnRequest{MessageCid: txn1.MessageCid, Wait: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "waiting for txn status timed out, but txn is still processing, query txn again if needed")
}

func TestGetTxnNonExistent(t *testing.T) {
	c, _, _, cleanup := requireSetup(t, ctx)
	defer cleanup()
	_, err := c.GetTxn(ctx, &pb.GetTxnRequest{MessageCid: randomCid().String()})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestListTxns(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Txns, 3)
}

func TestListTxnsFrom(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	addr2 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr2.String(), oneFil*2, true)
	requireSendFil(t, ctx, c, addr2.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{FromFilter: dAddr.String()})
	require.NoError(t, err)
	require.Len(t, res.Txns, 3)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{FromFilter: addr2.String()})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
}

func TestListTxnsTo(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	addr2 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr2.String(), oneFil*2, true)
	requireSendFil(t, ctx, c, addr2.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{ToFilter: addr1.String()})
	require.NoError(t, err)
	require.Len(t, res.Txns, 3)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{ToFilter: addr2.String()})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
}

func TestListTxnsInvolvingAddress(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	addr2 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr2.String(), oneFil*2, true)
	requireSendFil(t, ctx, c, addr2.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{InvolvingAddressFilter: dAddr.String()})
	require.NoError(t, err)
	require.Len(t, res.Txns, 3)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{InvolvingAddressFilter: addr1.String()})
	require.NoError(t, err)
	require.Len(t, res.Txns, 3)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{InvolvingAddressFilter: addr2.String()})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
}

func TestListTxnsAmtGt(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil * 2})
	require.NoError(t, err)
	require.Len(t, res.Txns, 0)
}

func TestListTxnsAmtGteq(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil * 3})
	require.NoError(t, err)
	require.Len(t, res.Txns, 0)
}

func TestListTxnsAmtLt(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilLtFilter: oneFil})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilLtFilter: oneFil / 2})
	require.NoError(t, err)
	require.Len(t, res.Txns, 0)
}

func TestListTxnsAmtLteq(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilLteqFilter: oneFil})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilLteqFilter: oneFil / 3})
	require.NoError(t, err)
	require.Len(t, res.Txns, 0)
}

func TestListTxnsAmtGtLt(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil / 2, AmountNanoFilLtFilter: oneFil * 3 / 2})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
}

func TestListTxnsAmtGteqLteq(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*3, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil, AmountNanoFilLteqFilter: oneFil * 2})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
}

func TestListTxnsAmtGteqLt(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGteqFilter: oneFil / 2, AmountNanoFilLtFilter: oneFil * 3 / 2})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
}

func TestListTxnsAmtGtLteq(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*3, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilGtFilter: oneFil, AmountNanoFilLteqFilter: oneFil * 2})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
}

func TestListTxnsAmtEq(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil/2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*3, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{AmountNanoFilEqFilter: oneFil})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
}

func TestListTxnsMessageState(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx, setupWithSpeed(1000))
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, true)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{MessageStateFilter: pb.MessageState_MESSAGE_STATE_ACTIVE})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{MessageStateFilter: pb.MessageState_MESSAGE_STATE_PENDING})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
}

func TestListTxnsWaiting(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx, setupWithSpeed(1000))
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, true)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	time.Sleep(time.Millisecond * 100) // Little time to allow the monitoring process to start waiting for all the txns.
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{WaitingFilter: pb.WaitingFilter_WAITING_FILTER_NOT_WAITING})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{WaitingFilter: pb.WaitingFilter_WAITING_FILTER_WAITING})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
}

func TestListTxnsCreatedAfter(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	t1 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{CreatedAfter: t1.CreatedAt})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
}

func TestListTxnsCreatedBefore(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	t3 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{CreatedBefore: t3.CreatedAt})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
}

func TestListTxnsCreatedAfterBefore(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	t1 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	t3 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{CreatedAfter: t1.CreatedAt, CreatedBefore: t3.CreatedAt})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
}

func TestListTxnsUpdatedAfter(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	t1 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{UpdatedAfter: t1.UpdatedAt})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
}

func TestListTxnsUpdatedBefore(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	t3 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{UpdatedBefore: t3.UpdatedAt})
	require.NoError(t, err)
	require.Len(t, res.Txns, 2)
}

func TestListTxnsUpdatedAfterBefore(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	t1 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	t3 := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{UpdatedAfter: t1.UpdatedAt, UpdatedBefore: t3.UpdatedAt})
	require.NoError(t, err)
	require.Len(t, res.Txns, 1)
}

func TestListTxnsOrder(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{Ascending: false})
	require.NoError(t, err)
	require.Len(t, res.Txns, 3)
	requireTxnsOrder(t, res.Txns, false)
	res, err = c.ListTxns(ctx, &pb.ListTxnsRequest{Ascending: true})
	require.NoError(t, err)
	require.Len(t, res.Txns, 3)
	requireTxnsOrder(t, res.Txns, true)
}

func TestListTxnsPaging(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	txFirst := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)
	txLast := requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil, false)

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
			res, err := c.ListTxns(ctx, req)
			require.NoError(t, err)
			if len(res.Txns) == 0 {
				break
			}
			requireTxnsOrder(t, res.Txns, ascending)
			if page < 2 {
				require.Len(t, res.Txns, 3)
			}
			if page == 2 {
				require.Len(t, res.Txns, 2)
			}
			page++
		}
		require.Equal(t, int64(3), page)
	}
	pageResults(false)
	pageResults(true)
}

func TestSummary(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx, setupWithSpeed(1000))
	defer cleanup()
	addr1 := requireLotusAddress(t, ctx, lc)
	addr2 := requireLotusAddress(t, ctx, lc)
	txFirst := requireSendFil(t, ctx, c, dAddr.String(), addr2.String(), oneFil*3, true)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	requireSendFil(t, ctx, c, dAddr.String(), addr1.String(), oneFil*2, false)
	txLast := requireSendFil(t, ctx, c, addr2.String(), addr1.String(), oneFil, false)
	time.Sleep(time.Millisecond * 100) // Little time to allow the monitoring process to start waiting for all the txns.
	res, err := c.Summary(ctx, &pb.SummaryRequest{})
	require.NoError(t, err)
	require.Equal(t, float64(oneFil*2), res.AvgNanoFilSent)
	require.Equal(t, int64(1), res.CountActive)
	require.Equal(t, int64(0), res.CountFailed)
	require.Equal(t, int64(2), res.CountFromAddrs)
	require.Equal(t, int64(3), res.CountPending)
	require.Equal(t, int64(2), res.CountToAddrs)
	require.Equal(t, int64(4), res.CountTxns)
	require.Equal(t, int64(3), res.CountWaiting)
	require.Equal(t, int64(oneFil*3), res.MaxNanoFilSent)
	require.Equal(t, int64(oneFil), res.MinNanoFilSent)
	require.Equal(t, int64(oneFil*8), res.TotalNanoFilSent)
	res, err = c.Summary(ctx, &pb.SummaryRequest{After: txFirst.CreatedAt, Before: txLast.CreatedAt})
	require.NoError(t, err)
	require.Equal(t, float64(oneFil*2), res.AvgNanoFilSent)
	require.Equal(t, int64(0), res.CountActive)
	require.Equal(t, int64(0), res.CountFailed)
	require.Equal(t, int64(1), res.CountFromAddrs)
	require.Equal(t, int64(2), res.CountPending)
	require.Equal(t, int64(1), res.CountToAddrs)
	require.Equal(t, int64(2), res.CountTxns)
	require.Equal(t, int64(2), res.CountWaiting)
	require.Equal(t, int64(oneFil*2), res.MaxNanoFilSent)
	require.Equal(t, int64(oneFil*2), res.MinNanoFilSent)
	require.Equal(t, int64(oneFil*4), res.TotalNanoFilSent)
}

type setupConfig struct {
	dbName             string
	messageWaitTimeout time.Duration
	messageConfidence  uint64
	retryWaitFrequency time.Duration
	speed              int
}

type setupOption = func(*setupConfig)

func setupWithSpeed(speed int) setupOption {
	return func(config *setupConfig) {
		config.speed = speed
	}
}

func setupWithDbName(dbName string) setupOption {
	return func(config *setupConfig) {
		config.dbName = dbName
	}
}

func setupWithMessageWaitTimeout(messageWaitTimeout time.Duration) setupOption {
	return func(config *setupConfig) {
		config.messageWaitTimeout = messageWaitTimeout
	}
}

func setupWithMessageConfidence(messageConfidence uint64) setupOption {
	return func(config *setupConfig) {
		config.messageConfidence = messageConfidence
	}
}

func setupWithRetryWaitFrequency(retryWaitFrequency time.Duration) setupOption {
	return func(config *setupConfig) {
		config.retryWaitFrequency = retryWaitFrequency
	}
}

func requireSetupLotus(t *testing.T, ctx context.Context, opts ...setupOption) (lotus.ClientBuilder, *apistruct.FullNodeStruct, address.Address, func()) {
	config := &setupConfig{
		speed: 300,
	}
	for _, opt := range opts {
		opt(config)
	}
	clientBuilder, addr, _ := tests.CreateLocalDevnet(t, 1, config.speed)
	time.Sleep(time.Millisecond * 500) // Allow the network to some tipsets

	lotusClient, closeLotusClient, err := clientBuilder(ctx)
	require.NoError(t, err)

	cleanup := func() {
		closeLotusClient()
	}
	return clientBuilder, lotusClient, addr, cleanup
}

func requireSetupService(t *testing.T, ctx context.Context, cb lotus.ClientBuilder, opts ...setupOption) (pb.SendFilServiceClient, func()) {
	config := &setupConfig{
		dbName:             util.MakeToken(12),
		messageWaitTimeout: time.Minute,
		messageConfidence:  2,
		retryWaitFrequency: time.Minute,
	}
	for _, opt := range opts {
		opt(config)
	}
	listener := bufconn.Listen(bufSize)

	conf := Config{
		Listener:           listener,
		ClientBuilder:      cb,
		MongoUri:           test.GetMongoUri(),
		MongoDbName:        config.dbName,
		MessageWaitTimeout: config.messageWaitTimeout,
		MessageConfidence:  config.messageConfidence,
		RetryWaitFrequency: config.retryWaitFrequency,
		Debug:              true,
	}
	s, err := New(conf)
	require.NoError(t, err)

	bufDialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}

	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	require.NoError(t, err)
	client := pb.NewSendFilServiceClient(conn)

	cleanup := func() {
		conn.Close()
		s.Close()
	}

	return client, cleanup
}

func requireSetup(t *testing.T, ctx context.Context, opts ...setupOption) (pb.SendFilServiceClient, *apistruct.FullNodeStruct, address.Address, func()) {
	cb, lotusClient, addr, cleanupLouts := requireSetupLotus(t, ctx, opts...)
	serviceClient, cleanupService := requireSetupService(t, ctx, cb, opts...)

	cleanup := func() {
		cleanupLouts()
		cleanupService()
	}

	return serviceClient, lotusClient, addr, cleanup
}

func requireLotusAddress(t *testing.T, ctx context.Context, lotusClient *apistruct.FullNodeStruct) address.Address {
	addr, err := lotusClient.WalletNew(ctx, types.KTBLS)
	require.NoError(t, err)
	require.Greater(t, len(addr.String()), 0)
	return addr
}

func requireSendFil(t *testing.T, ctx context.Context, c pb.SendFilServiceClient, from, to string, amt int64, wait bool) *pb.Txn {
	res, err := c.SendFil(ctx, &pb.SendFilRequest{From: from, To: to, AmountNanoFil: amt, Wait: wait})
	require.NoError(t, err)
	require.Equal(t, from, res.Txn.From)
	require.Equal(t, to, res.Txn.To)
	require.Equal(t, amt, res.Txn.AmountNanoFil)
	require.NotEmpty(t, res.Txn.MessageCid)
	return res.Txn
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

func randomCid() cid.Cid {
	data := make([]byte, 20)
	rand.Read(data)
	hash, _ := mh.Sum(data, mh.SHA2_256, -1)
	return cid.NewCidV1(cid.DagCBOR, hash)
}
