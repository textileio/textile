package core

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	grpcm "github.com/grpc-ecosystem/go-grpc-middleware"
	auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log"
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
	"github.com/textileio/textile/api/buckets"
	bpb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/api/hub"
	hpb "github.com/textileio/textile/api/hub/pb"
	"github.com/textileio/textile/api/users"
	upb "github.com/textileio/textile/api/users/pb"
	bucks "github.com/textileio/textile/buckets"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/dns"
	"github.com/textileio/textile/email"
	"github.com/textileio/textile/gateway"
	"github.com/textileio/textile/ipns"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("core")

	ignoreMethods = []string{
		"/hub.pb.API/Signup",
		"/hub.pb.API/Signin",
		"/hub.pb.API/IsUsernameAvailable",
	}
)

type Textile struct {
	collections *c.Collections

	ts    tc.NetBoostrapper
	th    *threads.Client
	thn   *netclient.Client
	bucks *bucks.Buckets
	powc  *powc.Client

	ipnsm *ipns.Manager
	dnsm  *dns.Manager

	server *grpc.Server
	proxy  *http.Server

	gateway         *gateway.Gateway
	gatewaySession  string
	emailSessionBus *broadcast.Broadcaster
}

type Config struct {
	RepoPath string

	AddrAPI          ma.Multiaddr
	AddrAPIProxy     ma.Multiaddr
	AddrThreadsHost  ma.Multiaddr
	AddrIPFSAPI      ma.Multiaddr
	AddrGatewayHost  ma.Multiaddr
	AddrGatewayURL   string
	AddrPowergateAPI ma.Multiaddr
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

	Debug bool
}

func NewTextile(ctx context.Context, conf Config) (*Textile, error) {
	if conf.Debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"core":       logging.LevelDebug,
			"hubapi":     logging.LevelDebug,
			"bucketsapi": logging.LevelDebug,
			"usersapi":   logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}
	t := &Textile{}

	// Configure clients
	ic, err := httpapi.NewApi(conf.AddrIPFSAPI)
	if err != nil {
		return nil, err
	}
	if conf.AddrPowergateAPI != nil {
		t.powc, err = powc.NewClient(conf.AddrPowergateAPI, grpc.WithInsecure(), grpc.WithPerRPCCredentials(powc.TokenAuth{}))
		if err != nil {
			return nil, err
		}
	}
	if conf.DNSToken != "" {
		t.dnsm, err = dns.NewManager(conf.DNSDomain, conf.DNSZoneID, conf.DNSToken, conf.Debug)
		if err != nil {
			return nil, err
		}
	}
	ec, err := email.NewClient(conf.EmailFrom, conf.EmailDomain, conf.EmailAPIKey, conf.Debug)
	if err != nil {
		return nil, err
	}
	t.collections, err = c.NewCollections(ctx, conf.AddrMongoURI, conf.MongoName)
	if err != nil {
		return nil, err
	}
	t.ipnsm, err = ipns.NewManager(t.collections.IPNSKeys, ic.Key(), ic.Name(), conf.Debug)
	if err != nil {
		return nil, err
	}

	// Configure threads
	t.ts, err = tc.DefaultNetwork(conf.RepoPath, tc.WithNetHostAddr(conf.AddrThreadsHost), tc.WithNetDebug(conf.Debug))
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
	t.bucks, err = bucks.New(t.th, t.powc, t.collections.FFSInstances, conf.Debug)
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
	t.emailSessionBus = broadcast.NewBroadcaster(0)
	hs := &hub.Service{
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
	}
	bs := &buckets.Service{
		Collections: t.collections,
		Buckets:     t.bucks,
		GatewayURL:  conf.AddrGatewayURL,
		IPFSClient:  ic,
		IPNSManager: t.ipnsm,
		DNSManager:  t.dnsm,
	}
	us := &users.Service{
		Collections: t.collections,
	}

	// Start serving
	ptarget, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPIProxy)
	if err != nil {
		return nil, err
	}
	t.server = grpc.NewServer(
		grpcm.WithUnaryServerChain(auth.UnaryServerInterceptor(t.authFunc), t.threadInterceptor()),
		grpcm.WithStreamServerChain(auth.StreamServerInterceptor(t.authFunc)))
	listener, err := net.Listen("tcp", target)
	if err != nil {
		return nil, err
	}
	go func() {
		dbpb.RegisterAPIServer(t.server, ts)
		netpb.RegisterAPIServer(t.server, ns)
		hpb.RegisterAPIServer(t.server, hs)
		bpb.RegisterAPIServer(t.server, bs)
		upb.RegisterAPIServer(t.server, us)
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
	t.gatewaySession = util.MakeToken(32)
	t.gateway, err = gateway.NewGateway(gateway.Config{
		Addr:            conf.AddrGatewayHost,
		URL:             conf.AddrGatewayURL,
		Subdomains:      conf.UseSubdomains,
		BucketsDomain:   conf.DNSDomain,
		APIAddr:         conf.AddrAPI,
		APISession:      t.gatewaySession,
		Collections:     t.collections,
		IPFSClient:      ic,
		EmailSessionBus: t.emailSessionBus,
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

func (t *Textile) Close() error {
	t.emailSessionBus.Discard()
	if err := t.gateway.Stop(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := t.proxy.Shutdown(ctx); err != nil {
		return err
	}
	t.server.GracefulStop()
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
		if sid == t.gatewaySession {
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
		ctx = c.NewSessionContext(ctx, session)

		dev, err := t.collections.Accounts.Get(ctx, session.Owner)
		if err != nil {
			return nil, status.Error(codes.NotFound, "User not found")
		}
		ctx = c.NewDevContext(ctx, dev)

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
				ctx = c.NewOrgContext(ctx, org)
				ctx = common.NewOrgSlugContext(ctx, orgSlug)
				ctx = thread.NewTokenContext(ctx, org.Token)
			}
		} else {
			ctx = thread.NewTokenContext(ctx, dev.Token)
		}
	} else if k, ok := common.APIKeyFromMD(ctx); ok {
		ctx = common.NewAPIKeyContext(ctx, k)
		msg, sig, ok := common.APISigFromMD(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "API key signature required")
		}
		ctx = common.NewAPISigContext(ctx, msg, sig)
		key, err := t.collections.APIKeys.Get(ctx, k)
		if err != nil || !key.Valid {
			return nil, status.Error(codes.NotFound, "API key not found or is invalid")
		}
		if !common.ValidateAPISigContext(ctx, key.Secret) {
			return nil, status.Error(codes.Unauthenticated, "Bad API key signature")
		}
		switch key.Type {
		case c.AccountKey:
			acc, err := t.collections.Accounts.Get(ctx, key.Owner)
			if err != nil {
				return nil, status.Error(codes.NotFound, "Account not found")
			}
			switch acc.Type {
			case c.Dev:
				ctx = c.NewDevContext(ctx, acc)
			case c.Org:
				ctx = c.NewOrgContext(ctx, acc)
			}
			ctx = thread.NewTokenContext(ctx, acc.Token)
		case c.UserKey:
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
					user = &c.User{Key: ukey.PubKey}
				}
				ctx = c.NewUserContext(ctx, user)
			} else if method != "/threads.pb.API/GetToken" && method != "/threads.net.pb.API/GetToken" {
				return nil, status.Error(codes.Unauthenticated, "Token required")
			}
		}
		ctx = c.NewAPIKeyContext(ctx, key)
	} else {
		return nil, status.Error(codes.Unauthenticated, "Session or API key required")
	}
	return ctx, nil
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
		if sid, ok := common.SessionFromContext(ctx); ok && sid == t.gatewaySession {
			return handler(ctx, req)
		}

		var owner crypto.PubKey
		if org, ok := c.OrgFromContext(ctx); ok {
			owner = org.Key
		} else if dev, ok := c.DevFromContext(ctx); ok {
			owner = dev.Key
		} else if user, ok := c.UserFromContext(ctx); ok {
			owner = user.Key
		}
		key, _ := c.APIKeyFromContext(ctx)

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
				if key != nil && key.Type == c.UserKey {
					// Extra user check for user API keys.
					if key.Key != th.Key {
						return nil, status.Error(codes.PermissionDenied, "Bad API key")
					}
				}
			}
		}

		// Collect the user if we haven't seen them before.
		user, ok := c.UserFromContext(ctx)
		if ok && user.CreatedAt.IsZero() {
			if err := t.collections.Users.Create(ctx, owner); err != nil {
				return nil, err
			}
		}
		if !ok && owner != nil {
			// Add the dev/org as the user for the user API.
			ctx = c.NewUserContext(ctx, &c.User{Key: owner})
		}

		// Preemptively track the new thread ID for the owner.
		// This needs to happen before the request is handled in case there's a conflict
		// with the owner and thread name.
		if newID.Defined() {
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
