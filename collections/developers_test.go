package collections_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	. "github.com/textileio/textile/collections"
)

func TestDevelopers_GetOrCreate(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	created, err := col.GetOrCreate(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)
	got, err := col.GetOrCreate(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)
	assert.Equal(t, created.Key, got.Key)
}

func TestDevelopers_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	created, err := col.GetOrCreate(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Key)
	require.Nil(t, err)
	assert.Equal(t, created.Key, got.Key)
}

func TestDevelopers_SetToken(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	created, err := col.GetOrCreate(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)

	iss, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	tok, err := thread.NewToken(iss, thread.NewLibp2pPubKey(created.Key))
	require.Nil(t, err)
	err = col.SetToken(context.Background(), created.Key, tok)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Key)
	require.Nil(t, err)
	assert.NotEmpty(t, got.Token)
}

func TestDevelopers_ListMembers(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	one, err := col.GetOrCreate(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)
	two, err := col.GetOrCreate(context.Background(), "jane", "jane@doe.com")
	require.Nil(t, err)
	_, err = col.GetOrCreate(context.Background(), "jone", "jone@doe.com")
	require.Nil(t, err)

	list, err := col.ListMembers(context.Background(), []Member{{Key: one.Key}, {Key: two.Key}})
	require.Nil(t, err)
	assert.Equal(t, 2, len(list))
}

func TestDevelopers_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	created, err := col.GetOrCreate(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.Key)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.Key)
	require.NotNil(t, err)
}
