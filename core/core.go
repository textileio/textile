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
	"github.com/ipfs/go-datastore"
	ktipfs "github.com/ipfs/go-datastore/keytransform"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log/v2"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	connmgr "github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mongods "github.com/textileio/go-ds-mongo"
	dbapi "github.com/textileio/go-threads/api"
	threads "github.com/textileio/go-threads/api/client"
	dbpb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/broadcast"
	tc "github.com/textileio/go-threads/common"
	kt "github.com/textileio/go-threads/db/keytransform"
	netapi "github.com/textileio/go-threads/net/api"
	netclient "github.com/textileio/go-threads/net/api/client"
	netpb "github.com/textileio/go-threads/net/api/pb"
	nutil "github.com/textileio/go-threads/net/util"
	tutil "github.com/textileio/go-threads/util"
	pow "github.com/textileio/powergate/v2/api/client"
	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	billing "github.com/textileio/textile/v2/api/billingd/client"
	"github.com/textileio/textile/v2/api/bucketsd"
	bpb "github.com/textileio/textile/v2/api/bucketsd/pb"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/api/hubd"
	hpb "github.com/textileio/textile/v2/api/hubd/pb"
	"github.com/textileio/textile/v2/api/usersd"
	upb "github.com/textileio/textile/v2/api/usersd/pb"
	"github.com/textileio/textile/v2/buckets/archive"
	"github.com/textileio/textile/v2/buckets/archive/retrieval"
	"github.com/textileio/textile/v2/buckets/archive/tracker"
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
	}

	// blockMethods are always blocked by auth.
	blockMethods = []string{
		"/threads.pb.API/ListDBs",
	}

	powergateServiceName = "powergate.user.v1.UserService"

	// ToDo: Add support for streaming methods and double check the list for completeness.

	// allowedPowMethods are methods allowed to be directly proxied through to powergate service.
	allowedPowMethods = map[string][]string{
		powergateServiceName: {
			"Addresses",
			"Balance",
			"SignMessage",
			"VerifyMessage",
			"CidInfo",
			"StorageDealRecords",
			"RetrievalDealRecords",
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
		"/api.bucketsd.pb.APIService/MovePath",
		"/api.bucketsd.pb.APIService/RemovePath",
		"/api.bucketsd.pb.APIService/PullPathAccessRoles",
		"/api.bucketsd.pb.APIService/PushPathAccessRoles",
	}
)

type Textile struct {
	collections *mdb.Collections

	tn tc.NetBoostrapper
	ts kt.TxnDatastoreExtended

	th  *threads.Client
	thn *netclient.Client
	bc  *billing.Client
	pc  *pow.Client

	bucks *tdb.Buckets
	mail  *tdb.Mail

	archiveTracker *tracker.Tracker
	filRetrieval   *retrieval.FilRetrieval
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
	Hub   bool
	Debug bool

	// Addresses
	AddrAPI          ma.Multiaddr
	AddrAPIProxy     ma.Multiaddr
	AddrMongoURI     string
	AddrMongoName    string
	AddrThreadsHost  ma.Multiaddr
	AddrGatewayHost  ma.Multiaddr
	AddrGatewayURL   string
	AddrIPFSAPI      ma.Multiaddr
	AddrBillingAPI   string
	AddrPowergateAPI string

	// Buckets
	MaxBucketArchiveRepFactor int
	MaxBucketArchiveSize      int64
	MinBucketArchiveSize      int64

	// Threads
	MaxNumberThreadsPerOwner int
	ThreadsConnManager       connmgr.ConnManager

	// IPNS
	IPNSRepublishSchedule    string
	IPNSRepublishConcurrency int

	// Powergate
	PowergateAdminToken string

	// Archives
	ArchiveJobPollIntervalSlow time.Duration
	ArchiveJobPollIntervalFast time.Duration

	// Gateway
	UseSubdomains bool

	// Cloudflare
	DNSDomain string
	DNSZoneID string
	DNSToken  string

	// Customer.io
	CustomerioAPIKey      string
	CustomerioConfirmTmpl string
	CustomerioInviteTmpl  string
	EmailSessionSecret    string
}

func NewTextile(ctx context.Context, conf Config, opts ...Option) (*Textile, error) {
	var args Options
	for _, opt := range opts {
		opt(&args)
	}

	if conf.Debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"core":          logging.LevelDebug,
			"hubapi":        logging.LevelDebug,
			"bucketsapi":    logging.LevelDebug,
			"usersapi":      logging.LevelDebug,
			"job-tracker":   logging.LevelDebug,
			"fil-retrieval": logging.LevelDebug,
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
	t.collections, err = mdb.NewCollections(ctx, conf.AddrMongoURI, conf.AddrMongoName, conf.Hub)
	if err != nil {
		return nil, err
	}
	t.ipnsm, err = ipns.NewManager(t.collections.IPNSKeys, ic.Key(), ic.Name(), conf.IPNSRepublishConcurrency, conf.Debug)
	if err != nil {
		return nil, err
	}

	// Configure threads
	netOptions := []tc.NetOption{
		tc.WithNetHostAddr(conf.AddrThreadsHost),
		tc.WithNetDebug(conf.Debug),
	}
	if args.ThreadsMongoUri != "" {
		netOptions = append(netOptions, tc.WithNetMongoPersistence(args.ThreadsMongoUri, args.ThreadsMongoDB))
	} else {
		netOptions = append(netOptions, tc.WithNetBadgerPersistence(args.ThreadsBadgerRepoPath))
	}
	if conf.ThreadsConnManager != nil {
		netOptions = append(netOptions, tc.WithConnectionManager(conf.ThreadsConnManager))
	}
	t.tn, err = tc.DefaultNetwork(netOptions...)
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
	if args.ThreadsMongoUri != "" {
		t.ts, err = mongods.New(
			ctx,
			args.ThreadsMongoUri,
			args.ThreadsMongoDB,
			mongods.WithCollName("eventstore"),
		)
		if err != nil {
			return nil, err
		}
	} else {
		t.ts, err = tutil.NewBadgerDatastore(args.ThreadsBadgerRepoPath, "eventstore", false)
		if err != nil {
			return nil, err
		}
	}
	ts, err := dbapi.NewService(t.ts, t.tn, dbapi.Config{
		Debug: conf.Debug,
	})
	if err != nil {
		return nil, err
	}
	ns, err := netapi.NewService(t.tn, netapi.Config{
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

	jobFinalizedEvents := make(chan archive.JobEvent)
	t.archiveTracker, err = tracker.New(
		t.collections,
		t.bucks,
		t.pc,
		t.internalHubSession,
		conf.ArchiveJobPollIntervalSlow,
		conf.ArchiveJobPollIntervalFast,
		jobFinalizedEvents,
	)
	if err != nil {
		return nil, err
	}

	filRetrievalDS := kt.WrapTxnDatastore(t.ts, ktipfs.PrefixTransform{
		Prefix: datastore.NewKey("buckets/filretrieval"),
	})
	t.filRetrieval, err = retrieval.NewFilRetrieval(filRetrievalDS, t.pc, t.archiveTracker, jobFinalizedEvents, t.internalHubSession)
	if err != nil {
		return nil, err
	}

	var hs *hubd.Service
	var us *usersd.Service
	if conf.Hub {
		cio, err := email.NewClient(
			email.Config{
				ConfirmTmpl: conf.CustomerioConfirmTmpl,
				InviteTmpl:  conf.CustomerioInviteTmpl,
				APIKey:      conf.CustomerioAPIKey,
				Debug:       conf.Debug,
			},
		)
		if err != nil {
			return nil, err
		}

		t.emailSessionBus = broadcast.NewBroadcaster(0)
		hs = &hubd.Service{
			Collections:         t.collections,
			Threads:             t.th,
			ThreadsNet:          t.thn,
			GatewayURL:          conf.AddrGatewayURL,
			EmailClient:         cio,
			EmailSessionBus:     t.emailSessionBus,
			EmailSessionSecret:  conf.EmailSessionSecret,
			IPFSClient:          ic,
			IPNSManager:         t.ipnsm,
			BillingClient:       t.bc,
			PowergateClient:     t.pc,
			PowergateAdminToken: conf.PowergateAdminToken,
		}
		us = &usersd.Service{
			Collections:     t.collections,
			Mail:            t.mail,
			BillingClient:   t.bc,
			FilRetrieval:    t.filRetrieval,
			PowergateClient: t.pc,
		}
	}

	t.buckLocks = nutil.NewSemaphorePool(1)
	bs := &bucketsd.Service{
		Collections:               t.collections,
		Buckets:                   t.bucks,
		GatewayURL:                conf.AddrGatewayURL,
		GatewayBucketsHost:        conf.DNSDomain,
		IPFSClient:                ic,
		IPNSManager:               t.ipnsm,
		PowergateClient:           t.pc,
		PowergateAdminToken:       conf.PowergateAdminToken,
		ArchiveTracker:            t.archiveTracker,
		Semaphores:                t.buckLocks,
		MaxBucketArchiveRepFactor: conf.MaxBucketArchiveRepFactor,
		MaxBucketArchiveSize:      conf.MaxBucketArchiveSize,
		MinBucketArchiveSize:      conf.MinBucketArchiveSize,
		FilRetrieval:              t.filRetrieval,
	}

	// We can avoid the chicken-egg-problem of below line in the future.
	// For more info, see "TODO(**)" in buckd/service.go
	t.filRetrieval.SetBucketCreator(bs)
	t.filRetrieval.RunDaemon()

	// Start serving
	ptarget, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPIProxy)
	if err != nil {
		return nil, err
	}
	var grpcopts []grpc.ServerOption
	if conf.Hub {
		var powStub *grpcdynamic.Stub
		var powergateServiceDesc *desc.ServiceDescriptor
		if conf.AddrPowergateAPI != "" {
			if powStub, err = createPowStub(conf.AddrPowergateAPI); err != nil {
				return nil, err
			}
			if powergateServiceDesc, err = createServiceDesciptor(
				"powergate/user/v1/user.proto",
				powergateServiceName,
			); err != nil {
				return nil, err
			}
		}
		grpcopts = []grpc.ServerOption{
			grpcm.WithUnaryServerChain(
				auth.UnaryServerInterceptor(t.authFunc),
				unaryServerInterceptor(t.preUsageFunc, t.postUsageFunc),
				t.threadInterceptor(),
				powInterceptor(
					powergateServiceName,
					allowedPowMethods[powergateServiceName],
					powergateServiceDesc,
					powStub,
					t.pc,
					conf.PowergateAdminToken,
					t.collections,
				),
			),
			grpcm.WithStreamServerChain(
				auth.StreamServerInterceptor(t.authFunc),
				streamServerInterceptor(t.preUsageFunc, t.postUsageFunc),
			),
			grpc.StatsHandler(&StatsHandler{t: t}),
		}
	} else {
		grpcopts = []grpc.ServerOption{
			grpcm.WithUnaryServerChain(auth.UnaryServerInterceptor(t.noAuthFunc)),
			grpcm.WithStreamServerChain(auth.StreamServerInterceptor(t.noAuthFunc)),
		}
	}
	t.server = grpc.NewServer(grpcopts...)
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
			userPb.RegisterUserServiceServer(t.server, &userPb.UnimplementedUserServiceServer{})
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

	// Start republishing ipns keys
	if err := t.ipnsm.StartRepublishing(conf.IPNSRepublishSchedule); err != nil {
		return nil, err
	}

	log.Info("started")

	return t, nil
}

func (t *Textile) Bootstrap() {
	t.tn.Bootstrap(tutil.DefaultBoostrapPeers())
}

func (t *Textile) Close() error {
	if t.emailSessionBus != nil {
		t.emailSessionBus.Discard()
	}
	log.Info("session bus was discarded")

	if err := t.gateway.Stop(); err != nil {
		return err
	}

	t.buckLocks.Stop()
	log.Info("locking buckets")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := t.proxy.Shutdown(ctx); err != nil {
		return err
	}
	log.Info("gRPC proxy was shutdown")

	stopped := make(chan struct{})
	go func() {
		t.server.GracefulStop()
		close(stopped)
	}()
	timer := time.NewTimer(10 * time.Second)
	select {
	case <-timer.C:
		t.server.Stop()
	case <-stopped:
		timer.Stop()
	}
	log.Info("gRPC was shutdown")

	log.Info("closing fil-retrieval module")
	if err := t.filRetrieval.Close(); err != nil {
		log.Errorf("closing fil-retrieval module: %s", err)
	} else {
		log.Info("fil-retrieval was shutdown")
	}

	if err := t.th.Close(); err != nil {
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
	log.Info("local clients were shutdown")

	t.ipnsm.Close()
	if t.archiveTracker != nil {
		if err := t.archiveTracker.Close(); err != nil {
			return err
		}
	}
	if err := t.bucks.Close(); err != nil {
		return err
	}
	log.Info("buckets was shutdown")

	if err := t.tn.Close(); err != nil {
		return err
	}
	if err := t.ts.Close(); err != nil {
		return err
	}
	log.Info("threads was shutdown")

	if err := t.collections.Close(); err != nil {
		return err
	}
	log.Info("mongo collections were shutdown")

	return nil
}

func (t *Textile) HostID() peer.ID {
	return t.tn.Host().ID()
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
