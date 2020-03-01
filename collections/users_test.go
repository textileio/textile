package collections_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
)

func TestUsers_GetOrCreate(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewUsers(context.Background(), db)
	require.Nil(t, err)

	deviceID := uuid.New().String()
	created, err := col.GetOrCreate(context.Background(), deviceID)
	require.Nil(t, err)
	got, err := col.GetOrCreate(context.Background(), deviceID)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestUsers_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewUsers(context.Background(), db)
	require.Nil(t, err)
	deviceID := uuid.New().String()
	created, err := col.GetOrCreate(context.Background(), deviceID)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), deviceID)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestUsers_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewUsers(context.Background(), db)
	require.Nil(t, err)
	deviceID := uuid.New().String()
	created, err := col.GetOrCreate(context.Background(), deviceID)
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), deviceID)
	require.NotNil(t, err)
}
