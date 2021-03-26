package service

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"net"
	"testing"
	"time"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/lotus/api"
	chainTypes "github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pb "github.com/textileio/textile/v2/api/sendfild/pb"
	"github.com/textileio/textile/v2/api/sendfild/service/interfaces"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const (
	bufSize = 1024 * 1024
	oneFil  = int64(1000000000)
)

var (
	ctx = context.Background()
)

func TestRestartWaiting(t *testing.T) {
	cid := randomCid()

	fcMock := interfaces.MockFilecoinClient{}
	msgLookup := &api.MsgLookup{
		Message: cid,
		Receipt: chainTypes.MessageReceipt{
			ExitCode: exitcode.Ok,
		},
	}
	fcMock.On(
		"StateWaitMsg",
		mock.Anything,
		mock.AnythingOfType("cid.Cid"),
		mock.AnythingOfType("uint64"),
	).Return(msgLookup, nil)

	storeMock := interfaces.MockTxnStore{}
	storeMock.On(
		"GetAllPending",
		mock.Anything,
		mock.AnythingOfType("bool"),
	).Return([]*pb.Txn{{Id: "id", MessageCid: cid.String()}}, nil)
	storeMock.On("SetWaiting", mock.Anything, cid.String(), mock.AnythingOfType("bool")).Return(nil)
	storeMock.On("Activate", mock.Anything, cid.String(), cid.String()).Return(nil)
	storeMock.On("Close").Return(nil).Maybe()

	_, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	time.Sleep(time.Millisecond * 500) // sleep to let the goroutines spin up

	storeMock.AssertExpectations(t)
	fcMock.AssertExpectations(t)
}

func TestSendFil(t *testing.T) {
	fromAddr, err := addr.NewIDAddress(0)
	require.NoError(t, err)
	toAddr, err := addr.NewIDAddress(1)
	require.NoError(t, err)

	attoFilAmt := (&big.Int{}).Mul((&big.Int{}).SetInt64(oneFil), (&big.Int{}).SetInt64(int64(math.Pow10(9))))

	signedMessage := &chainTypes.SignedMessage{
		Message: chainTypes.Message{
			To:    toAddr,
			From:  fromAddr,
			Value: chainTypes.BigInt{Int: attoFilAmt},
		},
	}

	fcMock := interfaces.MockFilecoinClient{}
	fcMock.On(
		"MpoolPushMessage",
		mock.Anything,
		mock.AnythingOfType("*types.Message"),
		mock.AnythingOfType("*api.MessageSendSpec"),
	).Return(signedMessage, nil)
	fcMock.On(
		"StateWaitMsg",
		mock.Anything,
		mock.AnythingOfType("cid.Cid"),
		mock.AnythingOfType("uint64"),
	).Return(
		&api.MsgLookup{
			Message: signedMessage.Cid(),
			Receipt: chainTypes.MessageReceipt{
				ExitCode: exitcode.Ok,
			},
		},
		nil,
	)

	storeMock := interfaces.MockTxnStore{}
	storeMock.On(
		"New",
		mock.Anything,
		signedMessage.Cid().String(),
		fromAddr.String(),
		toAddr.String(),
		oneFil,
	).Return(
		&pb.Txn{
			MessageCid:    signedMessage.Cid().String(),
			From:          fromAddr.String(),
			To:            toAddr.String(),
			AmountNanoFil: oneFil,
			MessageState:  pb.MessageState_MESSAGE_STATE_PENDING,
		},
		nil,
	)
	storeMock.On("SetWaiting", mock.Anything, signedMessage.Cid().String(), mock.AnythingOfType("bool")).Return(nil)
	storeMock.On(
		"Activate",
		mock.Anything,
		signedMessage.Cid().String(),
		signedMessage.Cid().String(),
	).Return(nil).Maybe()
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	txn := requireSendFil(t, ctx, c, fromAddr.String(), toAddr.String(), oneFil, false)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_PENDING, txn.MessageState)

	storeMock.AssertExpectations(t)
	fcMock.AssertExpectations(t)
}

func TestSendFilWait(t *testing.T) {
	fromAddr, err := addr.NewIDAddress(0)
	require.NoError(t, err)
	toAddr, err := addr.NewIDAddress(1)
	require.NoError(t, err)

	attoFilAmt := (&big.Int{}).Mul((&big.Int{}).SetInt64(oneFil), (&big.Int{}).SetInt64(int64(math.Pow10(9))))

	signedMessage := &chainTypes.SignedMessage{
		Message: chainTypes.Message{
			To:    toAddr,
			From:  fromAddr,
			Value: chainTypes.BigInt{Int: attoFilAmt},
		},
	}

	fcMock := interfaces.MockFilecoinClient{}
	fcMock.On(
		"MpoolPushMessage",
		mock.Anything,
		mock.AnythingOfType("*types.Message"),
		mock.AnythingOfType("*api.MessageSendSpec"),
	).Return(signedMessage, nil)
	fcMock.On(
		"StateWaitMsg",
		mock.Anything,
		mock.AnythingOfType("cid.Cid"),
		mock.AnythingOfType("uint64"),
	).Return(
		&api.MsgLookup{
			Message: signedMessage.Cid(),
			Receipt: chainTypes.MessageReceipt{
				ExitCode: exitcode.Ok,
			},
		},
		nil,
	)

	storeMock := interfaces.MockTxnStore{}
	storeMock.On(
		"New",
		mock.Anything,
		signedMessage.Cid().String(),
		fromAddr.String(),
		toAddr.String(),
		oneFil,
	).Return(
		&pb.Txn{
			MessageCid:    signedMessage.Cid().String(),
			From:          fromAddr.String(),
			To:            toAddr.String(),
			AmountNanoFil: oneFil,
			MessageState:  pb.MessageState_MESSAGE_STATE_PENDING,
		},
		nil,
	)
	storeMock.On("SetWaiting", mock.Anything, signedMessage.Cid().String(), mock.AnythingOfType("bool")).Return(nil)
	storeMock.On("Activate", mock.Anything, signedMessage.Cid().String(), signedMessage.Cid().String()).Return(nil)
	storeMock.On(
		"Get",
		mock.Anything,
		signedMessage.Cid().String(),
	).Return(
		&pb.Txn{
			MessageCid:    signedMessage.Cid().String(),
			From:          fromAddr.String(),
			To:            toAddr.String(),
			AmountNanoFil: oneFil,
			MessageState:  pb.MessageState_MESSAGE_STATE_ACTIVE,
		},
		nil,
	)
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	txn := requireSendFil(t, ctx, c, fromAddr.String(), toAddr.String(), oneFil, true)
	require.Equal(t, signedMessage.Cid().String(), txn.MessageCid)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_ACTIVE, txn.MessageState)

	storeMock.AssertExpectations(t)
	fcMock.AssertExpectations(t)
}

func TestSendFilWaitTimeout(t *testing.T) {
	fromAddr, err := addr.NewIDAddress(0)
	require.NoError(t, err)
	toAddr, err := addr.NewIDAddress(1)
	require.NoError(t, err)

	attoFilAmt := (&big.Int{}).Mul((&big.Int{}).SetInt64(oneFil), (&big.Int{}).SetInt64(int64(math.Pow10(9))))

	signedMessage := &chainTypes.SignedMessage{
		Message: chainTypes.Message{
			To:    toAddr,
			From:  fromAddr,
			Value: chainTypes.BigInt{Int: attoFilAmt},
		},
	}

	fcMock := interfaces.MockFilecoinClient{}
	fcMock.On(
		"MpoolPushMessage",
		mock.Anything,
		mock.AnythingOfType("*types.Message"),
		mock.AnythingOfType("*api.MessageSendSpec"),
	).Return(signedMessage, nil)
	fcMock.On(
		"StateWaitMsg",
		mock.Anything,
		mock.AnythingOfType("cid.Cid"),
		mock.AnythingOfType("uint64"),
	).Return(
		nil,
		fmt.Errorf("context canceled"),
	)

	storeMock := interfaces.MockTxnStore{}
	storeMock.On(
		"New",
		mock.Anything,
		signedMessage.Cid().String(),
		fromAddr.String(),
		toAddr.String(),
		oneFil,
	).Return(
		&pb.Txn{
			MessageCid:    signedMessage.Cid().String(),
			From:          fromAddr.String(),
			To:            toAddr.String(),
			AmountNanoFil: oneFil,
			MessageState:  pb.MessageState_MESSAGE_STATE_PENDING,
		},
		nil,
	)
	storeMock.On("SetWaiting", mock.Anything, signedMessage.Cid().String(), mock.AnythingOfType("bool")).Return(nil)
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	_, err = c.SendFil(
		ctx,
		&pb.SendFilRequest{
			From:          fromAddr.String(),
			To:            toAddr.String(),
			AmountNanoFil: oneFil,
			Wait:          true,
		},
	)
	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"waiting for txn status timed out, but txn is still processing, query txn again if needed",
	)

	fcMock.AssertExpectations(t)
	storeMock.AssertExpectations(t)
}

func TestGetTxn(t *testing.T) {
	cid := randomCid()
	fcMock := interfaces.MockFilecoinClient{}
	storeMock := interfaces.MockTxnStore{}
	storeMock.On("Get", mock.Anything, cid.String()).Return(&pb.Txn{MessageCid: cid.String()}, nil)
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	res, err := c.GetTxn(ctx, &pb.GetTxnRequest{MessageCid: cid.String()})
	require.NoError(t, err)
	require.Equal(t, cid.String(), res.Txn.MessageCid)

	fcMock.AssertExpectations(t)
	storeMock.AssertExpectations(t)
}

func TestGetTxnWait(t *testing.T) {
	cid := randomCid()

	fcMock := interfaces.MockFilecoinClient{}
	msgLookup := &api.MsgLookup{
		Message: cid,
		Receipt: chainTypes.MessageReceipt{
			ExitCode: exitcode.Ok,
		},
	}
	fcMock.On(
		"StateWaitMsg",
		mock.Anything,
		mock.AnythingOfType("cid.Cid"),
		mock.AnythingOfType("uint64"),
	).Return(msgLookup, nil)

	storeMock := interfaces.MockTxnStore{}
	storeMock.On(
		"Get",
		mock.Anything,
		cid.String(),
	).Return(
		&pb.Txn{
			Id:           "id",
			MessageCid:   cid.String(),
			MessageState: pb.MessageState_MESSAGE_STATE_PENDING},
		nil,
	).Once()
	storeMock.On("SetWaiting", mock.Anything, cid.String(), mock.AnythingOfType("bool")).Return(nil)
	storeMock.On("Activate", mock.Anything, cid.String(), cid.String()).Return(nil)
	storeMock.On(
		"Get",
		mock.Anything,
		cid.String(),
	).Return(
		&pb.Txn{
			Id:           "id",
			MessageCid:   cid.String(),
			MessageState: pb.MessageState_MESSAGE_STATE_ACTIVE,
		},
		nil,
	)
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	res, err := c.GetTxn(ctx, &pb.GetTxnRequest{MessageCid: cid.String(), Wait: true})
	require.NoError(t, err)
	require.Equal(t, cid.String(), res.Txn.MessageCid)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_ACTIVE, res.Txn.MessageState)

	fcMock.AssertExpectations(t)
	storeMock.AssertExpectations(t)
}

func TestGetTxnWaitSendFailure(t *testing.T) {
	cid := randomCid()

	fcMock := interfaces.MockFilecoinClient{}
	msgLookup := &api.MsgLookup{
		Message: cid,
		Receipt: chainTypes.MessageReceipt{
			ExitCode: exitcode.SysErrSenderInvalid,
		},
	}
	fcMock.On(
		"StateWaitMsg",
		mock.Anything,
		mock.AnythingOfType("cid.Cid"),
		mock.AnythingOfType("uint64"),
	).Return(msgLookup, nil)

	storeMock := interfaces.MockTxnStore{}
	storeMock.On(
		"Get",
		mock.Anything,
		cid.String(),
	).Return(
		&pb.Txn{
			Id:           "id",
			MessageCid:   cid.String(),
			MessageState: pb.MessageState_MESSAGE_STATE_PENDING,
		},
		nil,
	).Once()
	storeMock.On("SetWaiting", mock.Anything, cid.String(), mock.AnythingOfType("bool")).Return(nil)
	storeMock.On("Fail", mock.Anything, cid.String(), mock.AnythingOfType("string")).Return(nil)
	storeMock.On(
		"Get",
		mock.Anything,
		cid.String(),
	).Return(
		&pb.Txn{
			Id:           "id",
			MessageCid:   cid.String(),
			MessageState: pb.MessageState_MESSAGE_STATE_FAILED,
			FailureMsg:   "oops",
		},
		nil,
	).Once()
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	res, err := c.GetTxn(ctx, &pb.GetTxnRequest{MessageCid: cid.String(), Wait: true})
	require.NoError(t, err)
	require.Equal(t, cid.String(), res.Txn.MessageCid)
	require.Equal(t, pb.MessageState_MESSAGE_STATE_FAILED, res.Txn.MessageState)
	require.Equal(t, "oops", res.Txn.FailureMsg)

	fcMock.AssertExpectations(t)
	storeMock.AssertExpectations(t)
}

func TestGetTxnWaitTimeout(t *testing.T) {
	cid := randomCid()

	fcMock := interfaces.MockFilecoinClient{}
	fcMock.On(
		"StateWaitMsg",
		mock.Anything,
		mock.AnythingOfType("cid.Cid"),
		mock.AnythingOfType("uint64"),
	).Return(
		nil,
		fmt.Errorf("context canceled"),
	)

	storeMock := interfaces.MockTxnStore{}
	storeMock.On(
		"Get",
		mock.Anything,
		cid.String(),
	).Return(
		&pb.Txn{
			Id:           "id",
			MessageCid:   cid.String(),
			MessageState: pb.MessageState_MESSAGE_STATE_PENDING,
		},
		nil,
	).Once()
	storeMock.On("SetWaiting", mock.Anything, cid.String(), mock.AnythingOfType("bool")).Return(nil)
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	_, err := c.GetTxn(ctx, &pb.GetTxnRequest{MessageCid: cid.String(), Wait: true})
	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"waiting for txn status timed out, but txn is still processing, query txn again if needed",
	)

	fcMock.AssertExpectations(t)
	storeMock.AssertExpectations(t)
}

func TestGetTxnNonExistent(t *testing.T) {
	cid := randomCid()
	fcMock := interfaces.MockFilecoinClient{}
	storeMock := interfaces.MockTxnStore{}
	storeMock.On("Get", mock.Anything, cid.String()).Return(nil, interfaces.ErrTxnNotFound)
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	_, err := c.GetTxn(ctx, &pb.GetTxnRequest{MessageCid: cid.String()})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
	require.Contains(t, st.Message(), "no txn found for cid")
}

func TestListTxns(t *testing.T) {
	fcMock := interfaces.MockFilecoinClient{}
	storeMock := interfaces.MockTxnStore{}
	storeMock.On(
		"List",
		mock.Anything,
		mock.AnythingOfType("*pb.ListTxnsRequest"),
	).Return([]*pb.Txn{{Id: "1"}, {Id: "2"}, {Id: "3"}}, nil)
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	res, err := c.ListTxns(ctx, &pb.ListTxnsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Txns, 3)
	require.Equal(t, "1", res.Txns[0].Id)
	require.Equal(t, "2", res.Txns[1].Id)
	require.Equal(t, "3", res.Txns[2].Id)

	fcMock.AssertExpectations(t)
	storeMock.AssertExpectations(t)
}

func TestListTxnsStoreErr(t *testing.T) {
	fcMock := interfaces.MockFilecoinClient{}
	storeMock := interfaces.MockTxnStore{}
	storeMock.On("List", mock.Anything, mock.AnythingOfType("*pb.ListTxnsRequest")).Return(nil, fmt.Errorf("error"))
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	_, err := c.ListTxns(ctx, &pb.ListTxnsRequest{})
	require.Error(t, err)

	fcMock.AssertExpectations(t)
	storeMock.AssertExpectations(t)
}

func TestSummary(t *testing.T) {
	fcMock := interfaces.MockFilecoinClient{}
	storeMock := interfaces.MockTxnStore{}
	storeMock.On(
		"Summary",
		mock.Anything,
		mock.AnythingOfType("time.Time"),
		mock.AnythingOfType("time.Time"),
	).Return(&pb.SummaryResponse{CountTxns: 5}, nil)
	storeMock.On("GetAllPending", mock.Anything, mock.AnythingOfType("bool")).Return([]*pb.Txn{}, nil)
	storeMock.On("Close").Return(nil).Maybe()

	c, cleanup := requireSetup(t, ctx, &fcMock, &storeMock)
	defer cleanup()

	// ToDo: Test this more thuroughly in store tests.
	res, err := c.Summary(ctx, &pb.SummaryRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(5), res.CountTxns)
}

func requireSetup(
	t *testing.T,
	ctx context.Context,
	fcClient interfaces.FilecoinClient,
	txnStore interfaces.TxnStore,
) (pb.SendFilServiceClient, func()) {
	listener := bufconn.Listen(bufSize)

	clientBuilder := func(ctx context.Context) (interfaces.FilecoinClient, func(), error) {
		return fcClient, func() {}, nil
	}

	conf := Config{
		Listener:            listener,
		ClientBuilder:       clientBuilder,
		TxnStore:            txnStore,
		MessageWaitTimeout:  time.Minute,
		MessageConfidence:   2,
		RetryWaitFrequency:  time.Minute,
		Debug:               true,
		AllowEmptyFromAddrs: true,
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

func requireSendFil(
	t *testing.T,
	ctx context.Context,
	c pb.SendFilServiceClient,
	from,
	to string,
	amt int64,
	wait bool,
) *pb.Txn {
	res, err := c.SendFil(ctx, &pb.SendFilRequest{From: from, To: to, AmountNanoFil: amt, Wait: wait})
	require.NoError(t, err)
	require.Equal(t, from, res.Txn.From)
	require.Equal(t, to, res.Txn.To)
	require.Equal(t, amt, res.Txn.AmountNanoFil)
	require.NotEmpty(t, res.Txn.MessageCid)
	return res.Txn
}

func randomCid() cid.Cid {
	data := make([]byte, 20)
	rand.Read(data)
	hash, _ := mh.Sum(data, mh.SHA2_256, -1)
	return cid.NewCidV1(cid.DagCBOR, hash)
}
