package collections_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestThreads_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewThreads(context.Background(), db)
	require.Nil(t, err)

	created, err := col.Create(context.Background(), primitive.NewObjectID(), primitive.NewObjectID())
	require.Nil(t, err)
	assert.True(t, created.ID.Defined())
}

func TestThreads_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewThreads(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), primitive.NewObjectID())
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.ID)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestThreads_List(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewThreads(context.Background(), db)
	require.Nil(t, err)
	ownerID := primitive.NewObjectID()
	_, err = col.Create(context.Background(), ownerID, primitive.NewObjectID())
	require.Nil(t, err)
	_, err = col.Create(context.Background(), ownerID, primitive.NewObjectID())
	require.Nil(t, err)

	list1, err := col.List(context.Background(), ownerID)
	require.Nil(t, err)
	assert.Equal(t, len(list1), 2)

	list2, err := col.List(context.Background(), primitive.NewObjectID())
	require.Nil(t, err)
	assert.Equal(t, len(list2), 0)
}

func TestThreads_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewThreads(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), primitive.NewObjectID())
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.NotNil(t, err)
}
