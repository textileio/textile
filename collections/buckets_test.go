package collections_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBuckets_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewBuckets(context.Background(), db)
	require.Nil(t, err)

	ownerID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), ownerID, "test", uuid.New().String(), "test")
	require.Nil(t, err)
	assert.Equal(t, created.Name, "test")
	_, err = col.Create(context.Background(), ownerID, "test", uuid.New().String(), "test")
	require.NotNil(t, err)
}

func TestBuckets_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewBuckets(context.Background(), db)
	require.Nil(t, err)
	ownerID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), ownerID, "test", uuid.New().String(), "test")
	require.Nil(t, err)

	got, err := col.Get(context.Background(), ownerID, created.Name)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestBuckets_List(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewBuckets(context.Background(), db)
	require.Nil(t, err)
	ownerID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), ownerID, "test", uuid.New().String(), "test")
	require.Nil(t, err)

	list, err := col.List(context.Background(), ownerID)
	require.Nil(t, err)
	require.Equal(t, len(list), 1)
	assert.Equal(t, list[0].Name, created.Name)
}

func TestBuckets_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewBuckets(context.Background(), db)
	require.Nil(t, err)
	ownerID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), ownerID, "test", uuid.New().String(), "test")
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), ownerID, created.Name)
	require.NotNil(t, err)
}
