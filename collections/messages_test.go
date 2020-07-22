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

func TestMessages_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewMessages(context.Background(), db)
	require.Nil(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, to, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), from, to, []byte("hi"))
	require.Nil(t, err)
	assert.NotEmpty(t, created.ID)
	assert.NotEmpty(t, created.From)
	assert.NotEmpty(t, created.To)
	assert.NotEmpty(t, created.Body)
	assert.False(t, created.Read)
	assert.NotEmpty(t, created.CreatedAt)
}

func TestMessages_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewMessages(context.Background(), db)
	require.Nil(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, to, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), from, to, []byte("hi"))
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.ID)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestMessages_ListInbox(t *testing.T) {
	db := newDB(t)
	col, err := NewMessages(context.Background(), db)
	require.Nil(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, to, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)

	list, err := col.ListInbox(context.Background(), to)
	require.Nil(t, err)
	require.Empty(t, list)

	created, err := col.Create(context.Background(), from, to, []byte("hi"))
	require.Nil(t, err)

	list, err = col.ListInbox(context.Background(), to)
	require.Nil(t, err)
	require.Equal(t, 1, len(list))
	require.Equal(t, created.ID, list[0].ID)
}

func TestMessages_ListOutbox(t *testing.T) {
	db := newDB(t)
	col, err := NewMessages(context.Background(), db)
	require.Nil(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, to, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)

	list, err := col.ListOutbox(context.Background(), from)
	require.Nil(t, err)
	require.Empty(t, list)

	created, err := col.Create(context.Background(), from, to, []byte("hi"))
	require.Nil(t, err)

	list, err = col.ListOutbox(context.Background(), from)
	require.Nil(t, err)
	require.Equal(t, 1, len(list))
	require.Equal(t, created.ID, list[0].ID)
}

func TestMessages_Read(t *testing.T) {
	db := newDB(t)
	col, err := NewMessages(context.Background(), db)
	require.Nil(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, to, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), from, to, []byte("hi"))
	require.Nil(t, err)
	assert.False(t, created.Read)

	err = col.Read(context.Background(), created.ID)
	require.Nil(t, err)
	got, err := col.Get(context.Background(), created.ID)
	require.Nil(t, err)
	assert.True(t, got.Read)
}

func TestMessages_Delete(t *testing.T) {
	db := newDB(t)
	col, err := NewMessages(context.Background(), db)
	require.Nil(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, to, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), from, to, []byte("hi"))
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.NotNil(t, err)
}

func TestMessages_DeleteInbox(t *testing.T) {
	db := newDB(t)
	col, err := NewMessages(context.Background(), db)
	require.Nil(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, to, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), from, to, []byte("hi"))
	require.Nil(t, err)

	err = col.DeleteInbox(context.Background(), created.To)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.NotNil(t, err)
}

func TestMessages_DeleteOutbox(t *testing.T) {
	db := newDB(t)
	col, err := NewMessages(context.Background(), db)
	require.Nil(t, err)

	_, from, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, to, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), from, to, []byte("hi"))
	require.Nil(t, err)

	err = col.DeleteOutbox(context.Background(), created.From)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.NotNil(t, err)
}
