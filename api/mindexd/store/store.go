package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	logger "github.com/ipfs/go-log/v2"
	"github.com/textileio/textile/v2/api/mindexd/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	log = logger.Logger("record-store")

	errRecordNotFound = errors.New("record not found")
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

	res := make([]model.StorageDealRecord, 0, 1)
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

	res := make([]model.RetrievalRecord, 0, 1)
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

func (s *Store) PersistStorageDealRecords(ctx context.Context, powName string, psrs []model.PowStorageDealRecord) error {
	now := time.Now()

	wms := make([]mongo.WriteModel, len(psrs))
	for i, psr := range psrs {
		sr := model.StorageDealRecord{
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
	if br.UpsertedCount+br.ModifiedCount != int64(len(psrs)) {
		return fmt.Errorf("bulk write upserted %d and should be %d", br.UpsertedCount, len(psrs))
	}

	return nil
}

func (s *Store) PersistRetrievalRecords(ctx context.Context, powName string, prrs []model.PowRetrievalRecord) error {
	now := time.Now()

	wms := make([]mongo.WriteModel, len(prrs))
	for i, prr := range prrs {
		rr := model.RetrievalRecord{
			LastUpdatedAt:      now,
			PowName:            powName,
			PowRetrievalRecord: prr,
		}
		uwm := mongo.NewUpdateOneModel()
		uwm = uwm.SetFilter(bson.D{{Key: "_id", Value: prr.ID}})
		uwm = uwm.SetUpdate(bson.M{"$setOnInsert": bson.M{"_id": prr.ID}, "$set": rr})
		uwm = uwm.SetUpsert(true)
		wms[i] = uwm
	}

	br, err := s.rrc.BulkWrite(ctx, wms, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return fmt.Errorf("doing bulk write: %s", err)
	}
	if br.UpsertedCount+br.ModifiedCount != int64(len(prrs)) {
		return fmt.Errorf("bulk write upserted %d and should be %d", br.UpsertedCount, len(prrs))
	}

	return nil
}

func (s *Store) getStorageDealRecord(ctx context.Context, ID string) (model.StorageDealRecord, error) {
	filter := bson.M{"_id": ID}
	sr := s.sdrc.FindOne(ctx, filter)
	if sr.Err() == mongo.ErrNoDocuments {
		return model.StorageDealRecord{}, errRecordNotFound
	}
	if sr.Err() != nil {
		return model.StorageDealRecord{}, fmt.Errorf("get storage record: %s", sr.Err())
	}

	var sdr model.StorageDealRecord
	if err := sr.Decode(&sdr); err != nil {
		return model.StorageDealRecord{}, fmt.Errorf("decoding storage record: %s", err)
	}

	return sdr, nil
}

func (s *Store) getRetrievalRecord(ctx context.Context, ID string) (model.RetrievalRecord, error) {
	filter := bson.M{"_id": ID}
	sr := s.rrc.FindOne(ctx, filter)
	if sr.Err() == mongo.ErrNoDocuments {
		return model.RetrievalRecord{}, errRecordNotFound
	}
	if sr.Err() != nil {
		return model.RetrievalRecord{}, fmt.Errorf("get retrieval record: %s", sr.Err())
	}

	var rr model.RetrievalRecord
	if err := sr.Decode(&rr); err != nil {
		return model.RetrievalRecord{}, fmt.Errorf("decoding retrieval record: %s", err)
	}

	return rr, nil
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
