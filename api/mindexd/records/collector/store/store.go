package store

import (
	"context"
	"crypto/md5"
	"fmt"
	"time"

	logger "github.com/ipfs/go-log/v2"
	"github.com/textileio/textile/v2/api/mindexd/records"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	log = logger.Logger("record-store")
)

type Store struct {
	sdrc *mongo.Collection
	rrc  *mongo.Collection
}

func New(db *mongo.Database) (*Store, error) {
	s := &Store{
		sdrc: db.Collection("storagedealrecords"),
		rrc:  db.Collection("retrievalrecords"),
	}
	if err := s.ensureIndexes(); err != nil {
		return nil, fmt.Errorf("ensuring mongodb indexes: %s", err)
	}

	return s, nil
}

func (s *Store) GetLastStorageDealRecordUpdatedAt(ctx context.Context, powName string) (int64, error) {
	filter := bson.M{"pow_name": powName}
	opts := options.Find()
	opts = opts.SetSort(bson.D{{Key: "pow_storage_deal_record.updated_at", Value: -1}})
	opts = opts.SetLimit(1)
	opts = opts.SetProjection(bson.M{"pow_storage_deal_record.updated_at": 1})
	c, err := s.sdrc.Find(ctx, filter, opts)
	if err != nil {
		return 0, fmt.Errorf("executing find: %s", err)
	}
	defer func() {
		if err := c.Close(ctx); err != nil {
			log.Errorf("close last storage deal record updatedat cursor: %s", err)
		}
	}()

	res := make([]records.StorageDealRecord, 0, 1)
	if err := c.All(ctx, &res); err != nil {
		return 0, fmt.Errorf("fetching find result: %s", err)
	}
	if len(res) == 0 {
		// If we don't have any records, just return a very old lastUpdatedAt
		// to let it start from the beginning.
		return 0, nil
	}

	return (res)[0].PowStorageDealRecord.UpdatedAt, nil
}

func (s *Store) GetLastRetrievalRecordUpdatedAt(ctx context.Context, powName string) (int64, error) {
	filter := bson.M{"pow_name": powName}
	opts := options.Find()
	opts = opts.SetSort(bson.D{{Key: "pow_retrieval_record.updated_at", Value: -1}})
	opts = opts.SetLimit(1)
	opts = opts.SetProjection(bson.M{"pow_retrieval_record.updated_at": 1})
	c, err := s.rrc.Find(ctx, filter, opts)
	if err != nil {
		return 0, fmt.Errorf("executing find: %s", err)
	}
	defer func() {
		if err := c.Close(ctx); err != nil {
			log.Errorf("close last retrieval record updatedat cursor: %s", err)
		}
	}()

	res := make([]records.RetrievalRecord, 0, 1)
	if err := c.All(ctx, &res); err != nil {
		return 0, fmt.Errorf("fetching find result: %s", err)
	}
	if len(res) == 0 {
		// If we don't have any records, just return a very old lastUpdatedAt
		// to let it start from the beginning.
		return 0, nil
	}

	return (res)[0].PowRetrievalRecord.UpdatedAt, nil
}

func (s *Store) PersistStorageDealRecords(ctx context.Context, powName string, psrs []records.PowStorageDealRecord) error {
	now := time.Now()

	wms := make([]mongo.WriteModel, len(psrs))
	for i, psr := range psrs {
		sr := records.StorageDealRecord{
			LastUpdatedAt:        now,
			PowName:              powName,
			PowStorageDealRecord: psr,
		}
		uwm := mongo.NewUpdateOneModel()
		uwm = uwm.SetFilter(bson.D{{Key: "_id", Value: psr.DealInfo.ProposalCid}})
		uwm = uwm.SetUpdate(bson.M{"$setOnInsert": bson.M{"_id": psr.DealInfo.ProposalCid}, "$set": sr})
		uwm = uwm.SetUpsert(true)
		wms[i] = uwm
	}

	br, err := s.sdrc.BulkWrite(ctx, wms, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return fmt.Errorf("doing bulk write: %s", err)
	}
	if br.UpsertedCount != int64(len(psrs)) {
		return fmt.Errorf("bulk write upserted %d and should be %d", br.UpsertedCount, len(psrs))
	}

	return nil
}

func (s *Store) PersistRetrievalRecords(ctx context.Context, powName string, prrs []records.PowRetrievalRecord) error {
	now := time.Now()
	wms := make([]mongo.WriteModel, len(prrs))
	for i, prr := range prrs {
		rr := records.RetrievalRecord{
			LastUpdatedAt:      now,
			PowName:            powName,
			PowRetrievalRecord: prr,
		}
		id := retrievalID(prr)
		uwm := mongo.NewUpdateOneModel()
		uwm = uwm.SetFilter(bson.D{{Key: "_id", Value: id}})
		uwm = uwm.SetUpdate(bson.M{"$setOnInsert": bson.M{"_id": id}, "$set": rr})
		uwm = uwm.SetUpsert(true)
		wms[i] = uwm
	}

	br, err := s.rrc.BulkWrite(ctx, wms, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return fmt.Errorf("doing bulk write: %s", err)
	}
	if br.UpsertedCount != int64(len(prrs)) {
		return fmt.Errorf("bulk write upserted %d and should be %d", br.UpsertedCount, len(prrs))
	}

	return nil
}

// retrievalID generates the ID for a PowRetrievalRecord. We do the
// same calculation used in Powergate to generate it.
func retrievalID(rr records.PowRetrievalRecord) string {
	str := fmt.Sprintf("%v%v%v%v", rr.CreatedAt, rr.Address, rr.DealInfo.Miner, rr.DealInfo.RootCid)
	sum := md5.Sum([]byte(str))
	return fmt.Sprintf("%x", sum[:])
}

func (s *Store) ensureIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err := s.sdrc.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{primitive.E{Key: "pow_name", Value: 1}, primitive.E{Key: "pow_storage_deal_record.updated_at", Value: -1}},
		},
	})
	if err != nil {
		return fmt.Errorf("creating storage-deal records index: %s", err)
	}

	_, err = s.rrc.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{primitive.E{Key: "pow_name", Value: 1}, primitive.E{Key: "pow_retrieval_record.updated_at", Value: -1}},
		},
	})
	if err != nil {
		return fmt.Errorf("creating retrieval records index: %s", err)
	}

	return nil
}
