package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/golang/protobuf/proto"
	grpcm "github.com/grpc-ecosystem/go-grpc-middleware"
	auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	connmgr "github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	dbapi "github.com/textileio/go-threads/api"
	threads "github.com/textileio/go-threads/api/client"
	dbpb "github.com/textileio/go-threads/api/pb"
	"github.com/textileio/go-threads/broadcast"
	tc "github.com/textileio/go-threads/common"
	"github.com/textileio/go-threads/core/thread"
	netapi "github.com/textileio/go-threads/net/api"
	netclient "github.com/textileio/go-threads/net/api/client"
	netpb "github.com/textileio/go-threads/net/api/pb"
	tutil "github.com/textileio/go-threads/util"
	powc "github.com/textileio/powergate/api/client"
	ffsRpc "github.com/textileio/powergate/ffs/rpc"
	"github.com/textileio/textile/api/buckets"
	bpb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/api/hub"
	hpb "github.com/textileio/textile/api/hub/pb"
	"github.com/textileio/textile/api/users"
	upb "github.com/textileio/textile/api/users/pb"
	"github.com/textileio/textile/buckets/archive"
	"github.com/textileio/textile/dns"
	"github.com/textileio/textile/email"
	"github.com/textileio/textile/gateway"
	"github.com/textileio/textile/ipns"
	mdb "github.com/textileio/textile/mongodb"
	tdb "github.com/textileio/textile/threaddb"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrTooManyThreadsPerOwner indicates that the maximum amount of threads
	// are created for an owner.
	ErrTooManyThreadsPerOwner = errors.New("number of threads per owner exceeds quota")

	log = logging.Logger("core")

	// ignoreMethods are not intercepted by the auth.
	ignoreMethods = []string{
		"/hub.pb.API/Signup",
		"/hub.pb.API/Signin",
		"/hub.pb.API/IsUsernameAvailable",
	}

	// blockMethods are always blocked by auth.
	blockMethods = []string{
		"/threads.pb.API/ListDBs",
	}

	// WSPingInterval controls the WebSocket keepalive pinging interval. Must be >= 1s.
	WSPingInterval = time.Second * 5
)

type Textile struct {
	collections *mdb.Collections

	ts             tc.NetBoostrapper
	th             *threads.Client
	thn            *netclient.Client
	bucks          *tdb.Buckets
	mail           *tdb.Mail
	powc           *powc.Client
	powStub        grpcdynamic.Stub
	ffsServiceDesc *desc.ServiceDescriptor
	archiveTracker *archive.Tracker

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
	AddrPowergateAPI string
	AddrMongoURI     string

	UseSubdomains bool

	MongoName string

	DNSDomain string
	DNSZoneID string
	DNSToken  string

	EmailFrom          string
	EmailDomain        string
	EmailAPIKey        string
	EmailSessionSecret string

	BucketsMaxSize            int64
	BucketsTotalMaxSize       int64
	BucketsMaxNumberPerThread int

	ThreadsMaxNumberPerOwner int

	Hub   bool
	Debug bool

	ThreadsConnManager connmgr.ConnManager
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
		if t.powc, err = powc.NewClient(conf.AddrPowergateAPI); err != nil {
			return nil, err
		}
		if t.powStub, err = createPowStub(conf.AddrPowergateAPI); err != nil {
			return nil, err
		}
		if t.ffsServiceDesc, err = createFFSServiceDesciptor(); err != nil {
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
	t.bucks, err = tdb.NewBuckets(t.th, t.powc, t.collections.BucketArchives)
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

	var hs *hub.Service
	var us *users.Service
	if conf.Hub {
		ec, err := email.NewClient(conf.EmailFrom, conf.EmailDomain, conf.EmailAPIKey, conf.Debug)
		if err != nil {
			return nil, err
		}
		t.emailSessionBus = broadcast.NewBroadcaster(0)
		hs = &hub.Service{
			Collections:        t.collections,
			Threads:            t.th,
			ThreadsNet:         t.thn,
			GatewayURL:         conf.AddrGatewayURL,
			EmailClient:        ec,
			EmailSessionBus:    t.emailSessionBus,
			EmailSessionSecret: conf.EmailSessionSecret,
			IPFSClient:         ic,
			IPNSManager:        t.ipnsm,
			DNSManager:         t.dnsm,
			Pow:                t.powc,
		}
		us = &users.Service{
			Collections: t.collections,
			Mail:        t.mail,
		}
	}
	if conf.Hub {
		t.archiveTracker, err = archive.New(t.collections, t.bucks, t.powc, t.internalHubSession)
		if err != nil {
			return nil, err
		}
	}
	bs := &buckets.Service{
		Collections:               t.collections,
		Buckets:                   t.bucks,
		BucketsMaxSize:            conf.BucketsMaxSize,
		BucketsTotalMaxSize:       conf.BucketsTotalMaxSize,
		BucketsMaxNumberPerThread: conf.BucketsMaxNumberPerThread,
		GatewayURL:                conf.AddrGatewayURL,
		IPFSClient:                ic,
		IPNSManager:               t.ipnsm,
		DNSManager:                t.dnsm,
		PGClient:                  t.powc,
		ArchiveTracker:            t.archiveTracker,
	}

	ffsService := ffsRpc.UnimplementedRPCServiceServer{}

	// Start serving
	ptarget, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPIProxy)
	if err != nil {
		return nil, err
	}
	var opts []grpc.ServerOption
	if conf.Hub {
		opts = []grpc.ServerOption{
			grpcm.WithUnaryServerChain(auth.UnaryServerInterceptor(t.authFunc), t.threadInterceptor()),
			grpcm.WithStreamServerChain(auth.StreamServerInterceptor(t.authFunc)),
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
			hpb.RegisterAPIServer(t.server, hs)
			upb.RegisterAPIServer(t.server, us)
		}
		bpb.RegisterAPIServer(t.server, bs)
		ffsRpc.RegisterRPCServiceServer(t.server, &ffsService)
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
		grpcweb.WithWebsocketPingInterval(WSPingInterval),
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
	if t.powc != nil {
		if err := t.powc.Close(); err != nil {
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

func (t *Textile) authFunc(ctx context.Context) (context.Context, error) {
	method, _ := grpc.Method(ctx)
	for _, ignored := range ignoreMethods {
		if method == ignored {
			return ctx, nil
		}
	}
	for _, block := range blockMethods {
		if method == block {
			return nil, status.Error(codes.PermissionDenied, "Method is not accessible")
		}
	}

	if threadID, ok := common.ThreadIDFromMD(ctx); ok {
		ctx = common.NewThreadIDContext(ctx, threadID)
	}
	if threadName, ok := common.ThreadNameFromMD(ctx); ok {
		ctx = common.NewThreadNameContext(ctx, threadName)
	}
	if threadToken, err := thread.NewTokenFromMD(ctx); err != nil {
		return nil, err
	} else {
		ctx = thread.NewTokenContext(ctx, threadToken)
	}

	sid, ok := common.SessionFromMD(ctx)
	if ok {
		ctx = common.NewSessionContext(ctx, sid)
		if sid == t.internalHubSession {
			return ctx, nil
		}
		session, err := t.collections.Sessions.Get(ctx, sid)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "Invalid session")
		}
		if time.Now().After(session.ExpiresAt) {
			return nil, status.Error(codes.Unauthenticated, "Expired session")
		}
		if err := t.collections.Sessions.Touch(ctx, session.ID); err != nil {
			return nil, err
		}
		ctx = mdb.NewSessionContext(ctx, session)

		dev, err := t.collections.Accounts.Get(ctx, session.Owner)
		if err != nil {
			return nil, status.Error(codes.NotFound, "User not found")
		}
		ctx = mdb.NewDevContext(ctx, dev)

		orgSlug, ok := common.OrgSlugFromMD(ctx)
		if ok {
			isMember, err := t.collections.Accounts.IsMember(ctx, orgSlug, dev.Key)
			if err != nil {
				return nil, err
			}
			if !isMember {
				return nil, status.Error(codes.PermissionDenied, "User is not an org member")
			} else {
				org, err := t.collections.Accounts.GetByUsername(ctx, orgSlug)
				if err != nil {
					return nil, status.Error(codes.NotFound, "Org not found")
				}
				ctx = mdb.NewOrgContext(ctx, org)
				ctx = common.NewOrgSlugContext(ctx, orgSlug)
				ctx = thread.NewTokenContext(ctx, org.Token)
			}
		} else {
			ctx = thread.NewTokenContext(ctx, dev.Token)
		}
	} else if k, ok := common.APIKeyFromMD(ctx); ok {
		key, err := t.collections.APIKeys.Get(ctx, k)
		if err != nil || !key.Valid {
			return nil, status.Error(codes.NotFound, "API key not found or is invalid")
		}
		ctx = common.NewAPIKeyContext(ctx, k)
		if key.Secure {
			msg, sig, ok := common.APISigFromMD(ctx)
			if !ok {
				return nil, status.Error(codes.Unauthenticated, "API key signature required")
			} else {
				ctx = common.NewAPISigContext(ctx, msg, sig)
				if !common.ValidateAPISigContext(ctx, key.Secret) {
					return nil, status.Error(codes.Unauthenticated, "Bad API key signature")
				}
			}
		}
		switch key.Type {
		case mdb.AccountKey:
			acc, err := t.collections.Accounts.Get(ctx, key.Owner)
			if err != nil {
				return nil, status.Error(codes.NotFound, "Account not found")
			}
			switch acc.Type {
			case mdb.Dev:
				ctx = mdb.NewDevContext(ctx, acc)
			case mdb.Org:
				ctx = mdb.NewOrgContext(ctx, acc)
			}
			ctx = thread.NewTokenContext(ctx, acc.Token)
		case mdb.UserKey:
			token, ok := thread.TokenFromContext(ctx)
			if ok {
				var claims jwt.StandardClaims
				if _, _, err = new(jwt.Parser).ParseUnverified(string(token), &claims); err != nil {
					return nil, status.Error(codes.PermissionDenied, "Bad authorization")
				}
				ukey := &thread.Libp2pPubKey{}
				if err = ukey.UnmarshalString(claims.Subject); err != nil {
					return nil, err
				}
				user, err := t.collections.Users.Get(ctx, ukey.PubKey)
				if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
					return nil, err
				}
				if user == nil {
					// Attach a temp user context that will be accessible in the next interceptor.
					user = &mdb.User{Key: ukey.PubKey}
				}
				ctx = mdb.NewUserContext(ctx, user)
			} else if method != "/threads.pb.API/GetToken" && method != "/threads.net.pb.API/GetToken" {
				return nil, status.Error(codes.Unauthenticated, "Token required")
			}
		}
		ctx = mdb.NewAPIKeyContext(ctx, key)
	} else {
		return nil, status.Error(codes.Unauthenticated, "Session or API key required")
	}
	return ctx, nil
}

func (t *Textile) noAuthFunc(ctx context.Context) (context.Context, error) {
	if threadID, ok := common.ThreadIDFromMD(ctx); ok {
		ctx = common.NewThreadIDContext(ctx, threadID)
	}
	if threadToken, err := thread.NewTokenFromMD(ctx); err != nil {
		return nil, err
	} else {
		ctx = thread.NewTokenContext(ctx, threadToken)
	}
	return ctx, nil
}

func (t *Textile) powergateInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if !strings.Contains(info.FullMethod, "ffs.rpc.RPCService") {
		return handler(ctx, req)
	}

	var ffsInfo *mdb.FFSInfo
	if org, ok := mdb.OrgFromContext(ctx); ok {
		ffsInfo = org.FFSInfo
	} else if dev, ok := mdb.DevFromContext(ctx); ok {
		ffsInfo = dev.FFSInfo
	} else if user, ok := mdb.UserFromContext(ctx); ok {
		ffsInfo = user.FFSInfo
	}

	if ffsInfo == nil {
		return nil, fmt.Errorf("no account or no FFS info associated with account")
	}

	ffsCtx := context.WithValue(ctx, powc.AuthKey, ffsInfo.Token)

	methodParts := strings.Split(info.FullMethod, "/")
	if len(methodParts) < 2 {
		return nil, fmt.Errorf("error parsing method string %s", info.FullMethod)
	}
	methodName := methodParts[len(methodParts)-1]
	methodDesc := t.ffsServiceDesc.FindMethodByName(methodName)
	if methodDesc == nil {
		return nil, fmt.Errorf("no method found for %s", methodName)
	}
	return t.powStub.InvokeRpc(ffsCtx, methodDesc, req.(proto.Message))
}

// threadInterceptor monitors for thread creation and deletion.
// Textile tracks threads against dev, org, and user accounts.
// Users must supply a valid API key from a dev/org.
func (t *Textile) threadInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		method, _ := grpc.Method(ctx)
		for _, ignored := range ignoreMethods {
			if method == ignored {
				return handler(ctx, req)
			}
		}
		for _, block := range blockMethods {
			if method == block {
				return nil, status.Error(codes.PermissionDenied, "Method is not accessible")
			}
		}
		if sid, ok := common.SessionFromContext(ctx); ok && sid == t.internalHubSession {
			return handler(ctx, req)
		}

		var owner crypto.PubKey
		if org, ok := mdb.OrgFromContext(ctx); ok {
			owner = org.Key
		} else if dev, ok := mdb.DevFromContext(ctx); ok {
			owner = dev.Key
		} else if user, ok := mdb.UserFromContext(ctx); ok {
			owner = user.Key
		}

		var newID thread.ID
		var isDB bool
		var err error
		switch method {
		case "/threads.pb.API/NewDB":
			newID, err = thread.Cast(req.(*dbpb.NewDBRequest).DbID)
			if err != nil {
				return nil, err
			}
			isDB = true
		case "/threads.pb.API/NewDBFromAddr":
			addr, err := ma.NewMultiaddrBytes(req.(*dbpb.NewDBFromAddrRequest).Addr)
			if err != nil {
				return nil, err
			}
			newID, err = thread.FromAddr(addr)
			if err != nil {
				return nil, err
			}
			isDB = true
		case "/threads.net.pb.API/CreateThread":
			newID, err = thread.Cast(req.(*netpb.CreateThreadRequest).ThreadID)
			if err != nil {
				return nil, err
			}
		case "/threads.net.pb.API/AddThread":
			addr, err := ma.NewMultiaddrBytes(req.(*netpb.AddThreadRequest).Addr)
			if err != nil {
				return nil, err
			}
			newID, err = thread.FromAddr(addr)
			if err != nil {
				return nil, err
			}
		default:
			// If we're dealing with an existing thread, make sure that the owner
			// owns the thread directly or via an API key.
			threadID, ok := common.ThreadIDFromContext(ctx)
			if ok {
				th, err := t.collections.Threads.Get(ctx, threadID, owner)
				if err != nil {
					if errors.Is(err, mongo.ErrNoDocuments) {
						return nil, status.Error(codes.NotFound, "Thread not found")
					} else {
						return nil, err
					}
				}
				if owner == nil || !owner.Equals(th.Owner) { // Linter can't see that owner can't be nil
					return nil, status.Error(codes.PermissionDenied, "User does not own thread")

				}
				key, _ := mdb.APIKeyFromContext(ctx)
				if key != nil && key.Type == mdb.UserKey {
					// Extra user check for user API keys.
					if key.Key != th.Key {
						return nil, status.Error(codes.PermissionDenied, "Bad API key")
					}
				}
			}
		}

		// Collect the user if we haven't seen them before.
		user, ok := mdb.UserFromContext(ctx)
		if ok && user.CreatedAt.IsZero() {
			var ffsInfo *mdb.FFSInfo
			if t.powc != nil {
				ffsId, ffsToken, err := t.powc.FFS.Create(ctx)
				if err != nil {
					return nil, err
				}
				ffsInfo = &mdb.FFSInfo{ID: ffsId, Token: ffsToken}
			}
			if err := t.collections.Users.Create(ctx, owner, ffsInfo); err != nil {
				return nil, err
			}
		}
		if !ok && owner != nil {
			// Add the dev/org as the user for the user API.
			ctx = mdb.NewUserContext(ctx, &mdb.User{Key: owner})
		}

		// Preemptively track the new thread ID for the owner.
		// This needs to happen before the request is handled in case there's a conflict
		// with the owner and thread name.
		if newID.Defined() {
			thds, err := t.collections.Threads.ListByOwner(ctx, owner)
			if err != nil {
				return nil, err
			}
			if t.conf.ThreadsMaxNumberPerOwner > 0 && len(thds) >= t.conf.ThreadsMaxNumberPerOwner {
				return nil, ErrTooManyThreadsPerOwner
			}
			if _, err := t.collections.Threads.Create(ctx, newID, owner, isDB); err != nil {
				return nil, err
			}
		}

		// Track the thread ID marked for deletion.
		var deleteID thread.ID
		switch method {
		case "/threads.pb.API/DeleteDB":
			deleteID, err = thread.Cast(req.(*dbpb.DeleteDBRequest).DbID)
			if err != nil {
				return nil, err
			}
			keys, err := t.collections.IPNSKeys.ListByThreadID(ctx, deleteID)
			if err != nil {
				return nil, err
			}
			if len(keys) != 0 {
				return nil, status.Error(codes.FailedPrecondition, "DB not empty (delete buckets first)")
			}
		case "/threads.net.pb.API/DeleteThread":
			deleteID, err = thread.Cast(req.(*netpb.DeleteThreadRequest).ThreadID)
			if err != nil {
				return nil, err
			}
		}

		// Let the request pass through.
		res, err := handler(ctx, req)
		if err != nil {
			// Clean up the new thread if there was an error.
			if newID.Defined() {
				if err := t.collections.Threads.Delete(ctx, newID, owner); err != nil {
					log.Errorf("error deleting thread %s: %v", newID, err)
				}
			}
			return res, err
		}

		// Clean up the tracked thread if it was deleted.
		if deleteID.Defined() {
			if err := t.collections.Threads.Delete(ctx, deleteID, owner); err != nil {
				return nil, err
			}
		}
		return res, nil
	}
}

func createPowStub(target string) (grpcdynamic.Stub, error) {
	pgConn, err := powc.CreateClientConn(target)
	if err != nil {
		return grpcdynamic.Stub{}, err
	}
	return grpcdynamic.NewStub(pgConn), nil
}

func createFFSServiceDesciptor() (*desc.ServiceDescriptor, error) {
	ffsDesc, err := desc.LoadFileDescriptor("ffs/rpc/rpc.proto")
	if err != nil {
		return nil, err
	}
	ffsServiceDesc := ffsDesc.FindService("ffs.rpc.RPCService")
	if ffsServiceDesc == nil {
		return nil, fmt.Errorf("no ffs service description found")
	}
	return ffsServiceDesc, nil
}
