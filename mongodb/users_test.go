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

func TestUsers_GetOrCreate(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.NoError(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.Create(context.Background(), thread.NewLibp2pPubKey(key), nil)
	require.NoError(t, err)
	err = col.Create(context.Background(), thread.NewLibp2pPubKey(key), nil)
	require.NoError(t, err)
}

func TestUsers_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.NoError(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	user := thread.NewLibp2pPubKey(key)
	err = col.Create(context.Background(), user, &PowInfo{ID: "id", Token: "token"})
	require.NoError(t, err)

	got, err := col.Get(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, user, got.Key)
	assert.Equal(t, "id", got.PowInfo.ID)
	assert.Equal(t, "token", got.PowInfo.Token)
}

func TestUsers_UpdatePowInfo(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.NoError(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	user := thread.NewLibp2pPubKey(key)
	err = col.Create(context.Background(), user, &PowInfo{ID: "id", Token: "token"})
	require.NoError(t, err)

	got, err := col.Get(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, user, got.Key)
	assert.Equal(t, "id", got.PowInfo.ID)
	assert.Equal(t, "token", got.PowInfo.Token)

	updated, err := col.UpdatePowInfo(context.Background(), user, &PowInfo{ID: "id2", Token: "token2"})
	require.NoError(t, err)
	assert.Equal(t, user, updated.Key)
	assert.Equal(t, "id2", updated.PowInfo.ID)
	assert.Equal(t, "token2", updated.PowInfo.Token)

	got, err = col.Get(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, user, got.Key)
	assert.Equal(t, "id2", got.PowInfo.ID)
	assert.Equal(t, "token2", got.PowInfo.Token)
}

func TestUsers_Delete(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.NoError(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.Create(context.Background(), thread.NewLibp2pPubKey(key), nil)
	require.NoError(t, err)

	err = col.Delete(context.Background(), thread.NewLibp2pPubKey(key))
	require.NoError(t, err)
	_, err = col.Get(context.Background(), thread.NewLibp2pPubKey(key))
	require.Error(t, err)
}

func TestUsers_BucketsTotalSize(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.NoError(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.Create(context.Background(), thread.NewLibp2pPubKey(key), nil)
	require.NoError(t, err)

	err = col.SetBucketsTotalSize(context.Background(), thread.NewLibp2pPubKey(key), 1234)
	require.NoError(t, err)

	got, err := col.Get(context.Background(), thread.NewLibp2pPubKey(key))
	require.NoError(t, err)
	assert.Equal(t, int64(1234), got.BucketsTotalSize)
}
