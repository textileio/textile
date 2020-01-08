package core

import (
	"context"
	"fmt"
	"os"
	"path"

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
)

var (
	log = logging.Logger("core")
)

type Textile struct {
	ds datastore.Datastore

	ipfs iface.CoreAPI

	threadservice s.ServiceBoostrapper

	threadsServer *threadsapi.Server
	threadsClient *threadsclient.Client

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

	EmailFrom   string
	EmailDomain string
	EmailApiKey string

	SessionSecret []byte

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

	threadsServer, err := threadsapi.NewServer(ctx, threadservice, threadsapi.Config{
		RepoPath:  conf.RepoPath,
		Addr:      conf.AddrThreadsApi,
		ProxyAddr: conf.AddrThreadsApiProxy,
		Debug:     conf.Debug,
	})
	if err != nil {
		return nil, err
	}

	threadsClient, err := threadsclient.NewClient(conf.AddrThreadsApi)
	if err != nil {
		return nil, err
	}

	collections, err := c.NewCollections(ctx, threadsClient, ds)
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

	emailClient, err := email.NewClient(
		conf.EmailFrom, conf.EmailDomain, conf.EmailApiKey, conf.Debug)
	if err != nil {
		return nil, err
	}

	domain := ""
	email := ""
	SECRET := ""
	zoneID := ""
	dnsManager, err := dns.NewClient(domain, email, zoneID, SECRET, conf.Debug)
	if err != nil {
		return nil, err
	}
	fmt.Println(dnsManager)
	fmt.Printf("SUCCESS")

	server, err := api.NewServer(ctx, api.Config{
		Addr:            conf.AddrApi,
		AddrGatewayHost: conf.AddrGatewayHost,
		AddrGatewayUrl:  conf.AddrGatewayUrl,
		Collections:     collections,
		EmailClient:     emailClient,
		FCClient:        fcClient,
		SessionSecret:   conf.SessionSecret,
		Debug:           conf.Debug,
	})
	if err != nil {
		return nil, err
	}

	log.Info("started")

	return &Textile{
		ds: ds,

		ipfs: ipfs,

		threadservice: threadservice,
		threadsServer: threadsServer,
		threadsClient: threadsClient,

		server: server,
	}, nil
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
