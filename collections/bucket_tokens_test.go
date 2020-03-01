package collections_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBucketTokens_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewBucketTokens(context.Background(), db)
	require.Nil(t, err)

	buckID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), buckID)
	require.Nil(t, err)
	assert.Equal(t, created.BucketID, buckID)
}

func TestBucketTokens_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewBucketTokens(context.Background(), db)
	require.Nil(t, err)
	buckID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), buckID)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Token)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestBucketTokens_List(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewBucketTokens(context.Background(), db)
	require.Nil(t, err)
	buckID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), buckID)
	require.Nil(t, err)

	list, err := col.List(context.Background(), buckID)
	require.Nil(t, err)
	require.Equal(t, len(list), 1)
	assert.Equal(t, list[0].Token, created.Token)
}

func TestBucketTokens_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewBucketTokens(context.Background(), db)
	require.Nil(t, err)
	buckID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), buckID)
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.Token)
	require.NotNil(t, err)
}
