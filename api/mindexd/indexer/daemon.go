package indexer

import (
	"context"
	"fmt"
	"time"

	powAdmin "github.com/textileio/powergate/v2/api/client/admin"
	"github.com/textileio/textile/v2/api/mindexd/model"
)

var (
	minersBatchSize = 100
)

func (i *Indexer) generateIndex(ctx context.Context) error {
	if err := i.updateTextileMinerInfo(ctx); err != nil {
		log.Errorf("updating textile info: %s", err)
	}

	miners, err := i.getActiveMiners(ctx)
	if err != nil {
		return fmt.Errorf("getting active miners: %s", err)
	}
	for start := 0; start < len(miners); start += minersBatchSize {
		end := start + minersBatchSize
		if end > len(miners) {
			end = len(miners)
		}

		if err := i.updateOnChainMinersInfo(ctx, miners[start:end]); err != nil {
			return fmt.Errorf("updating on-chain info for miners: %s", err)
		}
	}

	return nil
}

func (i *Indexer) updateOnChainMinersInfo(ctx context.Context, miners []string) error {
	minfos, err := i.pow.Admin.Indices.GetMinerInfo(ctx, miners...)
	if err != nil {
		return fmt.Errorf("get miner on-chain info from powergate: %s", err)
	}

	for _, mi := range minfos.MinersInfo {
		onchain := model.FilecoinInfo{
			RelativePower:    mi.RelativePower,
			AskPrice:         mi.AskPrice,
			AskVerifiedPrice: mi.AskVerifiedPrice,
			MinPieceSize:     mi.MinPieceSize,
			MaxPieceSize:     mi.MaxPieceSize,
			SectorSize:       mi.SectorSize,
			UpdatedAt:        time.Now(),
		}

		if err := i.store.PutFilecoinInfo(ctx, mi.Address, onchain); err != nil {
			log.Errorf("put miner on-chain info in store: %s", err)
		}
	}

	return nil
}

func (i *Indexer) getActiveMiners(ctx context.Context) ([]string, error) {
	ms, err := i.pow.Admin.Indices.GetMiners(ctx, powAdmin.WithPowerGreaterThanZero(true))
	if err != nil {
		return nil, fmt.Errorf("get miners with power from powergate: %s", err)
	}

	miners := make([]string, len(ms.Miners))
	for i, m := range ms.Miners {
		miners[i] = m.Address
	}

	return miners, nil
}

func (i *Indexer) updateTextileMinerInfo(ctx context.Context) error {
	if err := i.store.UpdateTextileDealsInfo(ctx); err != nil {
		return fmt.Errorf("update textile miner deals info: %s", err)
	}

	if err := i.store.UpdateTextileRetrievalsInfo(ctx); err != nil {
		return fmt.Errorf("update textile retrievals info: %s", err)
	}

	return nil
}
