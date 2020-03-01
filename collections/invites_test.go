package collections_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestInvites_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewInvites(context.Background(), db)
	require.Nil(t, err)

	teamID := primitive.NewObjectID()
	fromID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), teamID, fromID, "jane@doe.com")
	require.Nil(t, err)
	assert.True(t, created.ExpiresAt.After(time.Now()))
}

func TestInvites_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewInvites(context.Background(), db)
	require.Nil(t, err)
	teamID := primitive.NewObjectID()
	fromID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), teamID, fromID, "jane@doe.com")
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Token)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestInvites_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewInvites(context.Background(), db)
	require.Nil(t, err)
	teamID := primitive.NewObjectID()
	fromID := primitive.NewObjectID()
	created, err := col.Create(context.Background(), teamID, fromID, "jane@doe.com")
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.Token)
	require.NotNil(t, err)
}
