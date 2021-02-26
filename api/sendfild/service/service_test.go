package service

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api/apistruct"
	"github.com/filecoin-project/lotus/chain/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-ds-mongo/test"
	"github.com/textileio/powergate/v2/tests"
	powutil "github.com/textileio/powergate/v2/util"
	pb "github.com/textileio/textile/v2/api/sendfild/pb"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var (
	ctx, _ = context.WithTimeout(context.Background(), 2*time.Minute)
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

func TestIt(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr := requireLotusAddress(t, ctx, lc)
	res, err := c.SendFil(ctx, &pb.SendFilRequest{From: dAddr.String(), To: addr.String(), AmountNanoFil: 1000, Wait: false})
	require.NoError(t, err)
	require.NotEmpty(t, res.Txn.MessageCid)
	infoRes, err := c.Txn(ctx, &pb.TxnRequest{MessageCid: res.Txn.MessageCid, Wait: false})
	require.NoError(t, err)
	require.NotNil(t, infoRes)
	infoRes2, err := c.Txn(ctx, &pb.TxnRequest{MessageCid: res.Txn.MessageCid, Wait: true})
	require.NoError(t, err)
	require.NotNil(t, infoRes2)
	infoRes3, err := c.Txn(ctx, &pb.TxnRequest{MessageCid: res.Txn.MessageCid, Wait: true})
	require.NoError(t, err)
	require.NotNil(t, infoRes3)
}

func TestSummary(t *testing.T) {
	c, lc, dAddr, cleanup := requireSetup(t, ctx)
	defer cleanup()
	addr := requireLotusAddress(t, ctx, lc)
	addr2 := requireLotusAddress(t, ctx, lc)
	_, err := c.SendFil(ctx, &pb.SendFilRequest{From: dAddr.String(), To: addr.String(), AmountNanoFil: 2500, Wait: false})
	require.NoError(t, err)
	_, err = c.SendFil(ctx, &pb.SendFilRequest{From: dAddr.String(), To: addr2.String(), AmountNanoFil: 1000, Wait: false})
	require.NoError(t, err)
	_, err = c.SendFil(ctx, &pb.SendFilRequest{From: dAddr.String(), To: addr.String(), AmountNanoFil: 400, Wait: true})
	require.NoError(t, err)
	summary, err := c.Summary(ctx, &pb.SummaryRequest{})
	require.NoError(t, err)
	require.NotNil(t, summary)
}

func requireSetup(t *testing.T, ctx context.Context) (pb.SendFilServiceClient, *apistruct.FullNodeStruct, address.Address, func()) {
	clientBuilder, addr, _ := tests.CreateLocalDevnet(t, 1)
	time.Sleep(time.Millisecond * 500) // Allow the network to some tipsets

	listener := bufconn.Listen(bufSize)

	conf := Config{
		Listener:      listener,
		ClientBuilder: clientBuilder,
		MongoUri:      test.GetMongoUri(),
		MongoDbName:   util.MakeToken(12),
		Debug:         true,
	}
	s, err := New(ctx, conf)
	require.NoError(t, err)

	bufDialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}

	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	require.NoError(t, err)
	client := pb.NewSendFilServiceClient(conn)

	lotusClient, closeLotusClient, err := clientBuilder(ctx)
	require.NoError(t, err)

	cleanup := func() {
		conn.Close()
		s.Close()
		closeLotusClient()
	}

	return client, lotusClient, addr, cleanup
}

func requireLotusAddress(t *testing.T, ctx context.Context, lotusClient *apistruct.FullNodeStruct) address.Address {
	addr, err := lotusClient.WalletNew(ctx, types.KTBLS)
	require.NoError(t, err)
	require.Greater(t, len(addr.String()), 0)
	return addr
}
