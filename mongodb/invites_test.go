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

func TestInvites_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewInvites(context.Background(), db)
	require.NoError(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(from), "myorg", "jane@doe.com")
	require.NoError(t, err)
	assert.True(t, created.ExpiresAt.After(time.Now()))
}

func TestInvites_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewInvites(context.Background(), db)
	require.NoError(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(from), "myorg", "jane@doe.com")
	require.NoError(t, err)

	got, err := col.Get(context.Background(), created.Token)
	require.NoError(t, err)
	assert.Equal(t, created.Token, got.Token)
}

func TestInvites_ListByEmail(t *testing.T) {
	db := newDB(t)
	col, err := NewInvites(context.Background(), db)
	require.NoError(t, err)

	list, err := col.ListByEmail(context.Background(), "jane@doe.com")
	require.NoError(t, err)
	require.Empty(t, list)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(from), "myorg", "jane@doe.com")
	require.NoError(t, err)

	list, err = col.ListByEmail(context.Background(), "jane@doe.com")
	require.NoError(t, err)
	require.Equal(t, 1, len(list))
	require.Equal(t, created.Token, list[0].Token)
}

func TestInvites_Accept(t *testing.T) {
	db := newDB(t)
	col, err := NewInvites(context.Background(), db)
	require.NoError(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(from), "myorg", "jane@doe.com")
	require.NoError(t, err)
	assert.False(t, created.Accepted)

	err = col.Accept(context.Background(), created.Token)
	require.NoError(t, err)
	got, err := col.Get(context.Background(), created.Token)
	require.NoError(t, err)
	assert.True(t, got.Accepted)
}

func TestInvites_Delete(t *testing.T) {
	db := newDB(t)
	col, err := NewInvites(context.Background(), db)
	require.NoError(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(from), "myorg", "jane@doe.com")
	require.NoError(t, err)

	err = col.Delete(context.Background(), created.Token)
	require.NoError(t, err)
	_, err = col.Get(context.Background(), created.Token)
	require.Error(t, err)
}

func TestInvites_DeleteByFrom(t *testing.T) {
	db := newDB(t)
	col, err := NewInvites(context.Background(), db)
	require.NoError(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(from), "myorg", "jane@doe.com")
	require.NoError(t, err)

	err = col.DeleteByFrom(context.Background(), created.From)
	require.NoError(t, err)
	_, err = col.Get(context.Background(), created.Token)
	require.Error(t, err)
}

func TestInvites_DeleteByOrg(t *testing.T) {
	db := newDB(t)
	col, err := NewInvites(context.Background(), db)
	require.NoError(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(from), "myorg", "jane@doe.com")
	require.NoError(t, err)

	err = col.DeleteByOrg(context.Background(), created.Org)
	require.NoError(t, err)
	_, err = col.Get(context.Background(), created.Token)
	require.Error(t, err)
}

func TestInvites_DeleteByFromAndOrg(t *testing.T) {
	db := newDB(t)
	col, err := NewInvites(context.Background(), db)
	require.NoError(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(context.Background(), thread.NewLibp2pPubKey(from), "myorg", "jane@doe.com")
	require.NoError(t, err)

	err = col.DeleteByFromAndOrg(context.Background(), thread.NewLibp2pPubKey(from), created.Org)
	require.NoError(t, err)
	_, err = col.Get(context.Background(), created.Token)
	require.Error(t, err)
}
