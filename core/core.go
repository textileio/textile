package core

import (
	"context"
	"errors"
	"path"
	"strconv"

	"github.com/libp2p/go-libp2p-core/peer"

	badger "github.com/ipfs/go-ds-badger"
	"github.com/textileio/textile/resources/users"

	"github.com/ipfs/go-datastore"

	httpapi "github.com/ipfs/go-ipfs-http-client"
	iface "github.com/ipfs/interface-go-ipfs-core"
	ma "github.com/multiformats/go-multiaddr"
	threadsapi "github.com/textileio/go-textile-threads/api"
	threadsclient "github.com/textileio/go-textile-threads/api/client"
	es "github.com/textileio/go-textile-threads/eventstore"
	"github.com/textileio/go-textile-threads/util"
	"github.com/textileio/textile/api"
)

var (
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

	usersStoreID, err := storeIDAtKey(ds, dsUsersKey.ChildString("store"))
	if err != nil {
		return nil, err
	}
	usersResourse, err := users.NewUsers(usersStoreID, threadsClient)
	if err != nil {
		return nil, err
	}

	server, err := api.NewServer(context.Background(), api.Config{
		Addr:  conf.AddrApi,
		Users: usersResourse,
		Debug: conf.Debug,
	})
	if err != nil {
		return nil, err
	}

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
	return nil
}

func (t *Textile) HostID() peer.ID {
	return t.threadservice.Host().ID()
}

func storeIDAtKey(ds datastore.Datastore, key datastore.Key) (string, error) {
	idv, err := ds.Get(key)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return string(idv), nil
}
