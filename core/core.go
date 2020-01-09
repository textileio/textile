package core

import (
	"context"
	"os"
	"path"
	"time"

	auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	fc "github.com/textileio/filecoin/api/client"
	threadsapi "github.com/textileio/go-threads/api"
	threadsclient "github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/dns"
	"github.com/textileio/textile/email"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("core")
)

// clientReqKey provides a concrete type for client request context values.
type clientReqKey string

type Textile struct {
	ds          datastore.Datastore
	collections *c.Collections

	ipfs iface.CoreAPI

	threadservice s.ServiceBoostrapper

	threadsServer        *threadsapi.Server
	threadsClient        *threadsclient.Client
	threadsInternalToken string

	server *api.Server
}

type Config struct {
	RepoPath string

	AddrApi              ma.Multiaddr
	AddrThreadsHost      ma.Multiaddr
	AddrThreadsHostProxy ma.Multiaddr
	AddrThreadsApi       ma.Multiaddr
	AddrThreadsApiProxy  ma.Multiaddr
	AddrIpfsApi          ma.Multiaddr
	AddrGatewayHost      ma.Multiaddr
	AddrGatewayUrl       string
	AddrFilecoinApi      ma.Multiaddr

	DNSDomain string
	DNSZoneID string
	DNSToken  string

	DefaultProjectHash string

	EmailFrom   string
	EmailDomain string
	EmailApiKey string

	SessionSecret        string
	ThreadsInternalToken string

	Debug bool
}

func NewTextile(ctx context.Context, conf Config) (*Textile, error) {
	if err := util.SetLogLevels(map[string]logging.LogLevel{
		"core": logging.LevelDebug,
	}); err != nil {
		return nil, err
	}

	dsPath := path.Join(conf.RepoPath, "textile")
	if err := os.MkdirAll(dsPath, os.ModePerm); err != nil {
		return nil, err
	}
	ds, err := badger.NewDatastore(dsPath, &badger.DefaultOptions)
	if err != nil {
		return nil, err
	}

	ipfs, err := httpapi.NewApi(conf.AddrIpfsApi)
	if err != nil {
		return nil, err
	}

	threadservice, err := s.DefaultService(
		conf.RepoPath,
		s.WithServiceHostAddr(conf.AddrThreadsHost),
		s.WithServiceHostProxyAddr(conf.AddrThreadsHostProxy),
		s.WithServiceDebug(conf.Debug))
	if err != nil {
		return nil, err
	}

	t := &Textile{
		threadsInternalToken: conf.ThreadsInternalToken,
	}
	threadsServer, err := threadsapi.NewServer(ctx, threadservice, threadsapi.Config{
		RepoPath:  conf.RepoPath,
		Addr:      conf.AddrThreadsApi,
		ProxyAddr: conf.AddrThreadsApiProxy,
		Debug:     conf.Debug,
	}, grpc.UnaryInterceptor(auth.UnaryServerInterceptor(t.clientAuthFunc)),
		grpc.StreamInterceptor(auth.StreamServerInterceptor(t.clientAuthFunc)))
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

	collections, err := c.NewCollections(ctx, threadsClient, conf.ThreadsInternalToken, ds)
	if err != nil {
		return nil, err
	}

	var fcClient *fc.Client
	if conf.AddrFilecoinApi != nil {
		fcClient, err = fc.NewClient(conf.AddrFilecoinApi)
		if err != nil {
			return nil, err
		}
	}

	dnsManager, err := dns.NewManager(conf.DNSDomain, conf.DNSZoneID, conf.DNSToken, conf.Debug)
	if err != nil {
		return nil, err
	}

	collections, err := c.NewCollections(ctx, threadsClient, ds, dnsManager, conf.DefaultProjectHash)
	if err != nil {
		return nil, err
	}

	emailClient, err := email.NewClient(
		conf.EmailFrom, conf.EmailDomain, conf.EmailApiKey, conf.Debug)
	if err != nil {
		return nil, err
	}

	server, err := api.NewServer(ctx, api.Config{
		Addr:            conf.AddrApi,
		AddrGatewayHost: conf.AddrGatewayHost,
		AddrGatewayUrl:  conf.AddrGatewayUrl,
		Collections:     collections,
		DNSManager:      dnsManager,
		EmailClient:     emailClient,
		FCClient:        fcClient,
		SessionSecret:   conf.SessionSecret,
		Debug:           conf.Debug,
	})
	if err != nil {
		return nil, err
	}

	log.Info("started")

	t.ds = ds
	t.collections = collections
	t.ipfs = ipfs
	t.threadservice = threadservice
	t.threadsServer = threadsServer
	t.threadsClient = threadsClient
	t.server = server

	return t, nil
}

func (t *Textile) Bootstrap() {
	t.threadservice.Bootstrap(util.DefaultBoostrapPeers())
}

func (t *Textile) Close() error {
	if err := t.threadsClient.Close(); err != nil {
		return err
	}
	if err := t.threadservice.Close(); err != nil {
		return err
	}
	t.threadsServer.Close()
	if err := t.server.Close(); err != nil {
		return err
	}
	return t.ds.Close()
}

func (t *Textile) HostID() peer.ID {
	return t.threadservice.Host().ID()
}

func (t *Textile) clientAuthFunc(ctx context.Context) (context.Context, error) {
	token, err := auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		return nil, err
	}
	if token == t.threadsInternalToken {
		return ctx, nil
	}

	session, err := t.collections.Sessions.Get(ctx, token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Invalid auth token")
	}
	if session.Expiry < int(time.Now().Unix()) {
		return nil, status.Error(codes.Unauthenticated, "Expired auth token")
	}
	user, err := t.collections.AppUsers.Get(ctx, session.UserID)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "User not found")
	}
	proj, err := t.collections.Projects.Get(ctx, user.ProjectID)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "Project not found")
	}

	if err := t.collections.Sessions.Touch(ctx, session); err != nil {
		return nil, err
	}

	newCtx := context.WithValue(ctx, clientReqKey("user"), user)
	newCtx = context.WithValue(newCtx, clientReqKey("project"), proj)
	return newCtx, nil
}
