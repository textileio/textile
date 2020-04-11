package collections_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestKeys_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewKeys(context.Background(), db)
	require.Nil(t, err)

	created, err := col.Create(context.Background(), primitive.NewObjectID())
	require.Nil(t, err)
	assert.NotEmpty(t, created.Token)
	assert.NotEmpty(t, created.Secret)
}

func TestKeys_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewKeys(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID())
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Token)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestKeys_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewKeys(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID())
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.Token)
	require.NotNil(t, err)
}
