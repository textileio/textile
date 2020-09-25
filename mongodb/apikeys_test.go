package mongodb_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	. "github.com/textileio/textile/v2/mongodb"
)

func TestAPIKeys_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewAPIKeys(context.Background(), db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(owner), AccountKey, true)
	require.NoError(t, err)
	assert.NotEmpty(t, created.Secret)
	assert.Equal(t, AccountKey, created.Type)
	assert.True(t, created.Secure)
}

func TestAPIKeys_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewAPIKeys(context.Background(), db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(owner), UserKey, false)
	require.NoError(t, err)

	got, err := col.Get(context.Background(), created.Key)
	require.NoError(t, err)
	assert.Equal(t, created.Key, got.Key)
}

func TestAPIKeys_ListByOwner(t *testing.T) {
	db := newDB(t)
	col, err := NewAPIKeys(context.Background(), db)
	require.NoError(t, err)

	_, owner1, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	_, err = col.Create(context.Background(), thread.NewLibp2pPubKey(owner1), UserKey, false)
	require.NoError(t, err)
	_, err = col.Create(context.Background(), thread.NewLibp2pPubKey(owner1), UserKey, false)
	require.NoError(t, err)

	list1, err := col.ListByOwner(context.Background(), thread.NewLibp2pPubKey(owner1))
	require.NoError(t, err)
	assert.Equal(t, 2, len(list1))

	_, owner2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	list2, err := col.ListByOwner(context.Background(), thread.NewLibp2pPubKey(owner2))
	require.NoError(t, err)
	assert.Equal(t, 0, len(list2))
}

func TestAPIKeys_Invalidate(t *testing.T) {
	db := newDB(t)
	col, err := NewAPIKeys(context.Background(), db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(owner), UserKey, false)
	require.NoError(t, err)

	err = col.Invalidate(context.Background(), created.Key)
	require.NoError(t, err)
	got, err := col.Get(context.Background(), created.Key)
	require.NoError(t, err)
	require.False(t, got.Valid)
}

func TestAPIKeys_DeleteByOwner(t *testing.T) {
	db := newDB(t)
	col, err := NewAPIKeys(context.Background(), db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(owner), UserKey, false)
	require.NoError(t, err)

	err = col.DeleteByOwner(context.Background(), thread.NewLibp2pPubKey(owner))
	require.NoError(t, err)
	_, err = col.Get(context.Background(), created.Key)
	require.Error(t, err)
}
