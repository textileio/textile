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

func TestSessions_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewSessions(context.Background(), db)
	require.Nil(t, err)

	created, err := col.Create(context.Background(), primitive.NewObjectID())
	require.Nil(t, err)
	assert.True(t, created.ExpiresAt.After(time.Now()))
}

func TestSessions_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewSessions(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID())
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Token)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestSessions_Touch(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewSessions(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID())
	require.Nil(t, err)

	err = col.Touch(context.Background(), created.Token)
	require.Nil(t, err)
	got, err := col.Get(context.Background(), created.Token)
	require.Nil(t, err)
	assert.True(t, got.ExpiresAt.After(created.ExpiresAt))
}

func TestSessions_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewSessions(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID())
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.Token)
	require.NotNil(t, err)
}
