package core

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	fc "github.com/textileio/filecoin/api/client"
	threadsapi "github.com/textileio/go-threads/api"
	threadsclient "github.com/textileio/go-threads/api/client"
	threadspb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/broadcast"
	"github.com/textileio/go-threads/core/thread"
	netapi "github.com/textileio/go-threads/service/api"
	netpb "github.com/textileio/go-threads/service/api/pb"
	s "github.com/textileio/go-threads/store"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/buckets"
	bucketspb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/api/cloud"
	cloudpb "github.com/textileio/textile/api/cloud/pb"
	"github.com/textileio/textile/api/common"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/dns"
	"github.com/textileio/textile/email"
	"github.com/textileio/textile/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("core")

	ignoreMethods = []string{
		"/cloud.pb.API/Login",
	}
)

type Textile struct {
	collections *c.Collections

	threadservice s.ServiceBoostrapper
	threadsClient *threadsclient.Client

	threadsToken string
	gatewayToken string

	filecoinClient *fc.Client

	server  *grpc.Server
	proxy   *http.Server
	gateway *gateway.Gateway

	sessionBus *broadcast.Broadcaster
}

type Config struct {
	RepoPath string

	AddrApi                 ma.Multiaddr
	AddrApiProxy            ma.Multiaddr
	AddrThreadsHost         ma.Multiaddr
	AddrIpfsApi             ma.Multiaddr
	AddrGatewayHost         ma.Multiaddr
	AddrGatewayUrl          string
	AddrGatewayBucketDomain string
	AddrFilecoinApi         ma.Multiaddr
	AddrMongoUri            string

	DNSDomain string
	DNSZoneID string
	DNSToken  string

	EmailFrom   string
	EmailDomain string
	EmailApiKey string

	SessionSecret string

	Debug bool
}

func NewTextile(ctx context.Context, conf Config) (*Textile, error) {
	if conf.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"core": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}
	t := &Textile{}

	// Configure clients
	ipfsClient, err := httpapi.NewApi(conf.AddrIpfsApi)
	if err != nil {
		return nil, err
	}
	if conf.AddrFilecoinApi != nil {
		t.filecoinClient, err = fc.NewClient(conf.AddrFilecoinApi)
		if err != nil {
			return nil, err
		}
	}
	var dnsManager *dns.Manager
	if conf.DNSToken != "" {
		dnsManager, err = dns.NewManager(conf.DNSDomain, conf.DNSZoneID, conf.DNSToken, conf.Debug)
		if err != nil {
			return nil, err
		}
	}
	emailClient, err := email.NewClient(
		conf.EmailFrom, conf.EmailDomain, conf.EmailApiKey, conf.Debug)
	if err != nil {
		return nil, err
	}
	t.collections, err = c.NewCollections(ctx, conf.AddrMongoUri)
	if err != nil {
		return nil, err
	}

	// Configure threads
	t.threadservice, err = s.DefaultService(
		conf.RepoPath,
		s.WithServiceHostAddr(conf.AddrThreadsHost),
		s.WithServiceDebug(conf.Debug))
	if err != nil {
		return nil, err
	}

	// Configure gRPC services
	ts, err := threadsapi.NewService(t.threadservice, threadsapi.Config{
		RepoPath: conf.RepoPath,
		Debug:    conf.Debug,
	})
	if err != nil {
		return nil, err
	}
	ns, err := netapi.NewService(t.threadservice, netapi.Config{
		Debug: conf.Debug,
	})
	if err != nil {
		return nil, err
	}
	t.sessionBus = broadcast.NewBroadcaster(0)
	cs := &cloud.Service{
		Collections:   t.collections,
		EmailClient:   emailClient,
		GatewayUrl:    conf.AddrGatewayUrl,
		SessionBus:    t.sessionBus,
		SessionSecret: conf.SessionSecret,
	}
	bs := &buckets.Service{
		Buckets:        buckets.NewBuckets(t.threadsClient),
		IPFSClient:     ipfsClient,
		FilecoinClient: t.filecoinClient,
		GatewayUrl:     conf.AddrGatewayUrl,
		DNSManager:     dnsManager,
	}

	// Configure gRPC server
	target, err := util.TCPAddrFromMultiAddr(conf.AddrApi)
	if err != nil {
		return nil, err
	}
	proxyTarget, err := util.TCPAddrFromMultiAddr(conf.AddrApiProxy)
	if err != nil {
		return nil, err
	}
	t.server = grpc.NewServer(
		grpc.UnaryInterceptor(auth.UnaryServerInterceptor(t.authFunc)),
		grpc.StreamInterceptor(auth.StreamServerInterceptor(t.authFunc)))
	listener, err := net.Listen("tcp", target)
	if err != nil {
		return nil, err
	}
	go func() {
		threadspb.RegisterAPIServer(t.server, ts)
		netpb.RegisterAPIServer(t.server, ns)
		cloudpb.RegisterAPIServer(t.server, cs)
		bucketspb.RegisterAPIServer(t.server, bs)
		if err := t.server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("serve error: %v", err)
		}
	}()
	webrpc := grpcweb.WrapServer(
		t.server,
		grpcweb.WithOriginFunc(func(origin string) bool {
			return true
		}),
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
			return true
		}))
	t.proxy = &http.Server{
		Addr: proxyTarget,
	}
	t.proxy.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if webrpc.IsGrpcWebRequest(r) ||
			webrpc.IsAcceptableGrpcCorsRequest(r) ||
			webrpc.IsGrpcWebSocketRequest(r) {
			webrpc.ServeHTTP(w, r)
		}
	})
	go func() {
		if err := t.proxy.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("proxy error: %v", err)
		}
	}()

	// Start threads client
	t.threadsClient, err = threadsclient.NewClient(target, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	// Configure gateway
	t.gatewayToken = uuid.New().String()
	t.gateway, err = gateway.NewGateway(conf.AddrGatewayHost, conf.AddrApi, t.gatewayToken,
		conf.AddrGatewayBucketDomain, t.collections, t.sessionBus, conf.Debug)
	if err != nil {
		return nil, err
	}
	t.gateway.Start()

	log.Info("started")

	return t, nil
}

func (t *Textile) Bootstrap() {
	t.threadservice.Bootstrap(util.DefaultBoostrapPeers())
}

func (t *Textile) Close() error {
	t.sessionBus.Discard()
	if err := t.gateway.Stop(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := t.proxy.Shutdown(ctx); err != nil {
		return err
	}
	t.server.GracefulStop()
	if err := t.threadsClient.Close(); err != nil {
		return err
	}
	if err := t.threadservice.Close(); err != nil {
		return err
	}
	if t.filecoinClient != nil {
		if err := t.filecoinClient.Close(); err != nil {
			return err
		}
	}
	if err := t.collections.Close(); err != nil {
		return err
	}
	return nil
}

func (t *Textile) HostID() peer.ID {
	return t.threadservice.Host().ID()
}

func (t *Textile) authFunc(ctx context.Context) (context.Context, error) {
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
	session, err := t.collections.Sessions.Get(ctx, token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Invalid auth token")
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, status.Error(codes.Unauthenticated, "Expired auth token")
	}
	if err := t.collections.Sessions.Touch(ctx, session.Token); err != nil {
		return nil, err
	}
	newCtx := c.NewSessionContext(ctx, session)

	// Load the developer
	dev, err := t.collections.Developers.Get(ctx, session.DeveloperID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	newCtx = c.NewDevContext(newCtx, dev)

	// Load org if available
	orgName := metautils.ExtractIncoming(ctx).Get("x-org")
	if orgName != "" {
		isMember, err := t.collections.Orgs.IsMember(ctx, orgName, dev.ID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, status.Error(codes.PermissionDenied, "User is not an org member")
		} else {
			org, err := t.collections.Orgs.Get(ctx, orgName)
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
		newCtx = common.NewDBContext(newCtx, id)

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
