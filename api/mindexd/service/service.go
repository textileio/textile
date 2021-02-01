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
	"go.mongodb.org/mongo-driver/bson"
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

	mdb *mongo.Collection
}

var _ pb.APIServiceServer = (*Service)(nil)

type Config struct {
	ListenAddr ma.Multiaddr
	Debug      bool

	DBURI  string
	DBName string
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
		return nil, err
	}
	db := client.Database(config.DBName)
	if err = migrations.Migrate(db); err != nil {
		return nil, err
	}

	mdb := db.Collection("miner_index")
	indexes, err := mdb.Indexes().CreateMany(ctx, []mongo.IndexModel{
		// TTODO
		{
			Keys:    bson.D{{"customer_id", 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating indexes: %w", err)
	}
	for _, index := range indexes {
		log.Infof("created index: %s", index)
	}

	s := &Service{
		config: config,
		mdb:    mdb,
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

func (s *Service) Stop() error {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return s.mdb.Database().Client().Disconnect(ctx)
}

/*
func (s *Service) CreateCustomer(ctx context.Context, req *pb.CreateCustomerRequest) (
	*pb.CreateCustomerResponse, error) {
	var parentKey string
	if req.Parent != nil {
		if _, err := s.createCustomer(ctx, req.Parent, ""); err != nil &&
			!errors.Is(err, ErrCustomerExists) {
			return nil, err
		}
		parentKey = req.Parent.Key
	}
	cus, err := s.createCustomer(ctx, req.Customer, parentKey)
	if err != nil {
		return nil, err
	}
	return &pb.CreateCustomerResponse{
		CustomerId: cus.CustomerID,
	}, nil
}
*/
