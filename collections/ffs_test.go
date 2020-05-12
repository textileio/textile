package collections_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
)

func TestFFSInstances_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewFFSInstances(context.Background(), db)
	require.NoError(t, err)

	err = col.Create(context.Background(), "buckkey1", "ffstoken1")
	require.NoError(t, err)
}

func TestFFSInstances_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewFFSInstances(context.Background(), db)
	require.NoError(t, err)

	err = col.Create(context.Background(), "buckkey1", "ffstoken1")
	require.NoError(t, err)

	got, err := col.Get(context.Background(), "buckkey1")
	require.NoError(t, err)
	require.Equal(t, "buckkey1", got.BucketKey)
	require.Equal(t, "ffstoken1", got.FFSToken)
}

func TestFFSInstances_Replace(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	col, err := NewFFSInstances(context.Background(), db)
	require.NoError(t, err)

	err = col.Create(context.Background(), "buckkey1", "ffstoken1")
	require.NoError(t, err)

	ffs, err := col.Get(context.Background(), "buckkey1")
	require.NoError(t, err)

	ffs.Archives.Current = Archive{
		Cid:       "Cid1",
		JobID:     "JobID1",
		JobStatus: 123,
		CreatedAt: time.Now().Unix(),
	}
	ffs.Archives.History = []Archive{
		Archive{
			Cid:       "Cid2",
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
