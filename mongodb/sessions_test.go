package mongodb_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	. "github.com/textileio/textile/v2/mongodb"
)

func TestSessions_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewSessions(context.Background(), db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(owner))
	require.NoError(t, err)
	assert.True(t, created.ExpiresAt.After(time.Now()))
}

func TestSessions_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewSessions(context.Background(), db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(owner))
	require.NoError(t, err)

	got, err := col.Get(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestSessions_Touch(t *testing.T) {
	db := newDB(t)
	col, err := NewSessions(context.Background(), db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(owner))
	require.NoError(t, err)

	time.Sleep(time.Second)
	err = col.Touch(context.Background(), created.ID)
	require.NoError(t, err)
	got, err := col.Get(context.Background(), created.ID)
	require.NoError(t, err)
	assert.True(t, got.ExpiresAt.After(created.ExpiresAt))
}

func TestSessions_Delete(t *testing.T) {
	db := newDB(t)
	col, err := NewSessions(context.Background(), db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(owner))
	require.NoError(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.NoError(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.Error(t, err)
}

func TestSessions_DeleteByOwner(t *testing.T) {
	db := newDB(t)
	col, err := NewSessions(context.Background(), db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(owner))
	require.NoError(t, err)

	err = col.DeleteByOwner(context.Background(), created.Owner)
	require.NoError(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.Error(t, err)
}
