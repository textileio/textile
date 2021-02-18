package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/textileio/textile/v2/api/mindexd/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/text/language"
)

var (
	errRecordNotFound = errors.New("record not found")
)

// GetPowergateTargets returns the external Powergates that will be
// polled to import storage-deal and retrieval records.
func (s *Store) GetPowergateTargets(ctx context.Context) ([]model.PowTarget, error) {
	c, err := s.ptc.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("cursor for powergate targets: %s", err)
	}
	defer c.Close(ctx)

	var res []model.PowTarget
	if err := c.All(ctx, &res); err != nil {
		return nil, fmt.Errorf("decoding powergate targets:  %s", err)
	}

	return res, nil
}

// GetLastStorageDealRecordUpdatedAt returns the latests updated-at timestamp of
// a particular external Powergate. This might serve to ask the external powergate,
// for newer records since this timestamp.
func (s *Store) GetLastStorageDealRecordUpdatedAt(ctx context.Context, powName string) (time.Time, error) {
	filter := bson.M{"pow_name": powName}
	opts := options.Find()
	opts = opts.SetSort(bson.D{{Key: "pow_storage_deal_record.updated_at", Value: -1}})
	opts = opts.SetLimit(1)
	opts = opts.SetProjection(bson.M{"pow_storage_deal_record.updated_at": 1})
	c, err := s.sdrc.Find(ctx, filter, opts)
	if err != nil {
		return time.Time{}, fmt.Errorf("executing find: %s", err)
	}
	defer func() {
		if err := c.Close(ctx); err != nil {
			log.Errorf("close last storage deal record updatedat cursor: %s", err)
		}
	}()

	var res []model.StorageDealRecord
	if err := c.All(ctx, &res); err != nil {
		return time.Time{}, fmt.Errorf("fetching find result: %s", err)
	}
	if len(res) == 0 {
		// If we don't have any records, just return a very old lastUpdatedAt
		// to let it start from the beginning.
		return time.Time{}, nil
	}

	return (res)[0].PowStorageDealRecord.UpdatedAt, nil
}

// GetLastRetrievalRecordUpdatedAt returns the latests updated-at timestamp of
// a particular external Powergate. This might serve to ask the external powergate,
// for newer records since this timestamp.
func (s *Store) GetLastRetrievalRecordUpdatedAt(ctx context.Context, powName string) (time.Time, error) {
	filter := bson.M{"pow_name": powName}
	opts := options.Find()
	opts = opts.SetSort(bson.D{{Key: "pow_retrieval_record.updated_at", Value: -1}})
	opts = opts.SetLimit(1)
	opts = opts.SetProjection(bson.M{"pow_retrieval_record.updated_at": 1})
	c, err := s.rrc.Find(ctx, filter, opts)
	if err != nil {
		return time.Time{}, fmt.Errorf("executing find: %s", err)
	}
	defer func() {
		if err := c.Close(ctx); err != nil {
			log.Errorf("close last retrieval record updatedat cursor: %s", err)
		}
	}()

	res := make([]model.RetrievalRecord, 0, 1)
	if err := c.All(ctx, &res); err != nil {
		return time.Time{}, fmt.Errorf("fetching find result: %s", err)
	}
	if len(res) == 0 {
		// If we don't have any records, just return a very old lastUpdatedAt
		// to let it start from the beginning.
		return time.Time{}, nil
	}

	return (res)[0].PowRetrievalRecord.UpdatedAt, nil
}

// PersistStorageDealRecords merge a set of storage-deal records for an external powergate.
// The action has upsert semantics, so if the record already exists, it's updated.
func (s *Store) PersistStorageDealRecords(ctx context.Context, powName, region string, psrs []model.PowStorageDealRecord) error {
	if powName == "" {
		return fmt.Errorf("powergate name is empty")
	}
	if _, err := language.ParseRegion(region); err != nil {
		return fmt.Errorf("region %s isn't a valid M49 code: %s", region, err)
	}

	now := time.Now()
	wms := make([]mongo.WriteModel, len(psrs))
	for i, psr := range psrs {
		sr := model.StorageDealRecord{
			LastUpdatedAt:        now,
			PowName:              powName,
			Region:               region,
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

// PersistRetrievalRecords merge a set of retrieval records for an external powergate.
// The action has upsert semantics, so if the record already exists, it's updated.
func (s *Store) PersistRetrievalRecords(ctx context.Context, powName, region string, prrs []model.PowRetrievalRecord) error {
	if powName == "" {
		return fmt.Errorf("powergate name is empty")
	}
	if _, err := language.ParseRegion(region); err != nil {
		return fmt.Errorf("region %s isn't a valid M49 code: %s", region, err)
	}

	now := time.Now()
	wms := make([]mongo.WriteModel, len(prrs))
	for i, prr := range prrs {
		rr := model.RetrievalRecord{
			LastUpdatedAt:      now,
			PowName:            powName,
			Region:             region,
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
