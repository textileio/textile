package migrations

import (
	"context"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	migrate "github.com/xakep666/mongo-migrate"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	log            = logging.Logger("migrations")
	migrateTimeout = time.Hour
)

var m001 = migrate.Migration{
	Version:     1,
	Description: "make accounts username index sparse",
	Up: func(db *mongo.Database) error {
		log.Info("migrating 001 up")
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		count, err := db.Collection("accounts").CountDocuments(ctx, bson.M{})
		if err != nil {
			return err
		}
		if count == 0 {
			return nil // namespace doesn't exist
		}
		_, err = db.Collection("accounts").Indexes().DropOne(ctx, "username_1")
		if err != nil {
			return err
		}
		_, err = db.Collection("accounts").Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{{"username", 1}},
			Options: options.Index().
				SetUnique(true).
				SetCollation(&options.Collation{Locale: "en", Strength: 2}).
				SetSparse(true),
		})
		return err
	},
	Down: func(db *mongo.Database) error {
		log.Info("migrating 001 down")
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		count, err := db.Collection("accounts").CountDocuments(ctx, bson.M{})
		if err != nil {
			return err
		}
		if count == 0 {
			return nil // namespace doesn't exist
		}
		_, err = db.Collection("accounts").Indexes().DropOne(ctx, "username_1")
		if err != nil {
			return err
		}
		_, err = db.Collection("accounts").Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{{"username", 1}},
			Options: options.Index().
				SetUnique(true).
				SetCollation(&options.Collation{Locale: "en", Strength: 2}),
		})
		return err
	},
}

var m002 = migrate.Migration{
	Version:     2,
	Description: "consolidate users and accounts",
	Up: func(db *mongo.Database) error {
		log.Info("migrating 002 up")
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		cursor, err := db.Collection("users").Find(ctx, bson.M{})
		if err != nil {
			return err
		}
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var user bson.M
			if err := cursor.Decode(&user); err != nil {
				return err
			}
			user["type"] = 2
			_, err := db.Collection("accounts").InsertOne(ctx, user)
			if err != nil {
				if !strings.Contains(err.Error(), "E11000 duplicate key error") {
					return err
				}
			}
		}
		if cursor.Err() != nil {
			return cursor.Err()
		}
		return db.Collection("users").Drop(ctx)
	},
	Down: func(db *mongo.Database) error {
		log.Info("migrating 002 down")
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		cursor, err := db.Collection("accounts").Find(ctx, bson.M{})
		if err != nil {
			return err
		}
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var account bson.M
			if err := cursor.Decode(&account); err != nil {
				return err
			}
			if v, ok := account["type"]; ok && v.(int32) == 2 {
				delete(account, "type")
				_, err := db.Collection("users").InsertOne(ctx, account)
				if err != nil {
					return err
				}
				_, err = db.Collection("accounts").DeleteOne(ctx, bson.M{"_id": account["_id"]})
				if err != nil {
					return err
				}
			}
		}
		return cursor.Err()
	},
}

var m003 = migrate.Migration{
	Version:     3,
	Description: "remove buckets_total_size from accounts",
	Up: func(db *mongo.Database) error {
		log.Info("migrating 003 up")
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		_, err := db.Collection("accounts").UpdateMany(ctx, bson.M{
			"buckets_total_size": bson.M{"$exists": 1},
		}, bson.M{
			"$unset": bson.M{"buckets_total_size": 1},
		})
		return err
	},
	Down: func(db *mongo.Database) error {
		log.Info("migrating 003 down")
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		_, err := db.Collection("accounts").UpdateMany(ctx, bson.M{}, bson.M{
			"$set": bson.M{"buckets_total_size": 0},
		})
		return err
	},
}

var m004 = migrate.Migration{
	Version:     4,
	Description: "set empty path field on all ipnskeys",
	Up: func(db *mongo.Database) error {
		log.Info("migrating 004 up")
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		_, err := db.Collection("ipnskeys").UpdateMany(ctx, bson.M{}, bson.M{
			"$set": bson.M{"path": ""},
		})
		return err
	},
	Down: func(db *mongo.Database) error {
		log.Info("migrating 004 down")
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		_, err := db.Collection("ipnskeys").UpdateMany(ctx, bson.M{
			"path": bson.M{"$exists": 1},
		}, bson.M{
			"$unset": bson.M{"path": 1},
		})
		return err

	},
}

func Migrate(db *mongo.Database) error {
	m := migrate.NewMigrate(
		db,
		m001,
		m002,
		m003,
		m004,
	)
	return m.Up(migrate.AllAvailable)
}
