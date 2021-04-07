package migrations

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	count, err = db.Collection("users").CountDocuments(ctx, bson.M{})
	require.NoError(t, err)
	assert.Equal(t, 0, int(count))

	// Run down
	err = migrate.NewMigrate(db, m002).Down(migrate.AllAvailable)
	require.NoError(t, err)

	count, err = db.Collection("accounts").CountDocuments(ctx, bson.M{})
	require.NoError(t, err)
	assert.Equal(t, 3, int(count))
	count, err = db.Collection("users").CountDocuments(ctx, bson.M{})
	require.NoError(t, err)
	assert.Equal(t, 3, int(count))
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

// Test remove buckets_total_size from accounts
func TestMigrations_m004(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)

	// Preload collections
	_, err := db.Collection("ipnskeys").InsertMany(ctx, []interface{}{
		bson.M{"_id": "name", "cid": "cid", "created_at": time.Now()},
	})
	require.NoError(t, err)

	// Run up
	err = migrate.NewMigrate(db, m004).Up(migrate.AllAvailable)
	require.NoError(t, err)

	res := db.Collection("ipnskeys").FindOne(ctx, bson.M{})
	require.NoError(t, res.Err())
	var key bson.M
	err = res.Decode(&key)
	require.NoError(t, err)
	assert.NotNil(t, key["path"])

	// Run down
	err = migrate.NewMigrate(db, m004).Down(migrate.AllAvailable)
	require.NoError(t, err)

	res = db.Collection("ipnskeys").FindOne(ctx, bson.M{})
	require.NoError(t, res.Err())
	var key2 bson.M
	err = res.Decode(&key2)
	require.NoError(t, err)
	assert.Nil(t, key2["path"])
}

func TestMigrations_m005(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)
	_, err := db.Collection("bucketarchives").InsertMany(ctx, []interface{}{
		bson.M{
			"_id": "id1",
			"archives": bson.M{
				"current": bson.M{
					"job_id":     "job3",
					"status":     1,
					"created_at": 1000,
				},
				"history": bson.A{
					bson.M{
						"job_id":     "job2",
						"status":     1,
						"created_at": 1000,
					},
					bson.M{
						"job_id":     "job1",
						"status":     1,
						"created_at": 1000,
					},
				},
			},
		},
		bson.M{
			"_id": "id2",
			"archives": bson.M{
				"current": bson.M{
					"job_id":     "job6",
					"job_status": 1,
					"created_at": 1000,
				},
				"history": bson.A{
					bson.M{
						"job_id":     "job5",
						"job_status": 1,
						"created_at": 1000,
					},
					bson.M{
						"job_id":     "job4",
						"job_status": 1,
						"created_at": 1000,
					},
				},
			},
		},
		bson.M{
			"_id": "id3",
			"archives": bson.M{
				"current": bson.M{
					"job_id":     "job9",
					"status":     1,
					"created_at": 1000,
				},
				"history": bson.A{
					bson.M{
						"job_id":     "job8",
						"job_status": 1,
						"created_at": 1000,
					},
					bson.M{
						"job_id":     "job7",
						"status":     1,
						"created_at": 1000,
					},
				},
			},
		},
		bson.M{
			"_id": "id4",
			"archives": bson.M{
				"current": bson.M{
					"job_id":     "job12",
					"job_status": 1,
					"created_at": 1000,
				},
				"history": bson.A{
					bson.M{
						"job_id":     "job11",
						"status":     1,
						"created_at": 1000,
					},
					bson.M{
						"job_id":     "job10",
						"status":     1,
						"created_at": 1000,
					},
				},
			},
		},
		bson.M{
			"_id": "id5",
			"archives": bson.M{
				"history": bson.A{
					bson.M{
						"job_id":     "job14",
						"job_status": 1,
						"created_at": 1000,
					},
					bson.M{
						"job_id":     "job13",
						"status":     1,
						"created_at": 1000,
					},
				},
			},
		},
		bson.M{
			"_id": "id6",
			"archives": bson.M{
				"current": bson.M{
					"job_id":     "job15",
					"job_status": 1,
					"created_at": 1000,
				},
			},
		},
	})
	require.NoError(t, err)

	// Run up
	err = migrate.NewMigrate(db, m005).Up(migrate.AllAvailable)
	require.NoError(t, err)

	count, err := db.Collection("bucketarchives").CountDocuments(ctx, bson.M{})
	require.NoError(t, err)
	assert.Equal(t, 6, int(count))
	cursor, err := db.Collection("bucketarchives").Find(ctx, bson.M{})
	require.NoError(t, err)
	defer cursor.Close(ctx)

	validateItem := func(item bson.M) {
		require.NotEmpty(t, item["job_id"])
		require.Contains(t, item["job_id"], "job")
		require.Equal(t, 1000, int(item["created_at"].(int32)))
		require.Nil(t, item["job_status"])
		require.Equal(t, 1, int(item["status"].(int32)))
	}

	for cursor.Next(ctx) {
		var item bson.M
		require.NoError(t, cursor.Decode(&item))
		require.NotEmpty(t, item["_id"])
		require.NotEmpty(t, item["archives"])
		if item["_id"] == "id1" || item["_id"] == "id2" || item["_id"] == "id3" || item["_id"] == "id4" {
			require.NotNil(t, item["archives"].(bson.M)["current"])
			require.NotNil(t, item["archives"].(bson.M)["history"])
		}
		if item["_id"] == "id5" {
			require.Nil(t, item["archives"].(bson.M)["current"])
			require.NotNil(t, item["archives"].(bson.M)["history"])
		}
		if item["_id"] == "id6" {
			require.NotNil(t, item["archives"].(bson.M)["current"])
			require.Nil(t, item["archives"].(bson.M)["history"])
		}
		if item["archives"].(bson.M)["current"] != nil {
			validateItem(item["archives"].(bson.M)["current"].(bson.M))
		}
		if item["archives"].(bson.M)["history"] != nil {
			for _, item := range item["archives"].(bson.M)["history"].(bson.A) {
				validateItem(item.(bson.M))
			}
		}
	}
	require.NoError(t, cursor.Err())
}

func setup(t *testing.T, ctx context.Context) *mongo.Database {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(test.GetMongoUri()))
	require.NoError(t, err)
	db := client.Database("test_textile_migrations")
	t.Cleanup(func() {
		err := db.Drop(ctx)
		require.NoError(t, err)
	})
	return db
}
