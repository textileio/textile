package mongodb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	. "github.com/textileio/textile/v2/mongodb"
)

func TestIPNSKeys_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.NoError(t, err)

	err = col.Create(context.Background(), "foo", "cid", thread.NewIDV1(thread.Raw, 32), "path")
	require.NoError(t, err)
}

func TestIPNSKeys_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.NoError(t, err)

	threadID := thread.NewIDV1(thread.Raw, 32)
	err = col.Create(context.Background(), "foo", "cid", threadID, "path")
	require.NoError(t, err)

	got, err := col.Get(context.Background(), "foo")
	require.NoError(t, err)
	assert.Equal(t, "cid", got.Cid)
	assert.Equal(t, "path", got.Path)
	assert.Equal(t, threadID, got.ThreadID)
}

func TestIPNSKeys_GetByCid(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.NoError(t, err)

	threadID := thread.NewIDV1(thread.Raw, 32)
	err = col.Create(context.Background(), "foo", "cid", threadID, "path")
	require.NoError(t, err)

	got, err := col.GetByCid(context.Background(), "cid")
	require.NoError(t, err)
	assert.Equal(t, "foo", got.Name)
	assert.Equal(t, "path", got.Path)
	assert.Equal(t, threadID, got.ThreadID)
}

func TestIPNSKeys_SetPath(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.NoError(t, err)

	threadID := thread.NewIDV1(thread.Raw, 32)
	err = col.Create(context.Background(), "foo", "cid", threadID, "path")
	require.NoError(t, err)

	err = col.SetPath(context.Background(), "path2", "foo")
	require.NoError(t, err)

	err = col.SetPath(context.Background(), "path2", "notfoo")
	require.Error(t, err)

	got, err := col.GetByCid(context.Background(), "cid")
	require.NoError(t, err)

	assert.Equal(t, "foo", got.Name)
	assert.Equal(t, "path2", got.Path)
	assert.Equal(t, threadID, got.ThreadID)
}

func TestIPNSKeys_ListByThreadID(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.NoError(t, err)

	threadID := thread.NewIDV1(thread.Raw, 32)
	err = col.Create(context.Background(), "foo1", "cid1", threadID, "path1")
	require.NoError(t, err)
	err = col.Create(context.Background(), "foo2", "cid2", threadID, "path2")
	require.NoError(t, err)

	list1, err := col.ListByThreadID(context.Background(), threadID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(list1))

	list2, err := col.ListByThreadID(context.Background(), thread.NewIDV1(thread.Raw, 32))
	require.NoError(t, err)
	assert.Equal(t, 0, len(list2))
}

func TestIPNSKeys_Delete(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.NoError(t, err)

	err = col.Create(context.Background(), "foo", "cid", thread.NewIDV1(thread.Raw, 32), "path")
	require.NoError(t, err)

	err = col.Delete(context.Background(), "foo")
	require.NoError(t, err)
	_, err = col.Get(context.Background(), "foo")
	require.Error(t, err)
}
