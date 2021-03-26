package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/filecoin-project/go-address"
	chainTypes "github.com/filecoin-project/lotus/chain/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
	pb "github.com/textileio/textile/v2/api/sendfild/pb"
	"github.com/textileio/textile/v2/api/sendfild/service/interfaces"
	"github.com/textileio/textile/v2/api/sendfild/service/waitmanager"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	listMaxPageSize = 100
)

var (
	_ pb.SendFilServiceServer = (*Service)(nil)

	log = logging.Logger("sendfil")
)

type Service struct {
	clientBuilder interfaces.FilecoinClientBuilder
	txnStore      interfaces.TxnStore
	waitManager   *waitmanager.WaitManager
	server        *grpc.Server
	config        Config
}

type Config struct {
	Listener            net.Listener
	ClientBuilder       interfaces.FilecoinClientBuilder
	TxnStore            interfaces.TxnStore
	MessageWaitTimeout  time.Duration
	MessageConfidence   uint64
	RetryWaitFrequency  time.Duration
	AllowedFromAddrs    []string
	AllowEmptyFromAddrs bool
	Debug               bool
}

func New(config Config) (*Service, error) {
	if config.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"sendfil": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	if !config.AllowEmptyFromAddrs && len(config.AllowedFromAddrs) == 0 {
		return nil, fmt.Errorf("empty allowed from addrs not allowed")
	}

	wm, err := waitmanager.New(config.ClientBuilder, config.TxnStore, config.MessageConfidence, config.MessageWaitTimeout, config.RetryWaitFrequency, config.Debug)
	if err != nil {
		return nil, fmt.Errorf("creating waitmanager: %v", err)
	}

	s := &Service{
		clientBuilder: config.ClientBuilder,
		txnStore:      config.TxnStore,
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
	isAllowedFromAddr := s.config.AllowEmptyFromAddrs
	for _, allowedFromAddr := range s.config.AllowedFromAddrs {
		if allowedFromAddr == req.From {
			isAllowedFromAddr = true
			break
		}
	}
	if !isAllowedFromAddr {
		return nil, status.Error(codes.InvalidArgument, "from address not allowed")
	}

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
	msg := &chainTypes.Message{
		From:  f,
		To:    t,
		Value: chainTypes.BigInt{Int: amount},
	}
	client, cls, err := s.clientBuilder(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating filecoin client: %v", err)
	}

	sm, err := client.MpoolPushMessage(ctx, msg, nil)
	cls()
	if err != nil {
		if strings.Contains(err.Error(), "not enough funds") {
			log.Errorf("not enough funds in %s to send %s fil to %s", f.String(), amount.String(), t.String())
		}
		return nil, status.Errorf(codes.Internal, "pushing message: %v", err)
	}

	txn, err := s.txnStore.New(ctx, sm.Cid().String(), req.From, req.To, req.AmountNanoFil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating new txn: %v", err)
	}

	if err := s.waitManager.RegisterTxn(txn.Id, sm.Cid().String()); err != nil {
		return nil, status.Errorf(codes.Internal, "registering txn: %v", err)
	}

	if req.Wait {
		txn, err = s.waitForTxn(ctx, txn.Id, sm.Cid().String())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "waiting for txn: %v", err)
		}
	}

	return &pb.SendFilResponse{Txn: txn}, nil
}

func (s *Service) GetTxn(ctx context.Context, req *pb.GetTxnRequest) (*pb.GetTxnResponse, error) {
	txn, err := s.txnStore.Get(ctx, req.MessageCid)
	if errors.Is(err, interfaces.ErrTxnNotFound) {
		return nil, status.Error(codes.NotFound, "no txn found for cid")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting txn for cid: %v", err)
	}

	if txn.MessageState == pb.MessageState_MESSAGE_STATE_PENDING && req.Wait {
		txn, err = s.waitForTxn(ctx, txn.Id, txn.MessageCid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "waiting for txn: %v", err)
		}
	}

	return &pb.GetTxnResponse{Txn: txn}, nil
}

func (s *Service) ListTxns(ctx context.Context, req *pb.ListTxnsRequest) (*pb.ListTxnsResponse, error) {
	if req.PageSize > listMaxPageSize || req.PageSize == 0 {
		req.PageSize = listMaxPageSize
	}
	txns, err := s.txnStore.List(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting txns list: %v", err)
	}

	return &pb.ListTxnsResponse{Txns: txns}, nil
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
	summary, err := s.txnStore.Summary(ctx, after, before)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generating summary: %v", err)
	}

	return summary, nil
}

func (s *Service) Close() error {
	var errs []error

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

	if err := s.waitManager.Close(); err != nil {
		log.Errorf("closing wait manager: %s", err)
		errs = append(errs, err)
	} else {
		log.Info("wait manager closed")
	}

	if err := s.txnStore.Close(); err != nil {
		log.Errorf("disconnecting mongo client: %s", err)
		errs = append(errs, err)
	} else {
		log.Info("mongo client disconnected")
	}

	if len(errs) > 0 {
		return fmt.Errorf("closing sendfil service: %q", errs)
	}
	return nil
}

func (s *Service) waitForTxn(ctx context.Context, txnID string, messageCid string) (*pb.Txn, error) {
	listener := make(chan waitmanager.WaitResult)
	cancel, err := s.waitManager.Subscribe(txnID, messageCid, listener)
	defer cancel()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "subscribing: %v", err)
	}
	select {
	case res, ok := <-listener:
		if !ok {
			return nil, status.Error(codes.Internal, "subscription unexpectedly closed")
		}
		if res.Err != nil {
			return nil, status.Errorf(codes.Internal, "listening for result: %v", res.Err)
		}
		txn, err := s.txnStore.Get(ctx, res.LatestMessageCid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "getting updated txn: %v", err)
		}
		return txn, nil
	case <-ctx.Done():
		return nil, status.Errorf(codes.Aborted, "request canceled")
	}
}
