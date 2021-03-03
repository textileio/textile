package collector

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	pow "github.com/textileio/powergate/v2/api/client"
	powc "github.com/textileio/powergate/v2/api/client"
	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	"github.com/textileio/textile/v2/api/mindexd/model"
)

func (c *Collector) collectTargets(ctx context.Context) int {
	targets, err := c.store.GetPowergateTargets(ctx)
	if err != nil {
		log.Errorf("getting powergate targets: %s", err)
		return 0
	}

	var lock sync.Mutex
	var countImported int
	var wg sync.WaitGroup
	wg.Add(len(targets))
	for _, source := range targets {
		go func(source model.PowTarget) {
			defer wg.Done()

			ctx := context.WithValue(ctx, powc.AdminKey, source.AdminToken)
			client, err := pow.NewClient(source.APIEndpoint)
			if err != nil {
				log.Errorf("creating powergate client for %s: %s", source.Name, err)
				return
			}

			countSDR, err := c.collectNewStorageDealRecords(ctx, client, source)
			if err != nil {
				log.Errorf("collecting new storage deal records from %s: %s", source.Name, err)
			}
			log.Infof("collected %d storage-deal records from %s", countSDR, source.Name)

			countRR, err := c.collectNewRetrievalRecords(ctx, client, source)
			if err != nil {
				log.Errorf("collecting new retrieval records from %s: %s", source.Name, err)
			}
			log.Infof("collected %d retrieval records from %s", countRR, source.Name)

			lock.Lock()
			countImported += countSDR + countRR
			lock.Unlock()
		}(source)
	}

	wg.Wait()

	return countImported
}

func (c *Collector) collectNewStorageDealRecords(ctx context.Context, pc *pow.Client, source model.PowTarget) (int, error) {
	lastUpdatedAt, err := c.store.GetLastStorageDealRecordUpdatedAt(ctx, source.Name)
	if err != nil {
		return 0, fmt.Errorf("get last updated-at: %s", err)
	}

	var totalCount int
	for {
		ctx, cancel := context.WithTimeout(ctx, c.cfg.fetchTimeout)
		defer cancel()

		res, err := pc.Admin.Records.GetUpdatedStorageDealRecordsSince(ctx, lastUpdatedAt, c.cfg.fetchLimit)
		if err != nil {
			return 0, fmt.Errorf("get storage deal records: %s", err)
		}

		if len(res.Records) == 0 {
			break
		}
		totalCount += len(res.Records)
		log.Debugf("%s collected a total of %d new/updated records, last %v", source.Name, len(res.Records), res.Records[len(res.Records)-1].UpdatedAt)

		records := toStorageDealRecords(res.Records)
		if err := c.store.PersistStorageDealRecords(ctx, source.Name, source.Region, records); err != nil {
			return 0, fmt.Errorf("persist fetched records: %s", err)
		}

		sort.Slice(res.Records, func(i, j int) bool {
			return res.Records[i].UpdatedAt.AsTime().Before(res.Records[j].UpdatedAt.AsTime())
		})
		lastUpdatedAt = res.Records[len(res.Records)-1].UpdatedAt.AsTime()

		// If we fetched less than limit, then there're no
		// more records to fetch.
		if len(res.Records) < c.cfg.fetchLimit {
			break
		}
	}

	return totalCount, nil
}

func (c *Collector) collectNewRetrievalRecords(ctx context.Context, pc *pow.Client, source model.PowTarget) (int, error) {
	lastUpdatedAt, err := c.store.GetLastRetrievalRecordUpdatedAt(ctx, source.Name)
	if err != nil {
		return 0, fmt.Errorf("get last updated-at: %s", err)
	}

	var totalCount int
	for {
		ctx, cancel := context.WithTimeout(ctx, c.cfg.fetchTimeout)
		defer cancel()

		res, err := pc.Admin.Records.GetUpdatedRetrievalRecordsSince(ctx, lastUpdatedAt.Add(time.Nanosecond), c.cfg.fetchLimit)
		if err != nil {
			return 0, fmt.Errorf("get retrieval records: %s", err)
		}

		if len(res.Records) == 0 {
			break
		}
		totalCount += len(res.Records)

		records := toRetrievalRecords(res.Records)
		if err := c.store.PersistRetrievalRecords(ctx, source.Name, source.Region, records); err != nil {
			return 0, fmt.Errorf("persist fetched records: %s", err)
		}

		lastUpdatedAt = res.Records[len(res.Records)-1].UpdatedAt.AsTime()

		// If we fetched less than limit, then there're no
		// more records to fetch.
		if len(res.Records) < c.cfg.fetchLimit {
			break
		}
	}

	return totalCount, nil
}

func toStorageDealRecords(rs []*userPb.StorageDealRecord) []model.PowStorageDealRecord {
	ret := make([]model.PowStorageDealRecord, len(rs))
	for i, s := range rs {
		sr := model.PowStorageDealRecord{
			RootCid:           s.RootCid,
			Address:           s.Address,
			Pending:           s.Pending,
			TransferSize:      s.TransferSize,
			DataTransferStart: s.DataTransferStart.AsTime(),
			DataTransferEnd:   s.DataTransferEnd.AsTime(),
			SealingStart:      s.SealingStart.AsTime(),
			SealingEnd:        s.SealingEnd.AsTime(),
			Failed:            s.ErrMsg != "",
			ErrMsg:            s.ErrMsg,
			CreatedAt:         s.Time,
			UpdatedAt:         s.UpdatedAt.AsTime(),
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid:     s.DealInfo.ProposalCid,
				StateId:         s.DealInfo.StateId,
				StateName:       s.DealInfo.StateName,
				Miner:           s.DealInfo.Miner,
				PieceCid:        s.DealInfo.PieceCid,
				Size:            s.DealInfo.Size,
				PricePerEpoch:   s.DealInfo.PricePerEpoch,
				StartEpoch:      s.DealInfo.StartEpoch,
				Duration:        s.DealInfo.Duration,
				DealId:          s.DealInfo.DealId,
				ActivationEpoch: s.DealInfo.ActivationEpoch,
				Message:         s.DealInfo.Message,
			},
		}
		ret[i] = sr
	}

	return ret
}

func toRetrievalRecords(rs []*userPb.RetrievalDealRecord) []model.PowRetrievalRecord {
	ret := make([]model.PowRetrievalRecord, len(rs))
	for i, r := range rs {
		pr := model.PowRetrievalRecord{
			ID:                r.Id,
			Address:           r.Address,
			DataTransferStart: r.DataTransferStart.AsTime(),
			DataTransferEnd:   r.DataTransferEnd.AsTime(),
			BytesReceived:     r.BytesReceived,
			Failed:            r.ErrMsg != "",
			ErrMsg:            r.ErrMsg,
			CreatedAt:         r.Time,
			UpdatedAt:         r.UpdatedAt.AsTime(),
			DealInfo: model.PowRetrievalRecordDealInfo{
				Miner:    r.DealInfo.Miner,
				MinPrice: r.DealInfo.MinPrice,
				RootCid:  r.DealInfo.RootCid,
				Size:     r.DealInfo.Size,
			},
		}
		ret[i] = pr
	}

	return ret
}
