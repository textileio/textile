package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/textile/v2/api/mindexd/model"
)

func (i *Indexer) generateIndex(ctx context.Context) error {
	miners, err := i.getActiveMiners(ctx)
	if err != nil {
		return fmt.Errorf("getting active miners: %s", err)
	}

	// Since we're updating a index without tight timing constraints,
	// we do this serially to avoid overwhealming the database with requests.
	// If we happen to realize this needs to be faster, we can easily
	// parallelize this.
	for _, m := range miners {
		if err := i.updateMinerInfo(ctx, m); err != nil {
			log.Errorf("updating information for miner %s: %s", m, err)
		}
	}

	return nil
}

func (i *Indexer) updateMinerInfo(ctx context.Context, miner string) {
	if err := i.updateOnChainMinerInfo(ctx, miner); err != nil {
		log.Errorf("updating on-chain info: %s", err)
	}

	if err := i.updateTextileMinerInfo(ctx, miner); err != nil {
		log.Errorf("updating textile info: %s", err)
	}
}

func (i *Indexer) updateOnChainMinerInfo(ctx context.Context, miner string) error {
	info, err := i.pow.Indices.GetMinerInfo(ctx, miner)
	if err != nil {
		return fmt.Errorf("get miner on-chain info from powergate: %s", err)
	}

	onchain := model.FilecoinInfo{
		RelativePower: info.RelativePower,
		AskPrice:      info.AskPrice,
		MinPieceSize:  info.MinPieceSize,
		MaxPieceSize:  info.MaxPieceSize,
		UpdatedAt:     time.Now(),
	}

	if err := i.store.PutMinerOnChainInfo(ctx, onchain); err != nil {
		return fmt.Errorf("put miner on-chain info in store: %s", err)
	}

	return nil
}

func (i *Indexer) getActiveMiners(ctx context.Context) ([]string, error) {
	ms, err := i.pow.Indices.GetMinersWithPower(ctx)
	if err != nil {
		return nil, fmt.Errorf("get miners with power from powergate: %s", err)
	}

	return ms
}

func (i *Indexer) updateTextileMinerInfo(ctx context.Context, miner string) error {
	dealsInfo, err := i.generateTextileMinerDealsInfo(ctx, miner)
	if err != nil {
		return fmt.Errorf("generate textile miner deals info: %s", err)
	}

	retrievalsInfo, err := i.generateTextileRetrievalsInfo(ctx, miner)
	if err != nil {
		return fmt.Errorf("generate textile retrievals info: %s", err)
	}

	textile := model.TextileInfo{
		Deals:      dealsInfo,
		Retrievals: retrievalsInfo,
		UpdatedAt:  time.Now(),
	}

	if err := i.store.PutMinerTextileInfo(ctx, textile); err != nil {
		return fmt.Errorf("put miner textile info in store: 5s", err)
	}

	return nil
}

func (i *Indexer) updateTextileMinersDealsInfo(ctx context.Context, miner string) (model.TextileDealsInfo, error) {
	generalInfo, err := i.recordStore.GetGeneralInfo(ctx, miner)
	if err != nil {
		return model.TextileDealsInfo{}, fmt.Errorf("get records general info: %s", err)
	}
	failureInfo, err := i.recordStore.GetFailureInfo(ctx, miner)
	if err != nil {
		return model.TextileDealsInfo{}, fmt.Errorf("get records failure info: %s", err)
	}
	transferInfo, err := i.recordStore.GetTransferInfo(ctx, miner)
	if err != nil {
		return model.TextileDealsInfo{}, fmt.Errorf("get records transfer info: %s", err)
	}
	sealingInfo, err := i.recordStore.GetSealingInfo(ctx, miner)
	if err != nil {
		return model.TextileDealsInfo{}, fmt.Errorf("get records sealing info: %s", err)
	}

	dealsInfo := model.TextileDealsInfo{
		Total: generalInfo.Total,
		Last:  generalInfo.Last,

		Failures:    failureInfo.Total,
		LastFailure: failureInfo.Last,

		LastTransferMiBPerSec:     transferInfo.LastMiBPerSec,
		Max7DaysTransferMiBPerSec: transferInfo.Max7DaysMiBPerSec,

		LastDealSealedDurationMins: sealingInfo.LastDurationMins,
		MinDealSealedDurationMins:  sealingInfo.MinDurationMins,
	}

	return dealsInfo, nil
}
