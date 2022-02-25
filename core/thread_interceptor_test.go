package core_test

import (
	"context"
	"crypto/rand"
	"os"
	"testing"

	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
	"github.com/textileio/crypto/symmetric"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/cbor"
	coredb "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	nc "github.com/textileio/go-threads/net/api/client"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/apitest"
	"github.com/textileio/textile/v2/api/common"
	hc "github.com/textileio/textile/v2/api/hubd/client"
	"github.com/textileio/textile/v2/core"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	cleanup := func() {}
	if os.Getenv("SKIP_SERVICES") != "true" {
		cleanup = apitest.StartServices()
	}
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

func Test_ThreadsDB(t *testing.T) {
	t.Parallel()
	conf, hub, threads, _ := setup(t, nil)
	ctx := context.Background()

	dev1 := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	dev2 := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx1 := common.NewSessionContext(ctx, dev1.Session)
	ctx2 := common.NewSessionContext(ctx, dev2.Session)

	// dev1 creates a thread
	id := thread.NewIDV1(thread.Raw, 32)
	err := threads.NewDB(ctx1, id, db.WithNewManagedCollections(collectionCongig))
	require.NoError(t, err)

	// dev1 creates an instance
	_, err = threads.Create(ctx1, id, "Dogs", tc.Instances{&Dog{Name: "Fido", Comments: []Comment{}}})
	require.NoError(t, err)

	// dev2 create an instance (should be fine, this can be controlled with write validator)
	_, err = threads.Create(ctx2, id, "Dogs", tc.Instances{&Dog{Name: "Fido", Comments: []Comment{}}})
	require.NoError(t, err)

	// dev2 attempts all the blocked methods
	_, err = threads.GetDBInfo(ctx2, id)
	require.Error(t, err)
	err = threads.DeleteDB(ctx2, id)
	require.Error(t, err)
	_, err = threads.ListDBs(ctx2)
	require.Error(t, err)
	err = threads.NewCollection(ctx2, id, collectionCongig)
	require.Error(t, err)
	err = threads.UpdateCollection(ctx2, id, collectionCongig)
	require.Error(t, err)
	err = threads.DeleteCollection(ctx2, id, "Dogs")
	require.Error(t, err)
	_, err = threads.GetCollectionInfo(ctx2, id, "Dogs")
	require.Error(t, err)
	_, err = threads.GetCollectionIndexes(ctx2, id, "Dogs")
	require.Error(t, err)
	_, err = threads.ListCollections(ctx2, id)
	require.Error(t, err)
}

func TestClient_ThreadsNet(t *testing.T) {
	t.Parallel()
	conf, hub, _, net := setup(t, nil)
	ctx := context.Background()

	dev1 := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	dev2 := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx1 := common.NewSessionContext(ctx, dev1.Session)
	ctx2 := common.NewSessionContext(ctx, dev2.Session)

	// dev1 creates a thread
	id := thread.NewIDV1(thread.Raw, 32)
	_, err := net.CreateThread(ctx1, id)
	require.NoError(t, err)

	// dev1 creates a record
	body, err := cbornode.WrapObject(map[string]interface{}{
		"foo": "bar",
		"baz": []byte("howdy"),
	}, mh.SHA2_256, -1)
	require.NoError(t, err)
	_, err = net.CreateRecord(ctx1, id, body)
	require.NoError(t, err)

	// dev2 attempts all the blocked methods
	_, err = net.GetThread(ctx2, id)
	require.Error(t, err)
	err = net.PullThread(ctx2, id)
	require.Error(t, err)
	err = net.DeleteThread(ctx2, id)
	require.Error(t, err)

	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(sk)
	require.NoError(t, err)
	addr, err := ma.NewMultiaddr("/p2p/" + pid.String())
	require.NoError(t, err)
	_, err = net.AddReplicator(ctx2, id, addr)
	require.Error(t, err)

	_, err = net.CreateRecord(ctx2, id, body)
	require.Error(t, err)

	event, err := cbor.CreateEvent(context.Background(), nil, body, symmetric.New())
	if err != nil {
		t.Fatal(err)
	}
	rec, err := cbor.CreateRecord(context.Background(), nil, cbor.CreateRecordConfig{
		Block:      event,
		Prev:       cid.Undef,
		Key:        sk,
		PubKey:     thread.NewLibp2pIdentity(sk).GetPublic(),
		ServiceKey: thread.NewRandomServiceKey().Service(),
	})
	require.NoError(t, err)
	err = net.AddRecord(ctx2, id, pid, rec)
	require.Error(t, err)

	_, err = net.GetRecord(ctx2, id, cid.Cid{})
	require.Error(t, err)
}

func setup(t *testing.T, conf *core.Config) (core.Config, *hc.Client, *tc.Client, *nc.Client) {
	if conf == nil {
		tmp := apitest.DefaultTextileConfig(t)
		conf = &tmp
	}
	return setupWithConf(t, *conf)
}

func setupWithConf(t *testing.T, conf core.Config) (core.Config, *hc.Client, *tc.Client, *nc.Client) {
	apitest.MakeTextileWithConfig(t, conf)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	hubclient, err := hc.NewClient(target, opts...)
	require.NoError(t, err)
	threadsclient, err := tc.NewClient(target, opts...)
	require.NoError(t, err)
	threadsnetclient, err := nc.NewClient(target, opts...)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, err)
		err = threadsclient.Close()
		require.NoError(t, err)
	})
	return conf, hubclient, threadsclient, threadsnetclient
}

type Comment struct {
	Body string
}

type Dog struct {
	ID       coredb.InstanceID `json:"_id"`
	Name     string
	Comments []Comment
}

var collectionCongig = db.CollectionConfig{
	Name:   "Dogs",
	Schema: tutil.SchemaFromInstance(&Dog{}, false),
	WriteValidator: `
		var type = event.patch.type
		var patch = event.patch.json_patch
		switch (type) {
		  case "delete":
		    return false
		  default:
		    if (patch.Name !== "Fido" && patch.Name != "Clyde") {
		      return false
		    }
		    return true
		}
	`,
	ReadFilter: `
		instance.Name = "Clyde"
		return instance
	`,
}
