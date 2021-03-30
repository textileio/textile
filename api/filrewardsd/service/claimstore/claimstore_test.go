package claimstore

import (
	"context"
	"os"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-ds-mongo/test"
	"github.com/textileio/textile/v2/api/filrewardsd/service/interfaces"
	"github.com/textileio/textile/v2/util"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ctx = context.Background()
)

func TestMain(m *testing.M) {
	logging.SetAllLoggers(logging.LevelError)

	cleanup := test.StartMongoDB()
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

func TestNew(t *testing.T) {
	cs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, cs, "org", "me")
}

func TestList(t *testing.T) {
	cs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	res, err := cs.List(ctx, interfaces.ListConfig{})
	require.NoError(t, err)
	require.Len(t, res, 3)
}

func TestListOrgKey(t *testing.T) {
	cs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, cs, "org1", "me")
	requireNew(t, cs, "org1", "me")
	requireNew(t, cs, "org2", "me")
	res, err := cs.List(ctx, interfaces.ListConfig{OrgKeyFilter: "org1"})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListClaimedBy(t *testing.T) {
	cs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "you")
	res, err := cs.List(ctx, interfaces.ListConfig{ClaimedByFilter: "me"})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListDescending(t *testing.T) {
	cs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	res, err := cs.List(ctx, interfaces.ListConfig{Ascending: false})
	require.NoError(t, err)
	require.Len(t, res, 3)
	requireClaimsOrder(t, res, false)
}

func TestListAscending(t *testing.T) {
	cs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	res, err := cs.List(ctx, interfaces.ListConfig{Ascending: true})
	require.NoError(t, err)
	require.Len(t, res, 3)
	requireClaimsOrder(t, res, true)
}

func TestListPaging(t *testing.T) {
	cs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")
	requireNew(t, cs, "org", "me")

	pageResults := func(ascending bool) {
		page := int64(0)
		for {
			conf := interfaces.ListConfig{
				PageSize:  3,
				Page:      page,
				Ascending: ascending,
			}
			res, err := cs.List(ctx, conf)
			require.NoError(t, err)
			if len(res) == 0 {
				break
			}
			requireClaimsOrder(t, res, ascending)
			if page < 2 {
				require.Len(t, res, 3)
			}
			if page == 2 {
				require.Len(t, res, 2)
			}
			page++
		}
		require.Equal(t, int64(3), page)
	}
	pageResults(false)
	pageResults(true)
}

func requireNew(t *testing.T, cs *ClaimStore, orgKey, claimedBy string) *interfaces.Claim {
	claim, err := cs.New(ctx, orgKey, claimedBy, 100, "msgcid")
	require.NoError(t, err)
	require.NotEmpty(t, claim.ID)
	require.Equal(t, orgKey, claim.OrgKey)
	require.Equal(t, claimedBy, claim.ClaimedBy)
	require.Equal(t, int64(100), claim.AmountNanoFil)
	require.Equal(t, "msgcid", claim.TxnCid)
	return claim
}

func requireClaimsOrder(t *testing.T, claims []*interfaces.Claim, ascending bool) {
	var last *time.Time
	for _, txn := range claims {
		if last != nil {
			a := *last
			b := txn.CreatedAt
			if ascending {
				a = txn.CreatedAt
				b = *last
			}
			require.True(t, a.After(b))
		}
		t := txn.CreatedAt
		last = &t
	}
}

func requireSetup(t *testing.T) (*ClaimStore, func()) {
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(test.GetMongoUri()))
	require.NoError(t, err)
	db := mongoClient.Database(util.MakeToken(12))
	cs, err := New(db, true)
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, mongoClient.Disconnect(ctx))
	}

	return cs, cleanup
}
