package mongodb_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/mongodb"
)

func TestUsers_GetOrCreate(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.NoError(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.Create(context.Background(), key, nil)
	require.NoError(t, err)
	err = col.Create(context.Background(), key, nil)
	require.NoError(t, err)
}

func TestUsers_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.NoError(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.Create(context.Background(), key, &FFSInfo{ID: "id", Token: "token"})
	require.NoError(t, err)

	got, err := col.Get(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, key, got.Key)
	assert.Equal(t, "id", got.FFSInfo.ID)
	assert.Equal(t, "token", got.FFSInfo.Token)
}

func TestUsers_UpdateFFSInfo(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.NoError(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.Create(context.Background(), key, &FFSInfo{ID: "id", Token: "token"})
	require.NoError(t, err)

	got, err := col.Get(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, key, got.Key)
	assert.Equal(t, "id", got.FFSInfo.ID)
	assert.Equal(t, "token", got.FFSInfo.Token)

	updated, err := col.UpdateFFSInfo(context.Background(), key, &FFSInfo{ID: "id2", Token: "token2"})
	require.NoError(t, err)
	assert.Equal(t, key, updated.Key)
	assert.Equal(t, "id2", updated.FFSInfo.ID)
	assert.Equal(t, "token2", updated.FFSInfo.Token)

	got, err = col.Get(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, key, got.Key)
	assert.Equal(t, "id2", got.FFSInfo.ID)
	assert.Equal(t, "token2", got.FFSInfo.Token)
}

func TestUsers_Delete(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.NoError(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.Create(context.Background(), key, nil)
	require.NoError(t, err)

	err = col.Delete(context.Background(), key)
	require.NoError(t, err)
	_, err = col.Get(context.Background(), key)
	require.Error(t, err)
}

func TestUsers_BucketsTotalSize(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.NoError(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.Create(context.Background(), key, nil)
	require.NoError(t, err)

	err = col.SetBucketsTotalSize(context.Background(), key, 1234)
	require.NoError(t, err)

	got, err := col.Get(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, int64(1234), got.BucketsTotalSize)
}
