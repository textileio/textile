package api

import (
	"context"
	"net"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-threads/util"
	pb "github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/messaging"
	"github.com/textileio/textile/resources/users"
	logger "github.com/whyrusleeping/go-logging"
	"google.golang.org/grpc"
)

var (
	log = logging.Logger("api")
)

// Server provides a gRPC API to the textile daemon.
type Server struct {
	rpc     *grpc.Server
	service *service

	ctx    context.Context
	cancel context.CancelFunc
}

// Config specifies server settings.
type Config struct {
	Addr  ma.Multiaddr
	Users *users.Users
	Email *messaging.EmailService
	//Projects *projects.Projects
	Debug bool
}

// NewServer starts and returns a new server.
func NewServer(ctx context.Context, conf Config) (*Server, error) {
	var err error
	if conf.Debug {
		err = util.SetLogLevels(map[string]logger.Level{
			"api": logger.DEBUG,
		})
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &Server{
		rpc: grpc.NewServer(),
		service: &service{
			users: conf.Users,
			email: conf.Email,
			//projects: conf.Projects,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	addr, err := util.TCPAddrFromMultiAddr(conf.Addr)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		pb.RegisterAPIServer(s.rpc, s.service)
		_ = s.rpc.Serve(listener)
	}()

	return s, nil
}

// Close the server.
func (s *Server) Close() {
	s.rpc.GracefulStop()
	s.cancel()
}
