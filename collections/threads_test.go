package collections_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	. "github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestThreads_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewThreads(context.Background(), db)
	require.Nil(t, err)

	threadID := thread.NewIDV1(thread.Raw, 32)
	ownerID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), threadID, ownerID, primitive.NewObjectID())
	require.Nil(t, err)
	assert.True(t, created.ThreadID.Defined())

	_, err = col.Create(context.Background(), threadID, ownerID, primitive.NewObjectID())
	require.NotNil(t, err)
}

func TestThreads_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewThreads(context.Background(), db)
	require.Nil(t, err)
	ownerID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), thread.NewIDV1(thread.Raw, 32), ownerID, primitive.NewObjectID())
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.ThreadID, ownerID)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestThreads_GetPrimary(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewThreads(context.Background(), db)
	require.Nil(t, err)
	ownerID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), thread.NewIDV1(thread.Raw, 32), ownerID, primitive.NewObjectID())
	require.Nil(t, err)

	got, err := col.GetPrimary(context.Background(), ownerID)
	require.Nil(t, err)
	require.True(t, got.Primary)
	require.Equal(t, created.ID, got.ID)
}

func TestThreads_SetPrimary(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewThreads(context.Background(), db)
	require.Nil(t, err)
	ownerID := primitive.NewObjectID()
	created1, err := col.Create(context.Background(), thread.NewIDV1(thread.Raw, 32), ownerID, primitive.NewObjectID())
	require.Nil(t, err)
	require.True(t, created1.Primary)

	created2, err := col.Create(context.Background(), thread.NewIDV1(thread.Raw, 32), ownerID, primitive.NewObjectID())
	require.Nil(t, err)

	err = col.SetPrimary(context.Background(), created2.ThreadID, ownerID)
	require.Nil(t, err)
	got, err := col.Get(context.Background(), created2.ThreadID, ownerID)
	require.Nil(t, err)
	require.True(t, got.Primary)

	got, err = col.Get(context.Background(), created1.ThreadID, ownerID)
	require.Nil(t, err)
	require.False(t, got.Primary)
}

func TestThreads_List(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewThreads(context.Background(), db)
	require.Nil(t, err)
	ownerID := primitive.NewObjectID()
	_, err = col.Create(context.Background(), thread.NewIDV1(thread.Raw, 32), ownerID, primitive.NewObjectID())
	require.Nil(t, err)
	_, err = col.Create(context.Background(), thread.NewIDV1(thread.Raw, 32), ownerID, primitive.NewObjectID())
	require.Nil(t, err)

	list1, err := col.List(context.Background(), ownerID)
	require.Nil(t, err)
	assert.Equal(t, len(list1), 2)

	list2, err := col.List(context.Background(), primitive.NewObjectID())
	require.Nil(t, err)
	assert.Equal(t, len(list2), 0)
}

func TestThreads_ListByKey(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewThreads(context.Background(), db)
	require.Nil(t, err)
	keyID := primitive.NewObjectID()
	_, err = col.Create(context.Background(), thread.NewIDV1(thread.Raw, 32), primitive.NewObjectID(), keyID)
	require.Nil(t, err)
	_, err = col.Create(context.Background(), thread.NewIDV1(thread.Raw, 32), primitive.NewObjectID(), keyID)
	require.Nil(t, err)

	list1, err := col.List(context.Background(), keyID)
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
	ownerID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), thread.NewIDV1(thread.Raw, 32), ownerID, primitive.NewObjectID())
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ThreadID, ownerID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.ThreadID, ownerID)
	require.NotNil(t, err)
}
