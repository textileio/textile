package migrations

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-ds-mongo/test"
	migrate "github.com/xakep666/mongo-migrate"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMain(m *testing.M) {
	cleanup := func() {}
	if os.Getenv("SKIP_SERVICES") != "true" {
		cleanup = test.StartMongoDB()
	}
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

// Test make customer_id index sparse
func TestMigrations_m001(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)

	// Preload collection
	_, err := db.Collection("customers").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{"customer_id", 1}},
		Options: options.Index().SetUnique(true),
	})
	require.NoError(t, err)
	_, err = db.Collection("customers").InsertMany(ctx, []interface{}{
		bson.M{"customer_id": "one"},
		bson.M{"customer_id": "two"},
		bson.M{"customer_id": "three"},
	})
	require.NoError(t, err)

	// Test that nil customer_id causes duplicate key error
	docs := []interface{}{
		bson.M{"foo": 1}, // nil customer_id
		bson.M{"bar": 1}, // nil customer_id
	}
	_, err = db.Collection("customers").InsertMany(ctx, docs)
	require.Error(t, err) // Duplicate key error

	// Run up
	err = migrate.NewMigrate(db, m001).Up(migrate.AllAvailable)
	require.NoError(t, err)

	// No duplicate key error this time
	_, err = db.Collection("customers").InsertMany(ctx, docs)
	require.NoError(t, err)

	// Clean up
	_, err = db.Collection("customers").DeleteMany(ctx, bson.M{})
	require.NoError(t, err)

	// Run down
	err = migrate.NewMigrate(db, m001).Down(migrate.AllAvailable)
	require.NoError(t, err)
}

func setup(t *testing.T, ctx context.Context) *mongo.Database {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(test.GetMongoUri()))
	require.NoError(t, err)
	db := client.Database("test_billing_migrations")
	t.Cleanup(func() {
		err := db.Drop(ctx)
		require.NoError(t, err)
	})
	return db
}
