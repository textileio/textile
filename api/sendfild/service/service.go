package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/chain/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/powergate/v2/lotus"
	pb "github.com/textileio/textile/v2/api/sendfild/pb"
	"github.com/textileio/textile/v2/api/sendfild/service/store"
	"github.com/textileio/textile/v2/api/sendfild/service/waitmanager"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	listMaxPageSize = 1000
)

var (
	_ pb.SendFilServiceServer = (*Service)(nil)

	log = logging.Logger("sendfil")
)

type Service struct {
	clientBuilder lotus.ClientBuilder
	store         *store.Store
	waitManager   *waitmanager.WaitManager
	server        *grpc.Server
	config        Config
}

type Config struct {
	Listener           net.Listener
	ClientBuilder      lotus.ClientBuilder
	MongoUri           string
	MongoDbName        string
	MessageWaitTimeout time.Duration
	MessageConfidence  uint64
	RetryWaitFrequency time.Duration
	Debug              bool
}

func New(config Config) (*Service, error) {
	if config.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"sendfil": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	st, err := store.New(config.MongoUri, config.MongoDbName, config.Debug)
	if err != nil {
		return nil, fmt.Errorf("creating store: %v", err)
	}

	wm, err := waitmanager.New(config.ClientBuilder, st, config.MessageConfidence, config.MessageWaitTimeout, config.RetryWaitFrequency)
	if err != nil {
		return nil, fmt.Errorf("creating waitmanager: %v", err)
	}

	s := &Service{
		clientBuilder: config.ClientBuilder,
		store:         st,
		waitManager:   wm,
		config:        config,
	}

	s.server = grpc.NewServer()
	go func() {
		pb.RegisterSendFilServiceServer(s.server, s)
		if err := s.server.Serve(config.Listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Errorf("serve error: %v", err)
		}
	}()

	return s, nil
}

func (s *Service) SendFil(ctx context.Context, req *pb.SendFilRequest) (*pb.SendFilResponse, error) {
	f, err := address.NewFromString(req.From)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parsing from address: %v", err)
	}
	t, err := address.NewFromString(req.To)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parsing to address: %v", err)
	}
	nanoAmount := (&big.Int{}).SetInt64(req.AmountNanoFil)
	factor := (&big.Int{}).SetInt64(int64(math.Pow10(9)))
	amount := (&big.Int{}).Mul(nanoAmount, factor)
	msg := &types.Message{
		From:  f,
		To:    t,
		Value: types.BigInt{Int: amount},
	}
	client, cls, err := s.clientBuilder(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating filecoin client: %v", err)
	}
	defer cls()

	sm, err := client.MpoolPushMessage(ctx, msg, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "pushing message: %v", err)
	}

	txn, err := s.store.NewTxn(ctx, sm.Message.Cid().String(), req.From, req.To, req.AmountNanoFil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating new txn: %v", err)
	}

	if err := s.waitManager.RegisterTxn(txn.ID, sm.Message.Cid().String()); err != nil {
		return nil, status.Errorf(codes.Internal, "registering txn: %v", err)
	}

	if req.Wait {
		txn, err = s.waitForTxn(ctx, txn.ID, sm.Message.Cid().String())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "waiting for txn: %v", err)
		}
	}

	pbTx, err := toPbTxn(txn)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "converting to pb txn: %v", err)
	}

	return &pb.SendFilResponse{Txn: pbTx}, nil
}

func (s *Service) GetTxn(ctx context.Context, req *pb.GetTxnRequest) (*pb.GetTxnResponse, error) {
	txn, err := s.store.GetTxn(ctx, req.MessageCid)
	if errors.Is(err, store.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "no txn found for cid")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting txn for cid: %v", err)
	}

	latestCid, err := txn.LatestMsgCid()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting latest txn message cid: %v", err)
	}

	if txn.MessageState == pb.MessageState_MESSAGE_STATE_PENDING && req.Wait {
		txn, err = s.waitForTxn(ctx, txn.ID, latestCid.Cid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "waiting for txn: %v", err)
		}
	}

	pbTx, err := toPbTxn(txn)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "converting to pb txn: %v", err)
	}

	return &pb.GetTxnResponse{Txn: pbTx}, nil
}

func (s *Service) ListTxns(ctx context.Context, req *pb.ListTxnsRequest) (*pb.ListTxnsResponse, error) {
	if req.PageSize > listMaxPageSize || req.PageSize == 0 {
		req.PageSize = listMaxPageSize
	}
	txns, err := s.store.ListTxns(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting txns list: %v", err)
	}

	var pbTxns []*pb.Txn
	for _, rec := range txns {
		pbTxn, err := toPbTxn(rec)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "converting txn to pb: %v", err)
		}
		pbTxns = append(pbTxns, pbTxn)
	}

	return &pb.ListTxnsResponse{Txns: pbTxns}, nil
}

func (s *Service) Summary(ctx context.Context, req *pb.SummaryRequest) (*pb.SummaryResponse, error) {
	after := time.Time{}
	before := time.Time{}
	if req.After != nil {
		after = req.After.AsTime()
	}
	if req.Before != nil {
		before = req.Before.AsTime()
	}
	summary, err := s.store.GenerateSummary(ctx, after, before)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generating summary: %v", err)
	}

	resp := &pb.SummaryResponse{}

	if len(summary.All) == 1 {
		resp.CountTxns = summary.All[0].Count
	}

	for _, state := range summary.ByMessageState {
		switch pb.MessageState(state.ID.(int32)) {
		case pb.MessageState_MESSAGE_STATE_PENDING:
			resp.CountPending = state.Count
		case pb.MessageState_MESSAGE_STATE_ACTIVE:
			resp.CountActive = state.Count
		case pb.MessageState_MESSAGE_STATE_FAILED:
			resp.CountFailed = state.Count
		}
	}

	if len(summary.Waiting) == 1 {
		resp.CountWaiting = summary.Waiting[0].Count
	}

	resp.CountFromAddrs = int64(len(summary.UniqueFrom))
	resp.CountToAddrs = int64(len(summary.UniqueTo))

	if len(summary.SentFilStats) == 1 {
		resp.TotalNanoFilSent = summary.SentFilStats[0].Total
		resp.AvgNanoFilSent = summary.SentFilStats[0].Avg
		resp.MaxNanoFilSent = summary.SentFilStats[0].Max
		resp.MinNanoFilSent = summary.SentFilStats[0].Min
	}

	return resp, nil
}

func (s *Service) Close() error {
	var errs []error

	if err := s.waitManager.Close(); err != nil {
		log.Errorf("closing wait manager: %s", err)
		errs = append(errs, err)
	} else {
		log.Info("wait manager closed")
	}

	if err := s.store.Close(); err != nil {
		log.Errorf("disconnecting mongo client: %s", err)
		errs = append(errs, err)
	} else {
		log.Info("mongo client disconnected")
	}

	stopped := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(stopped)
	}()
	t := time.NewTimer(10 * time.Second)
	select {
	case <-t.C:
		s.server.Stop()
	case <-stopped:
		t.Stop()
	}
	log.Info("gRPC server stopped")

	if len(errs) > 0 {
		return fmt.Errorf("closing sendfil service: %q", errs)
	}
	return nil
}

func (s *Service) waitForTxn(ctx context.Context, objID primitive.ObjectID, messageCid string) (*store.Txn, error) {
	listener := make(chan waitmanager.WaitResult)
	cancel, err := s.waitManager.Subscribe(objID, messageCid, listener)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "subscribing: %v", err)
	}
	select {
	case res := <-listener:
		if res.Err != nil {
			return nil, status.Errorf(codes.Internal, "listening for result: %v", res.Err)
		}
		txn, err := s.store.GetTxn(ctx, res.LatestMessageCid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "getting updated txn: %v", res.Err)
		}
		return txn, nil
	case <-ctx.Done():
		cancel()
		return nil, status.Errorf(codes.Aborted, "request canceled")
	}
}

func toPbTxn(txn *store.Txn) (*pb.Txn, error) {
	latestMsgCid, err := txn.LatestMsgCid()
	if err != nil {
		return nil, err
	}
	return &pb.Txn{
		Id:            txn.ID.Hex(),
		From:          txn.From,
		To:            txn.To,
		AmountNanoFil: txn.AmountNanoFil,
		MessageCid:    latestMsgCid.Cid,
		MessageState:  txn.MessageState,
		Waiting:       txn.Waiting,
		FailureMsg:    txn.FailureMsg,
		CreatedAt:     timestamppb.New(txn.CreatedAt),
		UpdatedAt:     timestamppb.New(txn.UpdatedAt),
	}, nil
}
