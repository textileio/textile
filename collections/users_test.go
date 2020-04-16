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
	t.Parallel()
	db := newDB(t)

	col, err := NewUsers(context.Background(), db)
	require.Nil(t, err)

	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.GetOrCreate(context.Background(), key)
	require.Nil(t, err)
	got, err := col.GetOrCreate(context.Background(), key)
	require.Nil(t, err)
	assert.Equal(t, created.Key, got.Key)
}

func TestUsers_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewUsers(context.Background(), db)
	require.Nil(t, err)
	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.GetOrCreate(context.Background(), key)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), key)
	require.Nil(t, err)
	assert.Equal(t, created.Key, got.Key)
}

func TestUsers_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewUsers(context.Background(), db)
	require.Nil(t, err)
	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.GetOrCreate(context.Background(), key)
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.Key)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.Key)
	require.NotNil(t, err)
}
