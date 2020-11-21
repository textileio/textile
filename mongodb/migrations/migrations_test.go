package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	migrate "github.com/xakep666/mongo-migrate"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Test make accounts username index sparse
func TestMigrations_m001(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)

	// Preload collection
	_, err := db.Collection("accounts").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{"username", 1}},
		Options: options.Index().
			SetUnique(true).
			SetCollation(&options.Collation{Locale: "en", Strength: 2}),
	})
	require.NoError(t, err)
	_, err = db.Collection("accounts").InsertMany(ctx, []interface{}{
		bson.M{"username": "one"},
		bson.M{"username": "two"},
		bson.M{"username": "three"},
	})
	require.NoError(t, err)

	// Test that nil username causes duplicate key error
	users := []interface{}{
		bson.M{"foo": 1}, // nil username
		bson.M{"bar": 1}, // nil username
	}
	_, err = db.Collection("accounts").InsertMany(ctx, users)
	require.Error(t, err) // Duplicate key error

	// Run up
	err = migrate.NewMigrate(db, m001).Up(migrate.AllAvailable)
	require.NoError(t, err)

	// No duplicate key error this time
	_, err = db.Collection("accounts").InsertMany(ctx, users)
	require.NoError(t, err)

	// Clean up
	_, err = db.Collection("accounts").DeleteMany(ctx, bson.M{})
	require.NoError(t, err)

	// Run down
	err = migrate.NewMigrate(db, m001).Down(migrate.AllAvailable)
	require.NoError(t, err)
}

// Test consolidate users and accounts
func TestMigrations_m002(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)

	// Preload collections
	_, err := db.Collection("accounts").InsertMany(ctx, []interface{}{
		bson.M{"type": 0, "created_at": time.Now()},
		bson.M{"type": 0, "created_at": time.Now()},
		bson.M{"type": 0, "created_at": time.Now()},
	})
	require.NoError(t, err)
	_, err = db.Collection("users").InsertMany(ctx, []interface{}{
		bson.M{"created_at": time.Now()},
		bson.M{"created_at": time.Now()},
		bson.M{"created_at": time.Now()},
	})
	require.NoError(t, err)

	// Run up
	err = migrate.NewMigrate(db, m002).Up(migrate.AllAvailable)
	require.NoError(t, err)

	count, err := db.Collection("accounts").CountDocuments(ctx, bson.M{})
	require.NoError(t, err)
	assert.Equal(t, 6, int(count))
	count, err = db.Collection("accounts").CountDocuments(ctx, bson.M{"type": 2})
	require.NoError(t, err)
	assert.Equal(t, 3, int(count))
	//count, err = db.Collection("users").CountDocuments(ctx, bson.M{})
	//require.NoError(t, err)
	//assert.Equal(t, 0, int(count))

	// Run down
	// err = migrate.NewMigrate(db, m002).Down(migrate.AllAvailable)
	// require.NoError(t, err)

	//count, err = db.Collection("accounts").CountDocuments(ctx, bson.M{})
	//require.NoError(t, err)
	//assert.Equal(t, 3, int(count))
	//count, err = db.Collection("users").CountDocuments(ctx, bson.M{})
	//require.NoError(t, err)
	//assert.Equal(t, 3, int(count))
}

// Test remove buckets_total_size from accounts
func TestMigrations_m003(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)

	// Preload collections
	_, err := db.Collection("accounts").InsertMany(ctx, []interface{}{
		bson.M{"buckets_total_size": 1024, "created_at": time.Now()},
	})
	require.NoError(t, err)

	// Run up
	err = migrate.NewMigrate(db, m003).Up(migrate.AllAvailable)
	require.NoError(t, err)

	res := db.Collection("accounts").FindOne(ctx, bson.M{})
	require.NoError(t, res.Err())
	var account bson.M
	err = res.Decode(&account)
	require.NoError(t, err)
	assert.Nil(t, account["buckets_total_size"])

	// Run down
	err = migrate.NewMigrate(db, m003).Down(migrate.AllAvailable)
	require.NoError(t, err)

	res = db.Collection("accounts").FindOne(ctx, bson.M{})
	require.NoError(t, res.Err())
	var account2 bson.M
	err = res.Decode(&account2)
	require.NoError(t, err)
	assert.NotNil(t, account2["buckets_total_size"])
}

func setup(t *testing.T, ctx context.Context) *mongo.Database {
	uri := "mongodb://127.0.0.1:27017"
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	require.NoError(t, err)
	db := client.Database("test_migrations")
	t.Cleanup(func() {
		err := db.Drop(ctx)
		require.NoError(t, err)
	})
	return db
}
