package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/gogo/status"
	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/util"
	powClient "github.com/textileio/powergate/v2/api/client"
	"github.com/textileio/textile/v2/api/mindexd/collector"
	"github.com/textileio/textile/v2/api/mindexd/indexer"
	"github.com/textileio/textile/v2/api/mindexd/migrations"
	pb "github.com/textileio/textile/v2/api/mindexd/pb"
	"github.com/textileio/textile/v2/api/mindexd/store"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	// Each epoch in the Filecoin network is ~30s.
	epochDurationSeconds = 30
)

var (
	log = logging.Logger("mindexd")
)

type Service struct {
	config Config
	server *grpc.Server

	db        *mongo.Database
	collector *collector.Collector
	indexer   *indexer.Indexer
	store     *store.Store
}

var _ pb.APIServiceServer = (*Service)(nil)

type Config struct {
	ListenAddr ma.Multiaddr
	Debug      bool

	DBURI  string
	DBName string

	PowAddrAPI    string
	PowAdminToken string

	CollectorRunOnStart   bool
	CollectorFrequency    time.Duration
	CollectorTargets      []collector.PowTarget
	CollectorFetchLimit   int
	CollectorFetchTimeout time.Duration

	IndexerRunOnStart bool
	IndexerFrequency  time.Duration
}

func NewService(ctx context.Context, config Config) (*Service, error) {
	if config.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"mindex": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.DBURI))
	if err != nil {
		return nil, fmt.Errorf("connecting to mongo: %s", err)
	}
	db := client.Database(config.DBName)
	if err = migrations.Migrate(db); err != nil {
		return nil, fmt.Errorf("executing migrations: %s", err)
	}

	store, err := store.New(db)
	if err != nil {
		return nil, fmt.Errorf("creating store: %s", err)
	}

	collectorOpts := []collector.Option{
		collector.WithRunOnStart(config.CollectorRunOnStart),
		collector.WithFrequency(config.CollectorFrequency),
		collector.WithTargets(config.CollectorTargets...),
		collector.WithFetchLimit(config.CollectorFetchLimit),
		collector.WithFetchTimeout(config.CollectorFetchTimeout),
	}
	collector, err := collector.New(store, collectorOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating collector: %s", err)
	}

	indexerOpts := []indexer.Option{
		indexer.WithRunOnStart(config.IndexerRunOnStart),
		indexer.WithFrequency(config.IndexerFrequency),
	}

	pow, err := powClient.NewClient(config.PowAddrAPI)
	if err != nil {
		return nil, err
	}
	indexer, err := indexer.New(pow, config.PowAdminToken, store, indexerOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating indexer: %s", err)
	}

	s := &Service{
		config:    config,
		db:        db,
		collector: collector,
		indexer:   indexer,
		store:     store,
	}

	return s, nil
}

func (s *Service) Start() error {
	s.server = grpc.NewServer()
	target, err := util.TCPAddrFromMultiAddr(s.config.ListenAddr)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", target)
	if err != nil {
		return err
	}
	go func() {
		pb.RegisterAPIServiceServer(s.server, s)
		if err := s.server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Errorf("serve error: %v", err)
		}
	}()

	return nil
}

func (s *Service) Stop() {
	stopped := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(stopped)
	}()
	timer := time.NewTimer(10 * time.Second)
	select {
	case <-timer.C:
		s.server.Stop()
	case <-stopped:
		timer.Stop()
	}
	log.Info("gRPC was shutdown")

	if err := s.indexer.Close(); err != nil {
		log.Errorf("closing indexer: %s", err)
	}

	if err := s.collector.Close(); err != nil {
		log.Errorf("closing collector: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	if err := s.db.Client().Disconnect(ctx); err != nil {
		log.Errorf("disconnecting from mongo: %s", err)
	}
}

// GetIndexDahsboard returns miners for the miner-index dashboard.
// Note: this is a temporary implementation. The miner-index dashboard isn't defined
// so the API needed is unkown. This is a naive implementation only to do testing in
// the meantime. No paging, no filtering, etc.
func (s *Service) GetIndexDashboard(ctx context.Context, req *pb.GetIndexDashboardRequest) (*pb.GetIndexDashboardResponse, error) {
	all, err := s.store.GetAllMiners(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all miners from index: %s", err)
	}

	ret := &pb.GetIndexDashboardResponse{
		Miners: make([]*pb.MinerIndexInfo, 0, len(all)),
	}
	for _, m := range all {
		ret.Miners = append(ret.Miners, toPbMinerIndexInfo(m))
	}

	return ret, nil
}

func (s *Service) GetMinerInfo(ctx context.Context, req *pb.GetMinerInfoRequest) (*pb.GetMinerInfoResponse, error) {
	mi, err := s.store.GetMinerInfo(ctx, req.MinerAddress)
	if err == store.ErrMinerNotExists {
		return nil, status.Error(codes.NotFound, "Miner not found")
	}

	return &pb.GetMinerInfoResponse{
		Info: toPbMinerIndexInfo(mi),
	}, nil
}

func (s *Service) CalculateDealPrice(ctx context.Context, req *pb.CalculateDealPriceRequest) (*pb.CalculateDealPriceResponse, error) {
	mi, err := s.store.GetMinerInfo(ctx, req.MinerAddress)
	if err == store.ErrMinerNotExists {
		return nil, status.Error(codes.NotFound, "Miner not found")
	}

	durationEpochs := uint64(req.DurationDays * 24 * 60 * 60 / epochDurationSeconds)
	paddedSize := uint64(128 << int(math.Ceil(math.Log2(math.Ceil(float64(req.DataSizeBytes)/127)))))

	ret := &pb.CalculateDealPriceResponse{
		TotalCost:         paddedSize * durationEpochs * mi.Filecoin.AskPrice,
		VerifiedTotalCost: paddedSize * durationEpochs * mi.Filecoin.AskVerifiedPrice,
		DurationEpochs:    durationEpochs,
		PaddedSize:        paddedSize,
	}

	return ret, nil
}
