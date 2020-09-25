package mongodb_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/v2/api/common"
	. "github.com/textileio/textile/v2/mongodb"
	"github.com/textileio/textile/v2/util"
)

func TestThreads_Create(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.NoError(t, err)

	id := thread.NewIDV1(thread.Raw, 32)
	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created1, err := col.Create(ctx, id, thread.NewLibp2pPubKey(owner), false)
	require.NoError(t, err)
	assert.True(t, created1.ID.Defined())
	assert.False(t, created1.IsDB)

	_, err = col.Create(ctx, id, thread.NewLibp2pPubKey(owner), false)
	require.Error(t, err)

	_, err = col.Create(common.NewThreadNameContext(ctx, "db1"), thread.NewIDV1(thread.Raw, 32), thread.NewLibp2pPubKey(owner), true)
	require.NoError(t, err)
	_, err = col.Create(common.NewThreadNameContext(ctx, "db1"), thread.NewIDV1(thread.Raw, 32), thread.NewLibp2pPubKey(owner), true)
	require.Error(t, err)
}

func TestThreads_Get(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(ctx, thread.NewIDV1(thread.Raw, 32), thread.NewLibp2pPubKey(owner), true)
	require.NoError(t, err)

	got, err := col.Get(ctx, created.ID, thread.NewLibp2pPubKey(owner))
	require.NoError(t, err)
	assert.Equal(t, created.Owner, got.Owner)
	assert.Equal(t, created.ID, got.ID)
	assert.True(t, created.IsDB)
}

func TestThreads_GetByName(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(common.NewThreadNameContext(ctx, "db1"), thread.NewIDV1(thread.Raw, 32), thread.NewLibp2pPubKey(owner), true)
	require.NoError(t, err)

	got, err := col.GetByName(ctx, "db1", thread.NewLibp2pPubKey(owner))
	require.NoError(t, err)
	assert.Equal(t, created.Owner, got.Owner)
	assert.Equal(t, created.ID, got.ID)
}

func TestThreads_ListByOwner(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.NoError(t, err)

	_, owner1, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	_, err = col.Create(ctx, thread.NewIDV1(thread.Raw, 32), thread.NewLibp2pPubKey(owner1), true)
	require.NoError(t, err)
	_, err = col.Create(ctx, thread.NewIDV1(thread.Raw, 32), thread.NewLibp2pPubKey(owner1), true)
	require.NoError(t, err)

	list1, err := col.ListByOwner(ctx, thread.NewLibp2pPubKey(owner1))
	require.NoError(t, err)
	assert.Equal(t, 2, len(list1))

	_, owner2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	list2, err := col.ListByOwner(ctx, thread.NewLibp2pPubKey(owner2))
	require.NoError(t, err)
	assert.Equal(t, 0, len(list2))
}

func TestThreads_ListByKey(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.NoError(t, err)

	key := util.MakeToken(12)
	_, owner1, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	_, err = col.Create(common.NewAPIKeyContext(ctx, key), thread.NewIDV1(thread.Raw, 32), thread.NewLibp2pPubKey(owner1), true)
	require.NoError(t, err)
	_, owner2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	_, err = col.Create(common.NewAPIKeyContext(ctx, key), thread.NewIDV1(thread.Raw, 32), thread.NewLibp2pPubKey(owner2), true)
	require.NoError(t, err)

	list1, err := col.ListByKey(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, 2, len(list1))

	_, owner3, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	list2, err := col.ListByOwner(ctx, thread.NewLibp2pPubKey(owner3))
	require.NoError(t, err)
	assert.Equal(t, 0, len(list2))
}

func TestThreads_Delete(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(ctx, thread.NewIDV1(thread.Raw, 32), thread.NewLibp2pPubKey(owner), true)
	require.NoError(t, err)

	err = col.Delete(ctx, created.ID, thread.NewLibp2pPubKey(owner))
	require.NoError(t, err)
	_, err = col.Get(ctx, created.ID, thread.NewLibp2pPubKey(owner))
	require.Error(t, err)
}

func TestThreads_DeleteByOwner(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.NoError(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	created, err := col.Create(ctx, thread.NewIDV1(thread.Raw, 32), thread.NewLibp2pPubKey(owner), true)
	require.NoError(t, err)

	err = col.DeleteByOwner(ctx, thread.NewLibp2pPubKey(owner))
	require.NoError(t, err)
	_, err = col.Get(ctx, created.ID, thread.NewLibp2pPubKey(owner))
	require.Error(t, err)
}
