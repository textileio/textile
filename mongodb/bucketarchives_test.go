package mongodb_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/v2/mongodb"
)

func TestBucketArchives_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewBucketArchives(context.Background(), db)
	require.NoError(t, err)

	res, err := col.Create(context.Background(), "buckkey1")
	require.NoError(t, err)
	require.Equal(t, "buckkey1", res.BucketKey)
}

func TestBucketArchives_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewBucketArchives(context.Background(), db)
	require.NoError(t, err)

	res, err := col.Create(context.Background(), "buckkey1")
	require.NoError(t, err)
	require.Equal(t, "buckkey1", res.BucketKey)

	got, err := col.GetOrCreate(context.Background(), "buckkey1")
	require.NoError(t, err)
	require.Equal(t, "buckkey1", got.BucketKey)
}

func TestBucketArchives_Replace(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	col, err := NewBucketArchives(context.Background(), db)
	require.NoError(t, err)

	res, err := col.Create(context.Background(), "buckkey1")
	require.NoError(t, err)
	require.Equal(t, "buckkey1", res.BucketKey)

	ba, err := col.GetOrCreate(context.Background(), "buckkey1")
	require.NoError(t, err)

	c1, _ := cid.Decode("QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D")
	c2, _ := cid.Decode("QmU7gJi6Bz3jrvbuVfB7zzXStLJrTHf6vWh8ZqkCsTGoRC")
	ba.Archives.Current = Archive{
		Cid:       c1.Bytes(),
		JobID:     "JobID1",
		Status:    123,
		CreatedAt: time.Now().Unix(),
	}
	ba.Archives.History = []Archive{
		{
			Cid:       c2.Bytes(),
			JobID:     "JobID2",
			Status:    456,
			CreatedAt: time.Now().Add(time.Hour * -24).Unix(),
		},
	}
	err = col.Replace(ctx, ba)
	require.NoError(t, err)

	ba2, err := col.GetOrCreate(context.Background(), "buckkey1")
	require.NoError(t, err)
	require.Equal(t, ba, ba2)
}
