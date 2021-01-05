package migrations

import (
	"context"
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
	Description: "make customer_id index sparse",
	Up: func(db *mongo.Database) error {
		log.Info("migrating 001 up")
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		count, err := db.Collection("customers").CountDocuments(ctx, bson.M{})
		if err != nil {
			return err
		}
		if count == 0 {
			return nil // namespace doesn't exist
		}
		_, err = db.Collection("customers").Indexes().DropOne(ctx, "customer_id_1")
		if err != nil {
			return err
		}
		_, err = db.Collection("customers").Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{"customer_id", 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		})
		return err
	},
	Down: func(db *mongo.Database) error {
		log.Info("migrating 001 down")
		ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
		defer cancel()
		count, err := db.Collection("customers").CountDocuments(ctx, bson.M{})
		if err != nil {
			return err
		}
		if count == 0 {
			return nil // namespace doesn't exist
		}
		_, err = db.Collection("customers").Indexes().DropOne(ctx, "customer_id_1")
		if err != nil {
			return err
		}
		_, err = db.Collection("customers").Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{"customer_id", 1}},
			Options: options.Index().SetUnique(true),
		})
		return err
	},
}

func Migrate(db *mongo.Database) error {
	m := migrate.NewMigrate(
		db,
		m001,
	)
	return m.Up(migrate.AllAvailable)
}
