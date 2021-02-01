package migrations

import (
	"time"

	logging "github.com/ipfs/go-log/v2"
	migrate "github.com/xakep666/mongo-migrate"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	log            = logging.Logger("migrations")
	migrateTimeout = time.Hour
)

func Migrate(db *mongo.Database) error {
	m := migrate.NewMigrate(
		db,
	)
	return m.Up(migrate.AllAvailable)
}
