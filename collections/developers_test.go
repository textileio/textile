package collections_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
)

func TestDevelopers_GetOrCreate(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	created, err := col.GetOrCreate(context.Background(), "jon@doe.com", uuid.New().String())
	require.Nil(t, err)
	got, err := col.GetOrCreate(context.Background(), "jon@doe.com", uuid.New().String())
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestDevelopers_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)
	created, err := col.GetOrCreate(context.Background(), "jon@doe.com", uuid.New().String())
	require.Nil(t, err)

	got, err := col.Get(context.Background(), "jon@doe.com")
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestDevelopers_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)
	created, err := col.GetOrCreate(context.Background(), "jon@doe.com", uuid.New().String())
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), "jon@doe.com")
	require.NotNil(t, err)
}
