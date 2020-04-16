package collections_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
)

func TestSessions_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewSessions(context.Background(), db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), owner)
	require.Nil(t, err)
	assert.True(t, created.ExpiresAt.After(time.Now()))
}

func TestSessions_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewSessions(context.Background(), db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), owner)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.ID)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestSessions_Touch(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewSessions(context.Background(), db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), owner)
	require.Nil(t, err)

	err = col.Touch(context.Background(), created.ID)
	require.Nil(t, err)
	got, err := col.Get(context.Background(), created.ID)
	require.Nil(t, err)
	assert.True(t, got.ExpiresAt.After(created.ExpiresAt))
}

func TestSessions_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewSessions(context.Background(), db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), owner)
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.NotNil(t, err)
}
