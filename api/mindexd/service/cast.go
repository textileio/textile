package service

import (
	"fmt"

	"github.com/textileio/textile/v2/api/mindexd/model"
	"github.com/textileio/textile/v2/api/mindexd/pb"
)

func toPbMinerIndexInfo(mi model.MinerInfo) *pb.MinerIndexInfo {
	return &pb.MinerIndexInfo{
		MinerAddr: mi.MinerID,
		Filecoin: &pb.FilecoinInfo{
			AskPrice:         mi.Filecoin.AskPrice,
			AskVerifiedPrice: mi.Filecoin.AskVerifiedPrice,
			MaxPieceSize:     mi.Filecoin.MaxPieceSize,
			MinPieceSize:     mi.Filecoin.MinPieceSize,
			RelativePower:    mi.Filecoin.RelativePower,
			SectorSize:       mi.Filecoin.SectorSize,
			UpdatedAt:        mi.Filecoin.UpdatedAt.Unix(),
		},
		Textile:   toPbTextileInfo(mi.Textile),
		UpdatedAt: mi.UpdatedAt.Unix(),
	}
}

func toPbTextileInfo(t model.TextileInfo) *pb.TextileInfo {
	ti := &pb.TextileInfo{
		Regions:   make(map[string]*pb.TextileRegionInfo, len(t.Regions)),
		UpdatedAt: t.UpdatedAt.Unix(),
	}

	for region, info := range t.Regions {
		ti.Regions[region] = &pb.TextileRegionInfo{
			Deals: &pb.TextileDealsInfo{
				Total:         int64(info.Deals.Total),
				Last:          info.Deals.Last.Unix(),
				Failures:      int64(info.Deals.Failures),
				LastFailure:   info.Deals.LastFailure.Unix(),
				TailSealed:    toPbSealedDurationMins(info.Deals.TailSealed),
				TailTransfers: toPbTransferMiBPerSec(info.Deals.TailTransfers),
			},
			Retrievals: &pb.TextileRetrievalsInfo{
				Total:         int64(info.Retrievals.Total),
				Last:          info.Retrievals.Last.Unix(),
				Failures:      int64(info.Retrievals.Failures),
				LastFailure:   info.Retrievals.LastFailure.Unix(),
				TailTransfers: toPbTransferMiBPerSec(info.Retrievals.TailTransfers),
			},
		}
	}

	return ti
}

func toPbTransferMiBPerSec(ts []model.TransferMiBPerSec) []*pb.TransferMiBPerSec {
	ret := make([]*pb.TransferMiBPerSec, len(ts))

	for i, t := range ts {
		ret[i] = &pb.TransferMiBPerSec{
			MibPerSec:    t.MiBPerSec,
			TransferedAt: t.TransferedAt.Unix(),
		}
	}

	return ret
}

func toPbSealedDurationMins(ss []model.SealedDurationMins) []*pb.SealedDurationMins {
	ret := make([]*pb.SealedDurationMins, len(ss))

	for i, s := range ss {
		ret[i] = &pb.SealedDurationMins{
			DurationSeconds: int64(s.DurationSeconds),
			SealedAt:        s.SealedAt.Unix(),
		}
	}

	return ret
}

func fromPbQueryIndexRequestFilters(f *pb.QueryIndexRequestFilters) store.QueryIndexFilters {
	return store.QueryIndexFilters{
		MinerCountry:  f.MinerCountry,
		TextileRegion: f.TextileRegion,
	}
}

func fromPbQueryIndexRequestSort(s *pb.QueryIndexRequestSort) (store.QueryIndexSort, error) {
	field, err := fromPbQueryIndexRequestSortField(s.Field)
	if err != nil {
		return store.QueryIndexSort{}, fmt.Errorf("parsing sort field: %s", err)
	}

	return store.QueryIndexSort{
		Ascending: s.Ascending,
		Field:     field,
	}, nil
}

func fromPbQueryIndexRequestSortField(field pb.QueryIndexRequestSortField) (store.QueryIndexSortField, error) {
	switch field {
	case pb.QueryIndexRequestSortField_QUERY_INDEX_REQUEST_SORT_FIELD_TEXTILE_TOTAL_SUCCESSFUL:
		return store.SortFieldTextileTotalSuccessful
	case pb.QueryIndexRequestSortField_QUERY_INDEX_REQUEST_SORT_FIELD_TEXTILE_LAST_SUCCESSFUL:
		return store.SortFieldLastSuccessful
	case pb.QueryIndexRequestSortField_QUERY_INDEX_REQUEST_SORT_FIELD_ASK_PRICE:
		return store.SortFieldAskPrice
	case pb.QueryIndexRequestSortField_QUERY_INDEX_REQUEST_SORT_FIELD_VERIFIED_ASK_PRICE:
		return store.SortFieldVerifiedAskPrice
	case pb.QueryIndexRequestSortField_QUERY_INDEX_REQUEST_SORT_FIELD_ACTIVE_SECTORS:
		return store.SortFieldActiveSectors
	default:
		return 0, fmt.Errorf("unkown sorting field %s", pb.QueryIndexRequestSortField_name[int32(field)])
	}
}
