package service

import (
	"fmt"
	"time"

	"github.com/textileio/textile/v2/api/mindexd/model"
	"github.com/textileio/textile/v2/api/mindexd/pb"
	"github.com/textileio/textile/v2/api/mindexd/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toPbMinerIndexInfo(mi model.MinerInfo) *pb.MinerIndexInfo {
	return &pb.MinerIndexInfo{
		MinerAddr: mi.MinerID,
		Metadata: &pb.MetadataInfo{
			Location: mi.Metadata.Location,
		},
		Filecoin: &pb.FilecoinInfo{
			AskPrice:         mi.Filecoin.AskPrice,
			AskVerifiedPrice: mi.Filecoin.AskVerifiedPrice,
			MaxPieceSize:     mi.Filecoin.MaxPieceSize,
			MinPieceSize:     mi.Filecoin.MinPieceSize,
			RelativePower:    mi.Filecoin.RelativePower,
			SectorSize:       mi.Filecoin.SectorSize,
			ActiveSectors:    mi.Filecoin.ActiveSectors,
			FaultySectors:    mi.Filecoin.FaultySectors,
			UpdatedAt:        formatPbTime(mi.Filecoin.UpdatedAt),
		},
		Textile:   toPbTextileInfo(mi.Textile),
		UpdatedAt: formatPbTime(mi.UpdatedAt),
	}
}

func toPbTextileInfo(t model.TextileInfo) *pb.TextileInfo {
	ti := &pb.TextileInfo{
		Regions: make(map[string]*pb.TextileRegionInfo, len(t.Regions)),
		DealsSummary: &pb.DealsSummary{
			Total:       int64(t.DealsSummary.Total),
			Last:        formatPbTime(t.DealsSummary.Last),
			Failures:    int64(t.DealsSummary.Failures),
			LastFailure: formatPbTime(t.DealsSummary.LastFailure),
		},
		RetrievalsSummary: &pb.RetrievalsSummary{
			Total:       int64(t.RetrievalsSummary.Total),
			Last:        formatPbTime(t.RetrievalsSummary.Last),
			Failures:    int64(t.RetrievalsSummary.Failures),
			LastFailure: formatPbTime(t.RetrievalsSummary.LastFailure),
		},
		UpdatedAt: formatPbTime(t.UpdatedAt),
	}

	for region, info := range t.Regions {
		ti.Regions[region] = &pb.TextileRegionInfo{
			Deals: &pb.TextileDealsInfo{
				Total:         int64(info.Deals.Total),
				Last:          formatPbTime(info.Deals.Last),
				Failures:      int64(info.Deals.Failures),
				LastFailure:   formatPbTime(info.Deals.LastFailure),
				TailSealed:    toPbSealedDurationMins(info.Deals.TailSealed),
				TailTransfers: toPbTransferMiBPerSec(info.Deals.TailTransfers),
			},
			Retrievals: &pb.TextileRetrievalsInfo{
				Total:         int64(info.Retrievals.Total),
				Last:          formatPbTime(info.Retrievals.Last),
				Failures:      int64(info.Retrievals.Failures),
				LastFailure:   formatPbTime(info.Retrievals.LastFailure),
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
			TransferedAt: formatPbTime(t.TransferedAt),
		}
	}

	return ret
}

func toPbSealedDurationMins(ss []model.SealedDurationMins) []*pb.SealedDurationMins {
	ret := make([]*pb.SealedDurationMins, len(ss))

	for i, s := range ss {
		ret[i] = &pb.SealedDurationMins{
			DurationSeconds: int64(s.DurationSeconds),
			SealedAt:        formatPbTime(s.SealedAt),
		}
	}

	return ret
}

func fromPbQueryIndexRequestFilters(f *pb.QueryIndexRequestFilters) store.QueryIndexFilters {
	r := store.QueryIndexFilters{}
	if f != nil {
		r.MinerLocation = f.MinerLocation
	}
	return r
}

func fromPbQueryIndexRequestSort(s *pb.QueryIndexRequestSort) (store.QueryIndexSort, error) {
	field, err := fromPbQueryIndexRequestSortField(s.Field)
	if err != nil {
		return store.QueryIndexSort{}, fmt.Errorf("parsing sort field: %s", err)
	}

	return store.QueryIndexSort{
		Ascending:     s.Ascending,
		TextileRegion: s.TextileRegion,
		Field:         field,
	}, nil
}

func fromPbQueryIndexRequestSortField(field pb.QueryIndexRequestSortField) (store.QueryIndexSortField, error) {
	switch field {
	case pb.QueryIndexRequestSortField_TEXTILE_DEALS_TOTAL_SUCCESSFUL:
		return store.SortFieldTextileDealTotalSuccessful, nil
	case pb.QueryIndexRequestSortField_TEXTILE_DEALS_LAST_SUCCESSFUL:
		return store.SortFieldTextileDealLastSuccessful, nil
	case pb.QueryIndexRequestSortField_TEXTILE_RETRIEVALS_TOTAL_SUCCESSFUL:
		return store.SortFieldTextileRetrievalTotalSuccessful, nil
	case pb.QueryIndexRequestSortField_TEXTILE_RETRIEVALS_LAST_SUCCESSFUL:
		return store.SortFieldTextileRetrievalLastSuccessful, nil
	case pb.QueryIndexRequestSortField_ASK_PRICE:
		return store.SortFieldAskPrice, nil
	case pb.QueryIndexRequestSortField_VERIFIED_ASK_PRICE:
		return store.SortFieldVerifiedAskPrice, nil
	case pb.QueryIndexRequestSortField_ACTIVE_SECTORS:
		return store.SortFieldActiveSectors, nil
	default:
		return 0, fmt.Errorf("unkown sorting field %s", pb.QueryIndexRequestSortField_name[int32(field)])
	}
}

func toPbQueryIndexResponse(ss []model.MinerInfo) *pb.QueryIndexResponse {
	res := &pb.QueryIndexResponse{
		Miners: make([]*pb.QueryIndexResponseMiner, len(ss)),
	}
	for i, m := range ss {
		res.Miners[i] = &pb.QueryIndexResponseMiner{
			Miner: toPbMinerIndexInfo(m),
		}
	}

	return res
}

func formatPbTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
