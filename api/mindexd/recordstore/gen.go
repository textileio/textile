package recordstore

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/textile/v2/api/mindexd/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	tailLimit = 50
)

type minerRegion struct {
	miner  string
	region string
}

func (s *Store) UpdateTextileDealsInfo(ctx context.Context) error {
	minerRegions, err := s.regenerateTextileDealsTotalsAndLasts(ctx)
	if err != nil {
		return fmt.Errorf("regenerating textile deals totals and lasts: %s", err)
	}

	for _, mr := range minerRegions {
		if err := s.regenerateTextileDealsTailMetrics(ctx, mr); err != nil {
			return fmt.Errorf("updating textile retrieval tail metrics: %s", err)
		}
	}

	return nil
}

func (s *Store) UpdateTextileRetrievalsInfo(ctx context.Context) error {
	minerRegions, err := s.regenerateTextileRetrievalsTotalsAndLasts(ctx)
	if err != nil {
		return fmt.Errorf("regenerating textile retrievals totals and lasts: %s", err)
	}

	for _, mr := range minerRegions {
		if err := s.regenerateTextileRetrievalsTailMetrics(ctx, mr); err != nil {
			return fmt.Errorf("updating textile retrievals tail metrics: %s", err)
		}
	}

	return nil
}

type regionalGeneralItem struct {
	ID struct {
		Region string `bson:"region"`
		Miner  string `bson:"miner"`
		Failed bool   `bson:"failed"`
	} `bson:"_id"`
	Total int   `bson:"total"`
	Last  int64 `bson:"last"`
}

func (s *Store) regenerateTextileDealsTailMetrics(ctx context.Context, mr minerRegion) error {
	filter := bson.M{
		"pow_storage_deal_record.deal_info.miner": mr.miner,
		"region":                          mr.region,
		"pow_storage_deal_record.pending": false,
		"pow_storage_deal_record.failed":  false,
	}
	sort := bson.D{bson.E{Key: "pow_storage_deal_record.updated_at", Value: -1}}
	proj := bson.M{
		"pow_storage_deal_record.transfer_size":      1,
		"pow_storage_deal_record.datatransfer_start": 1,
		"pow_storage_deal_record.datatransfer_end":   1,
		"pow_storage_deal_record.sealing_start":      1,
		"pow_storage_deal_record.sealing_end":        1,
		"pow_storage_deal_record.updated_at":         1,
	}
	opts := options.Find().SetSort(sort).SetProjection(proj).SetLimit(int64(tailLimit))
	c, err := s.sdrc.Find(ctx, filter, opts)
	if err != nil {
		return fmt.Errorf("finding miner-region records: %s", err)
	}
	defer c.Close(ctx)

	var records []model.StorageDealRecord
	if err := c.All(ctx, &records); err != nil {
		return fmt.Errorf("getting all results: %s", err)
	}

	tailTransfers := make([]model.TransferMiBPerSec, 0, len(records))
	tailSealed := make([]model.SealedDurationMins, 0, len(records))
	for _, record := range records {
		psd := record.PowStorageDealRecord

		if psd.DataTransferEnd-psd.DataTransferStart > 0 {
			tailTransfers = append(tailTransfers, model.TransferMiBPerSec{
				TransferedAt: time.Unix(psd.DataTransferEnd, 0),
				MiBPerSec:    float64(psd.TransferSize) / float64((psd.DataTransferEnd - psd.DataTransferStart)) / 1024 / 1024,
			})
		}

		if psd.SealingEnd-psd.SealingStart > 0 {
			tailSealed = append(tailSealed, model.SealedDurationMins{
				SealedAt:        time.Unix(psd.SealingEnd, 0),
				DurationSeconds: int(psd.SealingEnd - psd.SealingStart),
			})
		}
	}

	now := time.Now()
	prefix := "textile.regions." + mr.region
	filter = bson.M{"_id": mr.miner}
	setFields := bson.M{
		prefix + ".deals.tail_transfers": tailTransfers,
		prefix + ".deals.tail_sealed":    tailSealed,
		"textile.updated_at":             now,
		"updated_at":                     now,
	}
	if _, err := s.idxc.UpdateOne(ctx, filter, bson.M{"$set": setFields}); err != nil {
		return fmt.Errorf("updating miner index tails: %s", err)
	}

	return nil
}

func (s *Store) regenerateTextileDealsTotalsAndLasts(ctx context.Context) ([]minerRegion, error) {
	pipeline := bson.A{
		bson.M{
			"$match": bson.M{
				"pow_storage_deal_record.pending": false,
			},
		},
		bson.M{
			"$group": bson.M{
				"_id": bson.D{
					{Key: "miner", Value: "$pow_storage_deal_record.deal_info.miner"},
					{Key: "region", Value: "$region"},
					{Key: "failed", Value: "$pow_storage_deal_record.failed"},
				},
				"total": bson.M{
					"$sum": 1,
				},
				"last": bson.M{
					"$max": "$pow_storage_deal_record.updated_at",
				},
			},
		},
	}
	c, err := s.sdrc.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate in mongo: %s", err)
	}
	defer c.Close(ctx)

	mr, err := s.updateTextileRegion(ctx, "deals", c)
	if err != nil {
		return nil, fmt.Errorf("updating region: %s", err)
	}

	return mr, nil
}

func (s *Store) regenerateTextileRetrievalsTotalsAndLasts(ctx context.Context) ([]minerRegion, error) {
	pipeline := bson.A{
		bson.M{
			"$group": bson.M{
				"_id": bson.D{
					{Key: "miner", Value: "$pow_retrieval_record.deal_info.miner"},
					{Key: "region", Value: "$region"},
					{Key: "failed", Value: "$pow_retrieval_record.failed"},
				},
				"total": bson.M{
					"$sum": 1,
				},
				"last": bson.M{
					"$max": "$pow_retrieval_record.updated_at",
				},
			},
		},
	}
	c, err := s.rrc.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate in mongo: %s", err)
	}
	defer c.Close(ctx)

	mr, err := s.updateTextileRegion(ctx, "retrievals", c)
	if err != nil {
		return nil, fmt.Errorf("updating region: %s", err)
	}

	return mr, nil
}

func (s *Store) regenerateTextileRetrievalsTailMetrics(ctx context.Context, mr minerRegion) error {
	filter := bson.M{
		"pow_retrieval_record.deal_info.miner": mr.miner,
		"region":                               mr.region,
		"pow_retrieval_record.failed":          false,
	}
	sort := bson.D{bson.E{Key: "pow_storage_deal_record.updated_at", Value: -1}}
	proj := bson.M{
		"pow_retrieval_record.bytes_received":     1,
		"pow_retrieval_record.datatransfer_start": 1,
		"pow_retrieval_record.datatransfer_end":   1,
		"pow_storage_deal_record.updated_at":      1,
	}
	opts := options.Find().SetSort(sort).SetProjection(proj).SetLimit(int64(tailLimit))
	c, err := s.rrc.Find(ctx, filter, opts)
	if err != nil {
		return fmt.Errorf("finding miner-region records: %s", err)
	}
	defer c.Close(ctx)

	var records []model.RetrievalRecord
	if err := c.All(ctx, &records); err != nil {
		return fmt.Errorf("getting all results: %s", err)
	}

	tailTransfers := make([]model.TransferMiBPerSec, 0, len(records))
	for _, record := range records {
		psd := record.PowRetrievalRecord

		if psd.DataTransferEnd-psd.DataTransferStart > 0 {
			tailTransfers = append(tailTransfers, model.TransferMiBPerSec{
				TransferedAt: time.Unix(psd.DataTransferEnd, 0),
				MiBPerSec:    float64(psd.BytesReceived) / float64((psd.DataTransferEnd - psd.DataTransferStart)) / 1024 / 1024,
			})
		}
	}

	now := time.Now()
	prefix := "textile.regions." + mr.region
	filter = bson.M{"_id": mr.miner}
	setFields := bson.M{
		prefix + ".retrievals.tail_transfers": tailTransfers,
		"textile.updated_at":                  now,
		"updated_at":                          now,
	}
	if _, err := s.idxc.UpdateOne(ctx, filter, bson.M{"$set": setFields}); err != nil {
		return fmt.Errorf("updating miner index tails: %s", err)
	}

	return nil
}

func (s *Store) updateTextileRegion(ctx context.Context, prefixSuffix string, c *mongo.Cursor) ([]minerRegion, error) {
	mrm := map[minerRegion]struct{}{}
	opt := options.Update().SetUpsert(true)
	for c.Next(ctx) {
		var i regionalGeneralItem
		if err := c.Decode(&i); err != nil {
			return nil, fmt.Errorf("decoding item result: %s", err)
		}
		mrm[minerRegion{miner: i.ID.Miner, region: i.ID.Region}] = struct{}{}

		setFields := bson.M{}
		fieldPrefix := "textile.regions." + i.ID.Region + "." + prefixSuffix
		if i.ID.Failed {
			setFields[fieldPrefix+".failures"] = i.Total
			setFields[fieldPrefix+".last_failure"] = time.Unix(i.Last, 0)
		} else {
			setFields[fieldPrefix+".total"] = i.Total
			setFields[fieldPrefix+".last"] = time.Unix(i.Last, 0)
		}

		filter := bson.M{"_id": i.ID.Miner}
		update := bson.M{
			"$set":         setFields,
			"$setOnInsert": bson.M{"_id": i.ID.Miner},
		}
		_, err := s.idxc.UpdateOne(ctx, filter, update, opt)
		if err != nil {
			return nil, fmt.Errorf("upserting total/last: %s", err)
		}
	}
	if c.Err() != nil {
		return nil, fmt.Errorf("cursor error: %s", c.Err())
	}

	mr := make([]minerRegion, 0, len(mrm))
	for k := range mrm {
		mr = append(mr, k)
	}

	return mr, nil
}

// TTODO: create indexes for the above queries.
