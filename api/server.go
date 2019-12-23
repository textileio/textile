package api

import (
	"context"
	"net"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/util"
	pb "github.com/textileio/textile/api/pb"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/email"
	"github.com/textileio/textile/gateway"
	logger "github.com/whyrusleeping/go-logging"
	"google.golang.org/grpc"
)

var (
	log = logging.Logger("textileapi")
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
	Addr           ma.Multiaddr
	AddrGateway    ma.Multiaddr
	AddrGatewayUrl string

	Users    *c.Users
	Sessions *c.Sessions
	Teams    *c.Teams
	Projects *c.Projects

	EmailClient *email.Client

	SessionSecret []byte

	Debug bool
}

// NewServer starts and returns a new server.
func NewServer(ctx context.Context, conf Config) (*Server, error) {
	var err error
	if conf.Debug {
		err = util.SetLogLevels(map[string]logger.Level{
			"textileapi": logger.DEBUG,
		})
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &Server{
		rpc: grpc.NewServer(),
		service: &service{
			users:         conf.Users,
			sessions:      conf.Sessions,
			teams:         conf.Teams,
			projects:      conf.Projects,
			gateway:       gateway.NewGateway(conf.AddrGateway, conf.AddrGatewayUrl),
			emailClient:   conf.EmailClient,
			sessionSecret: conf.SessionSecret,
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
		if err := s.rpc.Serve(listener); err != nil {
			log.Errorf("error registering server: %v", err)
		}
	}()

	s.service.gateway.Start()

	return s, nil
}

// Close the server.
func (s *Server) Close() error {
	s.rpc.GracefulStop()
	if err := s.service.gateway.Stop(); err != nil {
		return err
	}
	s.cancel()
	return nil
}
