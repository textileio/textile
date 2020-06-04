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

func TestAPIKeys_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewAPIKeys(context.Background(), db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), owner, AccountKey, true)
	require.Nil(t, err)
	assert.NotEmpty(t, created.Secret)
	assert.Equal(t, AccountKey, created.Type)
	assert.True(t, created.Secure)
}

func TestAPIKeys_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewAPIKeys(context.Background(), db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), owner, UserKey, false)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Key)
	require.Nil(t, err)
	assert.Equal(t, created.Key, got.Key)
}

func TestAPIKeys_ListByOwner(t *testing.T) {
	db := newDB(t)
	col, err := NewAPIKeys(context.Background(), db)
	require.Nil(t, err)

	_, owner1, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, err = col.Create(context.Background(), owner1, UserKey, false)
	require.Nil(t, err)
	_, err = col.Create(context.Background(), owner1, UserKey, false)
	require.Nil(t, err)

	list1, err := col.ListByOwner(context.Background(), owner1)
	require.Nil(t, err)
	assert.Equal(t, 2, len(list1))

	_, owner2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	list2, err := col.ListByOwner(context.Background(), owner2)
	require.Nil(t, err)
	assert.Equal(t, 0, len(list2))
}

func TestAPIKeys_Invalidate(t *testing.T) {
	db := newDB(t)
	col, err := NewAPIKeys(context.Background(), db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), owner, UserKey, false)
	require.Nil(t, err)

	err = col.Invalidate(context.Background(), created.Key)
	require.Nil(t, err)
	got, err := col.Get(context.Background(), created.Key)
	require.Nil(t, err)
	require.False(t, got.Valid)
}

func TestAPIKeys_DeleteByOwner(t *testing.T) {
	db := newDB(t)
	col, err := NewAPIKeys(context.Background(), db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), owner, UserKey, false)
	require.Nil(t, err)

	err = col.DeleteByOwner(context.Background(), owner)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.Key)
	require.NotNil(t, err)
}
