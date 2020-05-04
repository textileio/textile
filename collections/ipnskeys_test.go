package collections_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	. "github.com/textileio/textile/collections"
)

func TestIPNSKeys_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.Nil(t, err)

	err = col.Create(context.Background(), "foo", "cid", thread.NewIDV1(thread.Raw, 32))
	require.Nil(t, err)
}

func TestIPNSKeys_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.Nil(t, err)

	threadID := thread.NewIDV1(thread.Raw, 32)
	err = col.Create(context.Background(), "foo", "cid", threadID)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), "foo")
	require.Nil(t, err)
	assert.Equal(t, "cid", got.Cid)
	assert.Equal(t, threadID, got.ThreadID)
}

func TestIPNSKeys_GetByCid(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.Nil(t, err)

	threadID := thread.NewIDV1(thread.Raw, 32)
	err = col.Create(context.Background(), "foo", "cid", threadID)
	require.Nil(t, err)

	got, err := col.GetByCid(context.Background(), "cid")
	require.Nil(t, err)
	assert.Equal(t, "foo", got.Name)
	assert.Equal(t, threadID, got.ThreadID)
}

func TestIPNSKeys_ListByThreadID(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.Nil(t, err)

	threadID := thread.NewIDV1(thread.Raw, 32)
	err = col.Create(context.Background(), "foo1", "cid1", threadID)
	require.Nil(t, err)
	err = col.Create(context.Background(), "foo2", "cid2", threadID)
	require.Nil(t, err)

	list1, err := col.ListByThreadID(context.Background(), threadID)
	require.Nil(t, err)
	assert.Equal(t, 2, len(list1))

	list2, err := col.ListByThreadID(context.Background(), thread.NewIDV1(thread.Raw, 32))
	require.Nil(t, err)
	assert.Equal(t, 0, len(list2))
}

func TestIPNSKeys_Delete(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.Nil(t, err)

	err = col.Create(context.Background(), "foo", "cid", thread.NewIDV1(thread.Raw, 32))
	require.Nil(t, err)

	err = col.Delete(context.Background(), "foo")
	require.Nil(t, err)
	_, err = col.Get(context.Background(), "foo")
	require.NotNil(t, err)
}
