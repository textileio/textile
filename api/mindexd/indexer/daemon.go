package indexer

import (
	"context"
	"fmt"
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
		i.updateMinerInfo(ctx, m)
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
	panic("TTODO")
	return nil
	/*
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

		if err := i.store.PutFilecoinInfo(ctx, miner, onchain); err != nil {
			return fmt.Errorf("put miner on-chain info in store: %s", err)
		}
	*/
}

func (i *Indexer) getActiveMiners(ctx context.Context) ([]string, error) {
	panic("TTODO")
	return nil, nil
	/*
		ms, err := i.pow.Indices.GetMinersWithPower(ctx)
		if err != nil {
			return nil, fmt.Errorf("get miners with power from powergate: %s", err)
		}

		return ms
	*/
}

func (i *Indexer) updateTextileMinerInfo(ctx context.Context, miner string) error {
	if err := i.rstore.UpdateTextileDealsInfo(ctx); err != nil {
		return fmt.Errorf("generate textile miner deals info: %s", err)
	}

	// TTODO
	/*
		if err := i.rstore.generateTextileRegionalRetrievalsInfo(ctx, miner); err != nil {
			return fmt.Errorf("generate textile retrievals info: %s", err)
		}
	*/

	return nil
}
