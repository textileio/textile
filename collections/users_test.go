package collections_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
)

func TestUsers_GetOrCreate(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.Nil(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	err = col.Create(context.Background(), key, nil)
	require.Nil(t, err)
	err = col.Create(context.Background(), key, nil)
	require.Nil(t, err)
}

func TestUsers_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.Nil(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	err = col.Create(context.Background(), key, &FFSInfo{ID: "id", Token: "token"})
	require.Nil(t, err)

	got, err := col.Get(context.Background(), key)
	require.Nil(t, err)
	assert.Equal(t, key, got.Key)
	assert.Equal(t, "id", got.FFSInfo.ID)
	assert.Equal(t, "token", got.FFSInfo.Token)
}

func TestUsers_Delete(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.Nil(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	err = col.Create(context.Background(), key, nil)
	require.Nil(t, err)

	err = col.Delete(context.Background(), key)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), key)
	require.NotNil(t, err)
}

func TestUsers_BucketsTotalSize(t *testing.T) {
	db := newDB(t)
	col, err := NewUsers(context.Background(), db)
	require.Nil(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	err = col.Create(context.Background(), key, nil)
	require.Nil(t, err)

	err = col.SetBucketsTotalSize(context.Background(), key, 1234)
	require.NoError(t, err)

	got, err := col.Get(context.Background(), key)
	require.Nil(t, err)
	assert.Equal(t, int64(1234), got.BucketsTotalSize)
}
