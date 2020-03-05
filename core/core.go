package core

import (
	"context"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	fc "github.com/textileio/filecoin/api/client"
	threadsapi "github.com/textileio/go-threads/api"
	threadsclient "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/broadcast"
	serviceapi "github.com/textileio/go-threads/service/api"
	s "github.com/textileio/go-threads/store"
	"github.com/textileio/go-threads/util"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/dns"
	"github.com/textileio/textile/email"
	"github.com/textileio/textile/gateway"
	"google.golang.org/grpc"
)

var (
	log = logging.Logger("core")
)

// clientReqKey provides a concrete type for client request context values.
//type clientReqKey string

type Textile struct {
	ds          datastore.Datastore
	collections *c.Collections

	threadservice        s.ServiceBoostrapper
	threadsServiceServer *serviceapi.Server
	threadsServer        *threadsapi.Server
	threadsClient        *threadsclient.Client
	threadsToken         string

	server  *cloud.Server
	gateway *gateway.Gateway

	sessionBus *broadcast.Broadcaster
}

type Config struct {
	RepoPath string

	AddrApi                    ma.Multiaddr
	AddrApiProxy               ma.Multiaddr
	AddrThreadsHost            ma.Multiaddr
	AddrThreadsServiceApi      ma.Multiaddr
	AddrThreadsServiceApiProxy ma.Multiaddr
	AddrThreadsApi             ma.Multiaddr
	AddrThreadsApiProxy        ma.Multiaddr
	AddrIpfsApi                ma.Multiaddr
	AddrGatewayHost            ma.Multiaddr
	AddrGatewayUrl             string
	AddrGatewayBucketDomain    string
	AddrFilecoinApi            ma.Multiaddr

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
			"core":        logging.LevelDebug,
			"collections": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	dsPath := path.Join(conf.RepoPath, "textile")
	if err := os.MkdirAll(dsPath, os.ModePerm); err != nil {
		return nil, err
	}
	ds, err := badger.NewDatastore(dsPath, &badger.DefaultOptions)
	if err != nil {
		return nil, err
	}

	t := &Textile{
		threadsToken: uuid.New().String(),
		sessionBus:   broadcast.NewBroadcaster(0),
	}
	threadservice, err := s.DefaultService(
		conf.RepoPath,
		s.WithServiceHostAddr(conf.AddrThreadsHost),
		s.WithServiceDebug(conf.Debug))
	if err != nil {
		return nil, err
	}
	serviceServer, err := serviceapi.NewServer(ctx, threadservice, serviceapi.Config{
		Addr:      conf.AddrThreadsServiceApi,
		ProxyAddr: conf.AddrThreadsServiceApiProxy,
		Debug:     conf.Debug,
	})
	//}, grpc.UnaryInterceptor(auth.UnaryServerInterceptor(t.clientAuthFunc)),
	//	grpc.StreamInterceptor(auth.StreamServerInterceptor(t.clientAuthFunc)))
	if err != nil {
		return nil, err
	}
	threadsServer, err := threadsapi.NewServer(ctx, threadservice, threadsapi.Config{
		RepoPath:  conf.RepoPath,
		Addr:      conf.AddrThreadsApi,
		ProxyAddr: conf.AddrThreadsApiProxy,
		Debug:     conf.Debug,
	})
	//}, grpc.UnaryInterceptor(auth.UnaryServerInterceptor(t.clientAuthFunc)),
	//	grpc.StreamInterceptor(auth.StreamServerInterceptor(t.clientAuthFunc)))
	if err != nil {
		return nil, err
	}

	threadsTarget, err := util.TCPAddrFromMultiAddr(conf.AddrThreadsApi)
	if err != nil {
		return nil, err
	}
	threadsClient, err := threadsclient.NewClient(threadsTarget, grpc.WithInsecure(),
		grpc.WithPerRPCCredentials(c.TokenAuth{}))
	if err != nil {
		return nil, err
	}

	collections, err := c.NewCollections(ctx, "mongodb://127.0.0.1:27017")
	if err != nil {
		return nil, err
	}

	ipfsClient, err := httpapi.NewApi(conf.AddrIpfsApi)
	if err != nil {
		return nil, err
	}

	var filecoinClient *fc.Client
	if conf.AddrFilecoinApi != nil {
		filecoinClient, err = fc.NewClient(conf.AddrFilecoinApi)
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

	gatewayToken := uuid.New().String()

	server, err := cloud.NewServer(ctx, cloud.Config{
		Addr:           conf.AddrApi,
		AddrProxy:      conf.AddrApiProxy,
		Collections:    collections,
		DNSManager:     dnsManager,
		EmailClient:    emailClient,
		IPFSClient:     ipfsClient,
		FilecoinClient: filecoinClient,
		GatewayUrl:     conf.AddrGatewayUrl,
		GatewayToken:   gatewayToken,
		SessionBus:     t.sessionBus,
		SessionSecret:  conf.SessionSecret,
		Debug:          conf.Debug,
	})
	if err != nil {
		return nil, err
	}

	t.ds = ds
	t.collections = collections
	t.threadservice = threadservice
	t.threadsServiceServer = serviceServer
	t.threadsServer = threadsServer
	t.threadsClient = threadsClient
	t.server = server

	t.gateway, err = gateway.NewGateway(
		conf.AddrGatewayHost,
		conf.AddrApi,
		gatewayToken,
		conf.AddrGatewayBucketDomain,
		collections,
		t.sessionBus,
		conf.Debug)
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
	if err := t.threadsClient.Close(); err != nil {
		return err
	}
	if err := t.threadservice.Close(); err != nil {
		return err
	}
	t.threadsServiceServer.Close()
	t.threadsServer.Close()
	if err := t.server.Close(); err != nil {
		return err
	}
	if err := t.collections.Close(); err != nil {
		return err
	}
	return t.ds.Close()
}

func (t *Textile) HostID() peer.ID {
	return t.threadservice.Host().ID()
}

//func (t *Textile) clientAuthFunc(ctx context.Context) (context.Context, error) {
//	token, err := auth.AuthFromMD(ctx, "bearer")
//	if err != nil {
//		return nil, err
//	}
//	if token == t.threadsToken {
//		return ctx, nil
//	}
//
//	session, err := t.collections.Sessions.Get(ctx, token)
//	if err != nil {
//		return nil, status.Error(codes.Unauthenticated, "Invalid auth token")
//	}
//	if session.Expiry < int(time.Now().Unix()) {
//		return nil, status.Error(codes.Unauthenticated, "Expired auth token")
//	}
//	user, err := t.collections.Users.Get(ctx, session.UserID)
//	if err != nil {
//		return nil, status.Error(codes.PermissionDenied, "User not found")
//	}
//	proj, err := t.collections.Projects.Get(ctx, user.ProjectID)
//	if err != nil {
//		return nil, status.Error(codes.PermissionDenied, "Project not found")
//	}
//
//	if err := t.collections.Sessions.Touch(ctx, session); err != nil {
//		return nil, err
//	}
//
//	newCtx := context.WithValue(ctx, clientReqKey("user"), user)
//	newCtx = context.WithValue(newCtx, clientReqKey("project"), proj)
//	return newCtx, nil
//}
