package migrations

import (
	"context"
	"fmt"
	"time"

	migrate "github.com/xakep666/mongo-migrate"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var migrateTimeout = time.Minute

var m001 = migrate.Migration{
	Version:     1,
	Description: "make accounts username index sparse",
	Up: func(db *mongo.Database) error {
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		_, err := db.Collection("accounts").Indexes().DropOne(ctx, "username_1")
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
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		_, err := db.Collection("accounts").Indexes().DropOne(ctx, "username_1")
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
			fmt.Println(user)
			_, err := db.Collection("accounts").InsertOne(ctx, user)
			if err != nil {
				return err
			}
		}
		return cursor.Err()
	},
	Down: func(db *mongo.Database) error {
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
			fmt.Println(account)
			if v, ok := account["type"]; ok && v == 2 {
				delete(account, "type")
				_, err := db.Collection("users").InsertOne(ctx, account)
				if err != nil {
					return err
				}
			}
			_, err := db.Collection("accounts").DeleteOne(ctx, account)
			if err != nil {
				return err
			}
		}
		return cursor.Err()
	},
}

var m003 = migrate.Migration{
	Version:     3,
	Description: "remove buckets_total_size from accounts",
	Up: func(db *mongo.Database) error {
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
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		_, err := db.Collection("accounts").UpdateMany(ctx, bson.M{}, bson.M{
			"$set": bson.M{"buckets_total_size": 0},
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
	)
	return m.Up(migrate.AllAvailable)
}
