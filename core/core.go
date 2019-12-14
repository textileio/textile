package core

import (
	"context"
	"os"
	"path"
	"strconv"

	"github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	threadsapi "github.com/textileio/go-textile-threads/api"
	threadsclient "github.com/textileio/go-textile-threads/api/client"
	es "github.com/textileio/go-textile-threads/eventstore"
	"github.com/textileio/go-textile-threads/util"
	"github.com/textileio/textile/api"
	"github.com/textileio/textile/resources"
	u "github.com/textileio/textile/resources/users"
	logger "github.com/whyrusleeping/go-logging"
)

var (
	log = logging.Logger("core")

	dsUsersKey = datastore.NewKey("/users")
	//dsProjectsKey = datastore.NewKey("/projects")
)

type Textile struct {
	ds datastore.Datastore

	ipfs iface.CoreAPI

	threadservice es.ThreadserviceBoostrapper

	threadsServer *threadsapi.Server
	threadsClient *threadsclient.Client

	server *api.Server
}

type Config struct {
	RepoPath             string
	AddrApi              ma.Multiaddr
	AddrThreadsHost      ma.Multiaddr
	AddrThreadsHostProxy ma.Multiaddr
	AddrThreadsApi       ma.Multiaddr
	AddrThreadsApiProxy  ma.Multiaddr
	AddrIpfsApi          ma.Multiaddr
	Debug                bool
}

func NewTextile(conf Config) (*Textile, error) {
	if err := util.SetLogLevels(map[string]logger.Level{
		"core": logger.DEBUG,
	}); err != nil {
		return nil, err
	}

	repoPath := path.Join(conf.RepoPath, "textile")
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return nil, err
	}
	ds, err := badger.NewDatastore(path.Join(conf.RepoPath, "textile"), &badger.DefaultOptions)
	if err != nil {
		return nil, err
	}

	ipfs, err := httpapi.NewApi(conf.AddrIpfsApi)
	if err != nil {
		return nil, err
	}

	threadservice, err := es.DefaultThreadservice(
		conf.RepoPath,
		es.HostAddr(conf.AddrThreadsHost),
		es.HostProxyAddr(conf.AddrThreadsHostProxy),
		es.Debug(conf.Debug))
	if err != nil {
		return nil, err
	}

	threadsServer, err := threadsapi.NewServer(context.Background(), threadservice, threadsapi.Config{
		RepoPath:  conf.RepoPath,
		Addr:      conf.AddrThreadsApi,
		ProxyAddr: conf.AddrThreadsApiProxy,
		Debug:     conf.Debug,
	})
	if err != nil {
		return nil, err
	}

	// @todo: Threads Client should take a multiaddress.
	threadsHost, err := conf.AddrThreadsApi.ValueForProtocol(ma.P_IP4)
	if err != nil {
		return nil, err
	}
	threadsPortStr, err := conf.AddrThreadsApi.ValueForProtocol(ma.P_TCP)
	if err != nil {
		return nil, err
	}
	threadsPort, err := strconv.Atoi(threadsPortStr)
	if err != nil {
		return nil, err
	}

	threadsClient, err := threadsclient.NewClient(threadsHost, threadsPort)
	if err != nil {
		return nil, err
	}

	users := &u.Users{}
	if err := resources.AddResource(threadsClient, ds, dsUsersKey, users); err != nil {
		return nil, err
	}
	log.Debugf("users store: %s", users.GetStoreID().String())

	server, err := api.NewServer(context.Background(), api.Config{
		Addr:  conf.AddrApi,
		Users: users,
		Debug: conf.Debug,
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
	if err := t.threadservice.Close(); err != nil {
		return err
	}
	t.threadsServer.Close()
	t.server.Close()
	return t.ds.Close()
}

func (t *Textile) HostID() peer.ID {
	return t.threadservice.Host().ID()
}
