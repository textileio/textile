package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/textile/v2/api/mindexd/model"
)

func TestPutFilecoinInfo(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)

	s, err := New(db)
	require.NoError(t, err)

	f0100 := model.FilecoinInfo{
		RelativePower:    0.00002,
		AskPrice:         "1000",
		AskVerifiedPrice: "501",
		MaxPieceSize:     32000,
		MinPieceSize:     12000,
		SectorSize:       11111,
	}
	f0101 := model.FilecoinInfo{
		RelativePower:    100.00002,
		AskPrice:         "1001000",
		AskVerifiedPrice: "100501",
		MaxPieceSize:     10032000,
		MinPieceSize:     10012000,
		SectorSize:       10011111,
	}

	err = s.PutFilecoinInfo(ctx, "f0100", f0100)
	require.NoError(t, err)
	err = s.PutFilecoinInfo(ctx, "f0101", f0101)
	require.NoError(t, err)

	mif0100, err := s.GetMinerInfo(ctx, "f0100")
	require.NoError(t, err)
	require.Equal(t, f0100.RelativePower, mif0100.Filecoin.RelativePower)
	require.Equal(t, f0100.AskPrice, mif0100.Filecoin.AskPrice)
	require.Equal(t, f0100.AskVerifiedPrice, mif0100.Filecoin.AskVerifiedPrice)
	require.Equal(t, f0100.MaxPieceSize, mif0100.Filecoin.MaxPieceSize)
	require.Equal(t, f0100.MinPieceSize, mif0100.Filecoin.MinPieceSize)
	require.Equal(t, f0100.SectorSize, mif0100.Filecoin.SectorSize)
	require.False(t, mif0100.Filecoin.UpdatedAt.IsZero())

	mif0101, err := s.GetMinerInfo(ctx, "f0101")
	require.NoError(t, err)
	require.Equal(t, f0101.RelativePower, mif0101.Filecoin.RelativePower)
	require.Equal(t, f0101.AskPrice, mif0101.Filecoin.AskPrice)
	require.Equal(t, f0101.AskVerifiedPrice, mif0101.Filecoin.AskVerifiedPrice)
	require.Equal(t, f0101.MaxPieceSize, mif0101.Filecoin.MaxPieceSize)
	require.Equal(t, f0101.MinPieceSize, mif0101.Filecoin.MinPieceSize)
	require.Equal(t, f0101.SectorSize, mif0101.Filecoin.SectorSize)
	require.False(t, mif0101.Filecoin.UpdatedAt.IsZero())
}
