package mongodb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const uri = "mongodb://127.0.0.1:27017"

func newDB(t *testing.T) *mongo.Database {
	ctx, cancel := context.WithCancel(context.Background())
	m, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	require.NoError(t, err)
	db := m.Database(util.MakeToken(12))

	t.Cleanup(func() {
		err := db.Drop(ctx)
		require.NoError(t, err)
		err = m.Disconnect(ctx)
		require.NoError(t, err)
		cancel()
	})
	return db
}
