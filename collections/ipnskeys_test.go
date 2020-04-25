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

	threadID := thread.NewIDV1(thread.Raw, 32)
	created, err := col.Create(context.Background(), threadID)
	require.Nil(t, err)
	assert.NotEmpty(t, created.Key)
	assert.NotEmpty(t, created.CreatedAt)
}

func TestIPNSKeys_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.Nil(t, err)

	threadID := thread.NewIDV1(thread.Raw, 32)
	created, err := col.Create(context.Background(), threadID)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.ID)
	require.Nil(t, err)
	assert.Equal(t, created.Key, got.Key)
	assert.Equal(t, created.ThreadID, got.ThreadID)
}

func TestIPNSKeys_Delete(t *testing.T) {
	db := newDB(t)
	col, err := NewIPNSKeys(context.Background(), db)
	require.Nil(t, err)

	threadID := thread.NewIDV1(thread.Raw, 32)
	created, err := col.Create(context.Background(), threadID)
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.NotNil(t, err)
}
