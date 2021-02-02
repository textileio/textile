package store

import (
	"time"

	"github.com/textileio/textile/v2/api/mindexd/records"
)

type powRetrievalRecord struct {
	ID                 string                     `bson:"_id"`
	PowName            string                     `bson:"pow_name"`
	FirstFetchedAt     time.Time                  `bson:"first_fetched_at"`
	LastUpdatedAt      time.Time                  `bson:"last_updated_at"`
	PowRetrievalRecord records.PowRetrievalRecord `bson:"pow_retrieval_record"`
}

type powStorageDealRecord struct {
	ID                   string                       `bson:"_id"`
	PowName              string                       `bson:"pow_name"`
	FirstFetchedAt       time.Time                    `bson:"first_fetched_at"`
	LastUpdatedAt        time.Time                    `bson:"last_updated_at"`
	PowStorageDealRecord records.PowStorageDealRecord `bson:"pow_storage_deal_record"`
}
