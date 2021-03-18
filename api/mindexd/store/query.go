package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/textileio/textile/v2/api/mindexd/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrMinerNotExists = errors.New("miner doesn't exists")
)

type QueryIndexSortField int

const (
	SortFieldTextileDealTotalSuccessful QueryIndexSortField = iota
	SortFieldTextileDealLastSuccessful
	SortFieldTextileRetrievalTotalSuccessful
	SortFieldTextileRetrievalLastSuccessful
	SortFieldAskPrice
	SortFieldVerifiedAskPrice
	SortFieldActiveSectors
)

type QueryIndexFilters struct {
	MinerLocation string
}

type QueryIndexSort struct {
	Ascending     bool
	TextileRegion string
	Field         QueryIndexSortField
}

func (s *Store) GetMinerInfo(ctx context.Context, miner string) (model.MinerInfo, error) {
	filter := bson.M{"_id": miner}
	r := s.idxc.FindOne(ctx, filter)
	if r.Err() == mongo.ErrNoDocuments {
		return model.MinerInfo{}, ErrMinerNotExists
	}

	var mi model.MinerInfo
	if err := r.Decode(&mi); err != nil {
		return model.MinerInfo{}, fmt.Errorf("decoding miner info: %s", err)
	}

	return mi, nil
}

func (s *Store) GetAllMiners(ctx context.Context) ([]model.MinerInfo, error) {
	r, err := s.idxc.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("get all miners: %s", err)
	}
	defer r.Close(ctx)

	var ms []model.MinerInfo
	if err := r.All(ctx, &ms); err != nil {
		return nil, fmt.Errorf("decoding all results: %s", err)
	}

	return ms, nil
}

func (s *Store) QueryIndex(ctx context.Context, filters QueryIndexFilters, sort QueryIndexSort, limit int, offset int64) ([]model.MinerInfo, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit should be greater than zero")
	}

	qFilters, qSort, err := buildMongoFiltersAndSort(filters, sort)
	if err != nil {
		return nil, fmt.Errorf("building filters and sort: %s", err)
	}

	opts := options.Find()
	opts = opts.SetSort(qSort).SetCollation(&options.Collation{Locale: "en", NumericOrdering: true})
	opts = opts.SetLimit(int64(limit))
	opts = opts.SetSkip(offset)
	c, err := s.idxc.Find(ctx, qFilters, opts)
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer func() {
		if err := c.Close(ctx); err != nil {
			log.Errorf("closing query index cursor: %s", err)
		}
	}()
	var ms []model.MinerInfo
	if err := c.All(ctx, &ms); err != nil {
		return nil, fmt.Errorf("decoding all results: %s", err)
	}

	return ms, nil
}

func buildMongoFiltersAndSort(filters QueryIndexFilters, sort QueryIndexSort) (bson.M, bson.D, error) {
	// Filter
	f := bson.M{}
	if filters.MinerLocation != "" {
		f["metadata.location"] = filters.MinerLocation
	}

	// Sort
	s := bson.E{}
	sortVal := 1
	if !sort.Ascending {
		sortVal = -1
	}

	switch sort.Field {
	case SortFieldTextileDealTotalSuccessful:
		s = bson.E{Key: "textile.deals_summary.total", Value: sortVal}
		if sort.TextileRegion != "" {
			s = bson.E{Key: "textile.regions." + sort.TextileRegion + ".deals.total", Value: sortVal}
		}
	case SortFieldTextileDealLastSuccessful:
		s = bson.E{Key: "textile.deals_summary.last", Value: sortVal}
		if sort.TextileRegion != "" {
			s = bson.E{Key: "textile.regions." + sort.TextileRegion + ".deals.last", Value: sortVal}
		}
	case SortFieldTextileRetrievalTotalSuccessful:
		s = bson.E{Key: "textile.retrievals_summary.total", Value: sortVal}
		if sort.TextileRegion != "" {
			s = bson.E{Key: "textile.regions." + sort.TextileRegion + ".retrievals.total", Value: sortVal}
		}
	case SortFieldTextileRetrievalLastSuccessful:
		s = bson.E{Key: "textile.retrievals_summary.last", Value: sortVal}
		if sort.TextileRegion != "" {
			s = bson.E{Key: "textile.regions." + sort.TextileRegion + ".retrievals.last", Value: sortVal}
		}
	case SortFieldAskPrice:
		s = bson.E{Key: "filecoin.ask_price", Value: sortVal}
	case SortFieldVerifiedAskPrice:
		s = bson.E{Key: "filecoin.ask_verified_price", Value: sortVal}
	case SortFieldActiveSectors:
		s = bson.E{Key: "filecoin.active_sectors", Value: sortVal}
	default:
		return nil, nil, fmt.Errorf("unkown sort field")
	}

	return f, bson.D{s}, nil
}
