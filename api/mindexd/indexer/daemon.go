package indexer

import (
	"context"
	"fmt"
	"time"

	powc "github.com/textileio/powergate/v2/api/client"
	powAdmin "github.com/textileio/powergate/v2/api/client/admin"
	"github.com/textileio/textile/v2/api/mindexd/model"
)

var (
	minersBatchSize = 100
)

func (i *Indexer) generateIndex(ctx context.Context) error {
	log.Debugf("starting updating textile miners info")
	if err := i.updateTextileMinerInfo(ctx); err != nil {
		log.Errorf("updating textile info: %s", err)
	}

	log.Debugf("getting miners with power")
	miners, err := i.getActiveMiners(ctx)
	if err != nil {
		return fmt.Errorf("getting active miners: %s", err)
	}

	log.Debugf("importing on-chain miners data started")
	for start := 0; start < len(miners); start += minersBatchSize {
		log.Debugf("importing on-chain batch number %d", start)
		end := start + minersBatchSize
		if end > len(miners) {
			end = len(miners)
		}

		if err := i.updateOnChainMinersInfo(ctx, miners[start:end]); err != nil {
			return fmt.Errorf("updating on-chain info for miners: %s", err)
		}
	}
	log.Debugf("importing on-chain miners data finished")

	return nil
}

func (i *Indexer) updateOnChainMinersInfo(ctx context.Context, miners []string) error {
	ctxAdmin := context.WithValue(ctx, powc.AdminKey, i.powAdminToken)
	minfos, err := i.pow.Admin.Indices.GetMinerInfo(ctxAdmin, miners...)
	if err != nil {
		return fmt.Errorf("get miner on-chain info from powergate: %s", err)
	}
	for _, mi := range minfos.MinersInfo {
		onchain := model.FilecoinInfo{
			RelativePower:    mi.RelativePower,
			AskPrice:         mi.AskPrice,
			AskVerifiedPrice: mi.AskVerifiedPrice,
			MinPieceSize:     int64(mi.MinPieceSize),
			MaxPieceSize:     int64(mi.MaxPieceSize),
			SectorSize:       int64(mi.SectorSize),
			ActiveSectors:    int64(mi.SectorsActive),
			FaultySectors:    int64(mi.SectorsFaulty),
			UpdatedAt:        time.Now(),
		}

		if err := i.store.PutFilecoinInfo(ctx, mi.Address, onchain); err != nil {
			log.Errorf("put miner on-chain info in store: %s", err)
		}

		if err := i.store.PutMetadataLocation(ctx, mi.Address, mi.Location); err != nil {
			log.Errorf("put miner on-chain info in store: %s", err)
		}
	}

	return nil
}

func (i *Indexer) getActiveMiners(ctx context.Context) ([]string, error) {
	ctxAdmin := context.WithValue(ctx, powc.AdminKey, i.powAdminToken)
	ms, err := i.pow.Admin.Indices.GetMiners(ctxAdmin, powAdmin.WithPowerGreaterThanZero(true))
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
