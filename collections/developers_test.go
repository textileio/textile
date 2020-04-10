package collections_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, created.ID, got.ID)
}

func TestDevelopers_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)
	created, err := col.GetOrCreate(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.ID)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestDevelopers_SetDBToken(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)
	created, err := col.GetOrCreate(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)

	iss, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	tok, err := thread.NewToken(iss, thread.NewLibp2pPubKey(created.ThreadIdentity.GetPublic()))
	require.Nil(t, err)
	err = col.SetDBToken(context.Background(), created.ID, tok)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.ID)
	require.Nil(t, err)
	assert.NotEmpty(t, got.ThreadToken)
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

	list, err := col.ListMembers(context.Background(), []Member{{ID: one.ID}, {ID: two.ID}})
	require.Nil(t, err)
	assert.Equal(t, len(list), 2)
}

func TestDevelopers_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)
	created, err := col.GetOrCreate(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.NotNil(t, err)
}
