package collections_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const uri = "mongodb://127.0.0.1:27017"

func newDB(t *testing.T) *mongo.Database {
	ctx, cancel := context.WithCancel(context.Background())
	m, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	require.Nil(t, err)
	db := m.Database(uuid.New().String())

	t.Cleanup(func() {
		err := db.Drop(ctx)
		require.Nil(t, err)
		err = m.Disconnect(ctx)
		require.Nil(t, err)
		cancel()
	})
	return db
}

func newDeveloper(t *testing.T, db *mongo.Database) *Developer {
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)
	dev, err := col.GetOrCreate(context.Background(), "jon@doe.com")
	require.Nil(t, err)
	return dev
}
