package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/mindexd/migrations"
	pb "github.com/textileio/textile/v2/api/mindexd/pb"
	"github.com/textileio/textile/v2/api/mindexd/records/collector"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
)

var (
	log = logging.Logger("mindexd")
)

type Service struct {
	config Config
	server *grpc.Server

	db        *mongo.Database
	collector *collector.Collector
}

var _ pb.APIServiceServer = (*Service)(nil)

type Config struct {
	ListenAddr ma.Multiaddr
	Debug      bool

	DBURI  string
	DBName string

	// TTODO: populate from viper
	CollectorRunOnStart   bool
	CollectorFrequency    time.Duration
	CollectorTargets      []collector.PowTarget
	CollectorFetchLimit   int
	CollectorFetchTimeout time.Duration
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

	collectorOpts := []collector.Option{
		collector.WithRunOnStart(config.CollectorRunOnStart),
		collector.WithFrequency(config.CollectorFrequency),
		collector.WithTargets(config.CollectorTargets...),
		collector.WithFetchLimit(config.CollectorFetchLimit),
		collector.WithFetchTimeout(config.CollectorFetchTimeout),
	}
	collector, err := collector.New(db, collectorOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating collector: %s", err)
	}

	s := &Service{
		config:    config,
		db:        db,
		collector: collector,
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

	if err := s.collector.Close(); err != nil {
		log.Errorf("closing collector: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	if err := s.db.Client().Disconnect(ctx); err != nil {
		log.Errorf("disconnecting from mongo: %s", err)
	}
}
