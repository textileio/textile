package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	grpcm "github.com/grpc-ecosystem/go-grpc-middleware"
	auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	connmgr "github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	dbapi "github.com/textileio/go-threads/api"
	threads "github.com/textileio/go-threads/api/client"
	dbpb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/broadcast"
	tc "github.com/textileio/go-threads/common"
	netapi "github.com/textileio/go-threads/net/api"
	netclient "github.com/textileio/go-threads/net/api/client"
	netpb "github.com/textileio/go-threads/net/api/pb"
	nutil "github.com/textileio/go-threads/net/util"
	tutil "github.com/textileio/go-threads/util"
	pow "github.com/textileio/powergate/api/client"
	ffsRpc "github.com/textileio/powergate/ffs/rpc"
	healthRpc "github.com/textileio/powergate/health/rpc"
	netRpc "github.com/textileio/powergate/net/rpc"
	walletRpc "github.com/textileio/powergate/wallet/rpc"
	billing "github.com/textileio/textile/v2/api/billingd/client"
	"github.com/textileio/textile/v2/api/bucketsd"
	bpb "github.com/textileio/textile/v2/api/bucketsd/pb"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/api/hubd"
	hpb "github.com/textileio/textile/v2/api/hubd/pb"
	"github.com/textileio/textile/v2/api/usersd"
	upb "github.com/textileio/textile/v2/api/usersd/pb"
	"github.com/textileio/textile/v2/buckets/archive"
	"github.com/textileio/textile/v2/dns"
	"github.com/textileio/textile/v2/email"
	"github.com/textileio/textile/v2/gateway"
	"github.com/textileio/textile/v2/ipns"
	mdb "github.com/textileio/textile/v2/mongodb"
	tdb "github.com/textileio/textile/v2/threaddb"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
)

var (
	// ErrTooManyThreadsPerOwner indicates that the maximum amount of threads
	// are created for an owner.
	ErrTooManyThreadsPerOwner = errors.New("number of threads per owner exceeds quota")

	log = logging.Logger("core")

	// authIgnoredMethods are not intercepted by the auth interceptor.
	authIgnoredMethods = []string{
		"/api.hubd.pb.APIService/Signup",
		"/api.hubd.pb.APIService/Signin",
		"/api.hubd.pb.APIService/IsUsernameAvailable",
	}

	// usageIgnoredMethods are not intercepted by the usage interceptor.
	usageIgnoredMethods = []string{
		"/api.hubd.pb.APIService/Signout",
		"/api.hubd.pb.APIService/DestroyAccount",
		"/api.hubd.pb.APIService/SetupBilling",
		"/api.hubd.pb.APIService/GetBillingSession",
		"/api.hubd.pb.APIService/GetBillingInfo",
	}

	// blockMethods are always blocked by auth.
	blockMethods = []string{
		"/threads.pb.API/ListDBs",
	}

	healthServiceName = "health.rpc.RPCService"
	netServiceName    = "net.rpc.RPCService"
	ffsServiceName    = "ffs.rpc.RPCService"
	walletServiceName = "wallet.rpc.RPCService"

	// allowedPowMethods are methods allowed to be directly proxied through to powergate service.
	allowedPowMethods = map[string][]string{
		healthServiceName: {
			"Check",
		},
		netServiceName: {
			"Peers",
			"FindPeer",
			"Connectedness",
		},
		ffsServiceName: {
			"Addrs",
			"Info",
			"Show",
			"ShowAll",
			"ListStorageDealRecords",
			"ListRetrievalDealRecords",
		},
		walletServiceName: {
			"Balance",
		},
	}

	// allowedCrossUserMethods are methods allowed to be called by users who do not own the target thread.
	allowedCrossUserMethods = []string{
		"/threads.pb.API/Create",
		"/threads.pb.API/Verify",
		"/threads.pb.API/Save",
		"/threads.pb.API/Delete",
		"/threads.pb.API/Has",
		"/threads.pb.API/Find",
		"/threads.pb.API/FindByID",
		"/threads.pb.API/ReadTransaction",
		"/threads.pb.API/WriteTransaction",
		"/threads.pb.API/Listen",
		"/api.bucketsd.pb.APIService/Root",
		"/api.bucketsd.pb.APIService/Links",
		"/api.bucketsd.pb.APIService/ListPath",
		"/api.bucketsd.pb.APIService/PushPath",
		"/api.bucketsd.pb.APIService/PullPath",
		"/api.bucketsd.pb.APIService/SetPath",
		"/api.bucketsd.pb.APIService/RemovePath",
		"/api.bucketsd.pb.APIService/PullPathAccessRoles",
		"/api.bucketsd.pb.APIService/PushPathAccessRoles",
	}
)

type Textile struct {
	collections *mdb.Collections

	ts tc.NetBoostrapper

	th  *threads.Client
	thn *netclient.Client
	bc  *billing.Client
	pc  *pow.Client

	bucks *tdb.Buckets
	mail  *tdb.Mail

	archiveTracker *archive.Tracker
	buckLocks      *nutil.SemaphorePool

	ipnsm *ipns.Manager
	dnsm  *dns.Manager

	server *grpc.Server
	proxy  *http.Server

	gateway            *gateway.Gateway
	internalHubSession string
	emailSessionBus    *broadcast.Broadcaster

	conf Config
}

type Config struct {
	RepoPath string

	AddrAPI          ma.Multiaddr
	AddrAPIProxy     ma.Multiaddr
	AddrThreadsHost  ma.Multiaddr
	AddrIPFSAPI      ma.Multiaddr
	AddrGatewayHost  ma.Multiaddr
	AddrGatewayURL   string
	AddrBillingAPI   string
	AddrPowergateAPI string
	AddrMongoURI     string

	ThreadsConnManager connmgr.ConnManager

	UseSubdomains bool

	MongoName string

	DNSDomain string
	DNSZoneID string
	DNSToken  string

	EmailFrom          string
	EmailDomain        string
	EmailAPIKey        string
	EmailSessionSecret string

	MaxBucketSize            int64
	MaxNumberThreadsPerOwner int

	Hub   bool
	Debug bool
}

func NewTextile(ctx context.Context, conf Config) (*Textile, error) {
	if conf.Debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"core":        logging.LevelDebug,
			"hubapi":      logging.LevelDebug,
			"bucketsapi":  logging.LevelDebug,
			"usersapi":    logging.LevelDebug,
			"pow-archive": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}
	t := &Textile{
		conf:               conf,
		internalHubSession: util.MakeToken(32),
	}

	// Configure clients
	ic, err := httpapi.NewApi(conf.AddrIPFSAPI)
	if err != nil {
		return nil, err
	}
	if conf.AddrPowergateAPI != "" {
		if t.pc, err = pow.NewClient(conf.AddrPowergateAPI); err != nil {
			return nil, err
		}
	}
	if conf.DNSToken != "" {
		t.dnsm, err = dns.NewManager(conf.DNSDomain, conf.DNSZoneID, conf.DNSToken, conf.Debug)
		if err != nil {
			return nil, err
		}
	}
	t.collections, err = mdb.NewCollections(ctx, conf.AddrMongoURI, conf.MongoName, conf.Hub)
	if err != nil {
		return nil, err
	}
	t.ipnsm, err = ipns.NewManager(t.collections.IPNSKeys, ic.Key(), ic.Name(), conf.Debug)
	if err != nil {
		return nil, err
	}

	// Configure threads
	netOptions := []tc.NetOption{
		tc.WithNetHostAddr(conf.AddrThreadsHost),
		tc.WithNetDebug(conf.Debug),
	}
	if conf.ThreadsConnManager != nil {
		netOptions = append(netOptions, tc.WithConnectionManager(conf.ThreadsConnManager))
	}
	t.ts, err = tc.DefaultNetwork(
		conf.RepoPath,
		netOptions...,
	)
	if err != nil {
		return nil, err
	}

	// Configure gRPC server
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	if err != nil {
		return nil, err
	}

	// Start threads clients
	t.th, err = threads.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{}))
	if err != nil {
		return nil, err
	}
	t.thn, err = netclient.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{}))
	if err != nil {
		return nil, err
	}
	t.bucks, err = tdb.NewBuckets(t.th, t.pc, t.collections.BucketArchives)
	if err != nil {
		return nil, err
	}
	t.mail, err = tdb.NewMail(t.th)
	if err != nil {
		return nil, err
	}

	// Configure gRPC services
	ts, err := dbapi.NewService(t.ts, dbapi.Config{
		RepoPath: conf.RepoPath,
		Debug:    conf.Debug,
	})
	if err != nil {
		return nil, err
	}
	ns, err := netapi.NewService(t.ts, netapi.Config{
		Debug: conf.Debug,
	})
	if err != nil {
		return nil, err
	}

	// Configure a billing client
	if conf.AddrBillingAPI != "" {
		t.bc, err = billing.NewClient(conf.AddrBillingAPI, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
	}

	var hs *hubd.Service
	var us *usersd.Service
	if conf.Hub {
		ec, err := email.NewClient(conf.EmailFrom, conf.EmailDomain, conf.EmailAPIKey, conf.Debug)
		if err != nil {
			return nil, err
		}
		t.emailSessionBus = broadcast.NewBroadcaster(0)
		hs = &hubd.Service{
			Collections:        t.collections,
			Threads:            t.th,
			ThreadsNet:         t.thn,
			GatewayURL:         conf.AddrGatewayURL,
			EmailClient:        ec,
			EmailSessionBus:    t.emailSessionBus,
			EmailSessionSecret: conf.EmailSessionSecret,
			IPFSClient:         ic,
			IPNSManager:        t.ipnsm,
			BillingClient:      t.bc,
			PowergateClient:    t.pc,
		}
		us = &usersd.Service{
			Collections: t.collections,
			Mail:        t.mail,
		}
	}
	if conf.Hub {
		t.archiveTracker, err = archive.New(t.collections, t.bucks, t.pc, t.internalHubSession)
		if err != nil {
			return nil, err
		}
	}
	t.buckLocks = nutil.NewSemaphorePool(1)
	bs := &bucketsd.Service{
		Collections:        t.collections,
		Buckets:            t.bucks,
		GatewayURL:         conf.AddrGatewayURL,
		GatewayBucketsHost: conf.DNSDomain,
		IPFSClient:         ic,
		IPNSManager:        t.ipnsm,
		PowergateClient:    t.pc,
		ArchiveTracker:     t.archiveTracker,
		Semaphores:         t.buckLocks,
		MaxBucketSize:      conf.MaxBucketSize,
	}

	// Start serving
	ptarget, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPIProxy)
	if err != nil {
		return nil, err
	}
	var opts []grpc.ServerOption
	if conf.Hub {
		var powStub *grpcdynamic.Stub
		var healthServiceDesc *desc.ServiceDescriptor
		var netServiceDesc *desc.ServiceDescriptor
		var ffsServiceDesc *desc.ServiceDescriptor
		var walletServiceDesc *desc.ServiceDescriptor
		if conf.AddrPowergateAPI != "" {
			if powStub, err = createPowStub(conf.AddrPowergateAPI); err != nil {
				return nil, err
			}
			if healthServiceDesc, err = createServiceDesciptor("health/rpc/rpc.proto", healthServiceName); err != nil {
				return nil, err
			}
			if netServiceDesc, err = createServiceDesciptor("net/rpc/rpc.proto", netServiceName); err != nil {
				return nil, err
			}
			if ffsServiceDesc, err = createServiceDesciptor("ffs/rpc/rpc.proto", ffsServiceName); err != nil {
				return nil, err
			}
			if walletServiceDesc, err = createServiceDesciptor("wallet/rpc/rpc.proto", walletServiceName); err != nil {
				return nil, err
			}
		}
		opts = []grpc.ServerOption{
			grpcm.WithUnaryServerChain(
				auth.UnaryServerInterceptor(t.authFunc),
				t.threadInterceptor(),
				unaryServerInterceptor(t.preUsageFunc, t.postUsageFunc),
				powInterceptor(healthServiceName, allowedPowMethods[healthServiceName], healthServiceDesc, powStub, t.pc, t.collections),
				powInterceptor(netServiceName, allowedPowMethods[netServiceName], netServiceDesc, powStub, t.pc, t.collections),
				powInterceptor(ffsServiceName, allowedPowMethods[ffsServiceName], ffsServiceDesc, powStub, t.pc, t.collections),
				powInterceptor(walletServiceName, allowedPowMethods[walletServiceName], walletServiceDesc, powStub, t.pc, t.collections),
			),
			grpcm.WithStreamServerChain(
				auth.StreamServerInterceptor(t.authFunc),
				streamServerInterceptor(t.preUsageFunc, t.postUsageFunc),
			),
			grpc.StatsHandler(&StatsHandler{t: t}),
		}
	} else {
		opts = []grpc.ServerOption{
			grpcm.WithUnaryServerChain(auth.UnaryServerInterceptor(t.noAuthFunc)),
			grpcm.WithStreamServerChain(auth.StreamServerInterceptor(t.noAuthFunc)),
		}
	}
	t.server = grpc.NewServer(opts...)
	listener, err := net.Listen("tcp", target)
	if err != nil {
		return nil, err
	}
	go func() {
		dbpb.RegisterAPIServer(t.server, ts)
		netpb.RegisterAPIServer(t.server, ns)
		if conf.Hub {
			hpb.RegisterAPIServiceServer(t.server, hs)
			upb.RegisterAPIServiceServer(t.server, us)
			healthRpc.RegisterRPCServiceServer(t.server, &healthRpc.UnimplementedRPCServiceServer{})
			netRpc.RegisterRPCServiceServer(t.server, &netRpc.UnimplementedRPCServiceServer{})
			ffsRpc.RegisterRPCServiceServer(t.server, &ffsRpc.UnimplementedRPCServiceServer{})
			walletRpc.RegisterRPCServiceServer(t.server, &walletRpc.UnimplementedRPCServiceServer{})
		}
		bpb.RegisterAPIServiceServer(t.server, bs)
		if err := t.server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("serve error: %v", err)
		}
		if err := ts.Close(); err != nil {
			log.Fatalf("error closing thread service: %v", err)
		}
	}()
	webrpc := grpcweb.WrapServer(
		t.server,
		grpcweb.WithOriginFunc(func(origin string) bool {
			return true
		}),
		grpcweb.WithAllowedRequestHeaders([]string{"Origin"}),
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
			return true
		}))
	t.proxy = &http.Server{
		Addr: ptarget,
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

	// Configure gateway
	t.gateway, err = gateway.NewGateway(gateway.Config{
		Addr:            conf.AddrGatewayHost,
		URL:             conf.AddrGatewayURL,
		Subdomains:      conf.UseSubdomains,
		BucketsDomain:   conf.DNSDomain,
		APIAddr:         conf.AddrAPI,
		APISession:      t.internalHubSession,
		Collections:     t.collections,
		IPFSClient:      ic,
		EmailSessionBus: t.emailSessionBus,
		Hub:             conf.Hub,
		Debug:           conf.Debug,
	})
	if err != nil {
		return nil, err
	}
	t.gateway.Start()

	log.Info("started")

	return t, nil
}

func (t *Textile) Bootstrap() {
	t.ts.Bootstrap(tutil.DefaultBoostrapPeers())
}

func (t *Textile) Close(force bool) error {
	if t.emailSessionBus != nil {
		t.emailSessionBus.Discard()
	}
	if err := t.gateway.Stop(); err != nil {
		return err
	}
	t.buckLocks.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := t.proxy.Shutdown(ctx); err != nil {
		return err
	}
	if force {
		t.server.Stop()
	} else {
		t.server.GracefulStop()
	}
	if t.archiveTracker != nil {
		if err := t.archiveTracker.Close(); err != nil {
			return err
		}
	}
	if err := t.bucks.Close(); err != nil {
		return err
	}
	if err := t.th.Close(); err != nil {
		return err
	}
	if err := t.ts.Close(); err != nil {
		return err
	}
	if t.bc != nil {
		if err := t.bc.Close(); err != nil {
			return err
		}
	}
	if t.pc != nil {
		if err := t.pc.Close(); err != nil {
			return err
		}
	}
	if err := t.collections.Close(); err != nil {
		return err
	}
	t.ipnsm.Cancel()
	return nil
}

func (t *Textile) HostID() peer.ID {
	return t.ts.Host().ID()
}

func createServiceDesciptor(file string, serviceName string) (*desc.ServiceDescriptor, error) {
	fileDesc, err := desc.LoadFileDescriptor(file)
	if err != nil {
		return nil, err
	}
	serviceDesc := fileDesc.FindService(serviceName)
	if serviceDesc == nil {
		return nil, fmt.Errorf("no service description found")
	}
	return serviceDesc, nil
}
