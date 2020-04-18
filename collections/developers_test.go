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

func TestDevelopers_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	created, err := col.Create(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)
	require.Equal(t, "jon", created.Username)
	require.Equal(t, "jon@doe.com", created.Email)
	require.NotEmpty(t, created.Key)
	require.NotEmpty(t, created.Secret)

	_, err = col.Create(context.Background(), "jon", "jon2@doe.com")
	require.NotNil(t, err)
	_, err = col.Create(context.Background(), "jon2", "jon@doe.com")
	require.NotNil(t, err)
}

func TestDevelopers_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	created, err := col.Create(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Key)
	require.Nil(t, err)
	assert.Equal(t, created.Key, got.Key)
}

func TestDevelopers_GetByUsernameOrEmail(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	created, err := col.Create(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)

	got, err := col.GetByUsernameOrEmail(context.Background(), "jon")
	require.Nil(t, err)
	assert.Equal(t, created.Key, got.Key)

	got, err = col.GetByUsernameOrEmail(context.Background(), "jon@doe.com")
	require.Nil(t, err)
	assert.Equal(t, created.Key, got.Key)

	_, err = col.GetByUsernameOrEmail(context.Background(), "jon2")
	require.NotNil(t, err)
}

func TestDevelopers_CheckUsername(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	ok, err := col.CheckUsername(context.Background(), "jon")
	require.Nil(t, err)
	require.True(t, ok)

	_, err = col.Create(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)

	ok, err = col.CheckUsername(context.Background(), "jon")
	require.Nil(t, err)
	require.False(t, ok)
}

func TestDevelopers_SetToken(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewDevelopers(context.Background(), db)
	require.Nil(t, err)

	created, err := col.Create(context.Background(), "jon", "jon@doe.com")
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

	one, err := col.Create(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)
	two, err := col.Create(context.Background(), "jane", "jane@doe.com")
	require.Nil(t, err)
	_, err = col.Create(context.Background(), "jone", "jone@doe.com")
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

	created, err := col.Create(context.Background(), "jon", "jon@doe.com")
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.Key)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.Key)
	require.NotNil(t, err)
}
