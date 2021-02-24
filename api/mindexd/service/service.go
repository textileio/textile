package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/gogo/status"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/util"
	powClient "github.com/textileio/powergate/v2/api/client"
	"github.com/textileio/textile/v2/api/mindexd/collector"
	"github.com/textileio/textile/v2/api/mindexd/indexer"
	"github.com/textileio/textile/v2/api/mindexd/migrations"
	"github.com/textileio/textile/v2/api/mindexd/pb"
	"github.com/textileio/textile/v2/api/mindexd/store"
	"github.com/textileio/textile/v2/cmd"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	// Each epoch in the Filecoin network is ~30s.
	epochDurationSeconds = 30
	queryMaxLimit        = 50
)

var (
	log = logging.Logger("mindexd")
)

type Service struct {
	config         Config
	server         *grpc.Server
	grpcRESTServer *http.Server

	db        *mongo.Database
	collector *collector.Collector
	indexer   *indexer.Indexer
	store     *store.Store
}

var _ pb.APIServiceServer = (*Service)(nil)

type Config struct {
	ListenAddrGRPC ma.Multiaddr
	ListenAddrREST string
	Debug          bool

	DBURI  string
	DBName string

	PowAddrAPI    string
	PowAdminToken string

	CollectorRunOnStart   bool
	CollectorFrequency    time.Duration
	CollectorFetchLimit   int
	CollectorFetchTimeout time.Duration

	IndexerRunOnStart     bool
	IndexerFrequency      time.Duration
	IndexerSnapshotMaxAge time.Duration
}

func NewService(ctx context.Context, config Config) (*Service, error) {
	if config.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"mindexd":   logging.LevelDebug,
			"collector": logging.LevelDebug,
			"indexer":   logging.LevelDebug,
			"store":     logging.LevelDebug,
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
		indexer.WithSnapshotMaxAge(config.IndexerSnapshotMaxAge),
	}

	pow, err := (*powClient.Client)(nil), nil
	if err != nil {
		return nil, err
	}

	sub := collector.Subscribe()
	indexer, err := indexer.New(pow, sub, config.PowAdminToken, store, indexerOpts...)
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
	target, err := util.TCPAddrFromMultiAddr(s.config.ListenAddrGRPC)
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

	// Register gRPC server endpoint
	// Note: Make sure the gRPC server is running properly and accessible
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	err = pb.RegisterAPIServiceHandlerFromEndpoint(context.Background(), mux, "localhost:5000", opts)
	cmd.ErrCheck(err)

	s.grpcRESTServer = &http.Server{
		Addr:    s.config.ListenAddrREST,
		Handler: mux,
	}
	go func() {
		if err := s.grpcRESTServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("gRPC REST API closed unexpectedly: %s", err)
		}
	}()

	return nil
}

func (s *Service) Stop() {
	ctx, cls := context.WithTimeout(context.Background(), time.Second*10)
	defer cls()
	if err := s.grpcRESTServer.Shutdown(ctx); err != nil {
		log.Errorf("closing REST endpoint: %s", err)
	}
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

func (s *Service) QueryIndex(ctx context.Context, req *pb.QueryIndexRequest) (*pb.QueryIndexResponse, error) {
	filters := fromPbQueryIndexRequestFilters(req.Filters)

	sort, err := fromPbQueryIndexRequestSort(req.Sort)
	if err != nil {
		return nil, fmt.Errorf("parsing sorting fields: %s", err)
	}

	if req.Limit > queryMaxLimit {
		req.Limit = queryMaxLimit
	}

	res, err := s.store.QueryIndex(ctx, filters, sort, int(req.Limit), req.Offset)
	if err != nil {
		return nil, fmt.Errorf("querying miners from index: %s", err)
	}

	return toPbQueryIndexResponse(res), nil
}

// GetMinerInfo returns miner's index information for a miner. If no information is
// available, it returns a codes.NotFound status code.
func (s *Service) GetMinerInfo(ctx context.Context, req *pb.GetMinerInfoRequest) (*pb.GetMinerInfoResponse, error) {
	mi, err := s.store.GetMinerInfo(ctx, req.MinerAddress)
	if err == store.ErrMinerNotExists {
		return nil, status.Error(codes.NotFound, "Miner not found")
	}

	return &pb.GetMinerInfoResponse{
		Info: toPbMinerIndexInfo(mi),
	}, nil
}

// CalculateDealPrice calculates deal price for a miner.
func (s *Service) CalculateDealPrice(ctx context.Context, req *pb.CalculateDealPriceRequest) (*pb.CalculateDealPriceResponse, error) {

	durationEpochs := req.DurationDays * 24 * 60 * 60 / epochDurationSeconds
	paddedSize := int64(128 << int(math.Ceil(math.Log2(math.Ceil(float64(req.DataSizeBytes)/127)))))

	ret := &pb.CalculateDealPriceResponse{
		DurationEpochs: durationEpochs,
		PaddedSize:     paddedSize,
		Results:        make([]*pb.CalculateDealPriceMiner, len(req.MinerAddresses)),
	}

	for i, minerAddr := range req.MinerAddresses {
		mi, err := s.store.GetMinerInfo(ctx, minerAddr)
		if err == store.ErrMinerNotExists {
			return nil, status.Errorf(codes.NotFound, "Miner %s not found", minerAddr)
		}
		var askPrice, askVerifiedPrice big.Int
		if _, ok := askPrice.SetString(mi.Filecoin.AskPrice, 10); !ok {
			return nil, fmt.Errorf("parsing ask price: %s", err)
		}
		if _, ok := askVerifiedPrice.SetString(mi.Filecoin.AskVerifiedPrice, 10); !ok {
			return nil, fmt.Errorf("parsing ask verified price: %s", err)
		}

		gibEpochs := big.NewInt(0).Mul(&askPrice, big.NewInt(durationEpochs))
		ret.Results[i] = &pb.CalculateDealPriceMiner{
			Miner:             minerAddr,
			TotalCost:         big.NewInt(0).Mul(gibEpochs, &askPrice).String(),
			VerifiedTotalCost: big.NewInt(0).Mul(gibEpochs, &askVerifiedPrice).String(),
		}
	}

	return ret, nil
}
