package mongodb_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/mongodb"
)

func TestFFSInstances_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewFFSInstances(context.Background(), db)
	require.NoError(t, err)

	err = col.Create(context.Background(), "buckkey1", "ffstoken1", "waddr1")
	require.NoError(t, err)
}

func TestFFSInstances_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewFFSInstances(context.Background(), db)
	require.NoError(t, err)

	err = col.Create(context.Background(), "buckkey1", "ffstoken1", "waddr1")
	require.NoError(t, err)

	got, err := col.Get(context.Background(), "buckkey1")
	require.NoError(t, err)
	require.Equal(t, "buckkey1", got.BucketKey)
	require.Equal(t, "ffstoken1", got.FFSToken)
	require.Equal(t, "waddr1", got.WalletAddr)
}

func TestFFSInstances_Replace(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	col, err := NewFFSInstances(context.Background(), db)
	require.NoError(t, err)

	err = col.Create(context.Background(), "buckkey1", "ffstoken1", "waddr1")
	require.NoError(t, err)

	ffs, err := col.Get(context.Background(), "buckkey1")
	require.NoError(t, err)

	c1, _ := cid.Decode("QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D")
	c2, _ := cid.Decode("QmU7gJi6Bz3jrvbuVfB7zzXStLJrTHf6vWh8ZqkCsTGoRC")
	ffs.Archives.Current = Archive{
		Cid:       c1.Bytes(),
		JobID:     "JobID1",
		JobStatus: 123,
		CreatedAt: time.Now().Unix(),
	}
	ffs.Archives.History = []Archive{
		{
			Cid:       c2.Bytes(),
			JobID:     "JobID2",
			JobStatus: 456,
			CreatedAt: time.Now().Add(time.Hour * -24).Unix(),
		},
	}
	err = col.Replace(ctx, ffs)
	require.NoError(t, err)

	ffs2, err := col.Get(context.Background(), "buckkey1")
	require.NoError(t, err)
	require.Equal(t, ffs, ffs2)
}
