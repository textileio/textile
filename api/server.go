package api

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	logging "github.com/ipfs/go-log"
	iface "github.com/ipfs/interface-go-ipfs-core"
	ma "github.com/multiformats/go-multiaddr"
	fc "github.com/textileio/filecoin/api/client"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/broadcast"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/buckets"
	bucketspb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/api/cloud"
	cloudpb "github.com/textileio/textile/api/cloud/pb"
	"github.com/textileio/textile/api/common"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/dns"
	"github.com/textileio/textile/email"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("textileapi")

	ignoreMethods = []string{
		"/cloudpb.API/Login",
	}
)

// Server provides a gRPC API to the textile daemon.
type Server struct {
	rpc   *grpc.Server
	proxy *http.Server

	cloud   *cloud.Service
	buckets *buckets.Service

	gatewayToken string

	ctx    context.Context
	cancel context.CancelFunc
}

// Config specifies server settings.
type Config struct {
	Addr      ma.Multiaddr
	AddrProxy ma.Multiaddr

	Collections *c.Collections

	ThreadsClient  *tc.Client
	IPFSClient     iface.CoreAPI
	FilecoinClient *fc.Client

	DNSManager *dns.Manager

	EmailClient *email.Client

	GatewayUrl   string
	GatewayToken string

	SessionBus    *broadcast.Broadcaster
	SessionSecret string

	Debug bool
}

// NewServer starts and returns a new server.
func NewServer(ctx context.Context, conf Config) (*Server, error) {
	var err error
	if conf.Debug {
		err = util.SetLogLevels(map[string]logging.LogLevel{
			"textileapi": logging.LevelDebug,
		})
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &Server{
		cloud: &cloud.Service{
			Collections:   conf.Collections,
			EmailClient:   conf.EmailClient,
			GatewayUrl:    conf.GatewayUrl,
			SessionBus:    conf.SessionBus,
			SessionSecret: conf.SessionSecret,
		},
		buckets: &buckets.Service{
			Buckets:        buckets.NewBuckets(conf.ThreadsClient),
			IPFSClient:     conf.IPFSClient,
			FilecoinClient: conf.FilecoinClient,
			GatewayUrl:     conf.GatewayUrl,
			DNSManager:     conf.DNSManager,
		},
		gatewayToken: conf.GatewayToken,
		ctx:          ctx,
		cancel:       cancel,
	}
	s.rpc = grpc.NewServer(
		grpc.UnaryInterceptor(auth.UnaryServerInterceptor(s.authFunc)),
		grpc.StreamInterceptor(auth.StreamServerInterceptor(s.authFunc)),
	)

	addr, err := util.TCPAddrFromMultiAddr(conf.Addr)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		cloudpb.RegisterAPIServer(s.rpc, s.cloud)
		bucketspb.RegisterAPIServer(s.rpc, s.buckets)
		if err := s.rpc.Serve(listener); err != nil {
			log.Errorf("error serving listener: %v", err)
		}
	}()

	webrpc := grpcweb.WrapServer(
		s.rpc,
		grpcweb.WithOriginFunc(func(origin string) bool {
			return true
		}),
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
			return true
		}))
	proxyAddr, err := util.TCPAddrFromMultiAddr(conf.AddrProxy)
	if err != nil {
		return nil, err
	}
	s.proxy = &http.Server{
		Addr: proxyAddr,
	}
	s.proxy.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if webrpc.IsGrpcWebRequest(r) ||
			webrpc.IsAcceptableGrpcCorsRequest(r) ||
			webrpc.IsGrpcWebSocketRequest(r) {
			webrpc.ServeHTTP(w, r)
		}
	})

	go func() {
		if err := s.proxy.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("proxy error: %v", err)
		}
	}()

	return s, nil
}

// Close the server.
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.proxy.Shutdown(ctx); err != nil {
		log.Errorf("proxy shutdown error: %s", err)
	}

	s.rpc.GracefulStop()
	s.cancel()
	return nil
}

func (s *Server) authFunc(ctx context.Context) (context.Context, error) {
	method, _ := grpc.Method(ctx)
	for _, ignored := range ignoreMethods {
		if method == ignored {
			return ctx, nil
		}
	}

	token, err := auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		return nil, err
	}
	//if token == s.gatewayToken {
	//	return context.WithValue(ctx, reqKey("org"), "*"), nil
	//}

	// Check for an active session
	session, err := s.cloud.Collections.Sessions.Get(ctx, token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Invalid auth token")
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, status.Error(codes.Unauthenticated, "Expired auth token")
	}
	if err := s.cloud.Collections.Sessions.Touch(ctx, session.Token); err != nil {
		return nil, err
	}
	newCtx := c.NewSessionContext(ctx, session)

	// Load the developer
	dev, err := s.cloud.Collections.Developers.Get(ctx, session.DeveloperID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	newCtx = c.NewDevContext(newCtx, dev)

	// Load org if available
	orgName := metautils.ExtractIncoming(ctx).Get("x-org")
	if orgName != "" {
		isMember, err := s.cloud.Collections.Orgs.IsMember(ctx, orgName, dev.ID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, status.Error(codes.PermissionDenied, "User is not an org member")
		} else {
			org, err := s.cloud.Collections.Orgs.Get(ctx, orgName)
			if err != nil {
				return nil, err
			}
			newCtx = c.NewOrgContext(newCtx, org)
		}
	}

	// /thread/sfvdfamdfvvfioadfvononofdvnovfionfvn/collection/buckets?id=UUID
	// /thread/sfvdfamdfvvfioadfvononofdvnovfionfvn/collection/buckets?name=foo
	// /thread/sfvdfamdfvvfioadfvononofdvnovfionfvn?collection=buckets&name=foo

	// Validate store header if available
	threadAddr := metautils.ExtractIncoming(ctx).Get("x-thread-addr")
	if threadAddr != "" {
		u, err := url.Parse(threadAddr)
		if err != nil {
			return nil, err
		}
		addr, err := ma.NewMultiaddr(u.Path)
		if err != nil {
			return nil, err
		}
		idstr, err := addr.ValueForProtocol(thread.Code)
		if err != nil {
			return nil, err
		}
		id, err := thread.Decode(idstr)
		if err != nil {
			return nil, err
		}
		// @todo: Make sure the caller owns this store.
		newCtx = common.NewStoreContext(newCtx, id)

		//query, err := url.ParseQuery(u.RawQuery)
		//if err != nil {
		//	return nil, err
		//}
		//name := query.Get("name")
		//if name != "" {
		//	buck, err := s.buckets.Buckets.Get(ctx, id, name)
		//	if err != nil {
		//		return nil, err
		//	}
		//	newCtx = newBucketContext(newCtx, buck)
		//}
	}

	return newCtx, nil
}
