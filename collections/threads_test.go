package collections_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/api/common"
	. "github.com/textileio/textile/collections"
	"github.com/textileio/textile/util"
)

func TestThreads_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.Nil(t, err)

	id := thread.NewIDV1(thread.Raw, 32)
	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(ctx, id, owner)
	require.Nil(t, err)
	assert.True(t, created.ID.Defined())

	_, err = col.Create(ctx, id, owner)
	require.NotNil(t, err)

	_, err = col.Create(common.NewThreadNameContext(ctx, "db1"), thread.NewIDV1(thread.Raw, 32), owner)
	require.Nil(t, err)
	_, err = col.Create(common.NewThreadNameContext(ctx, "db1"), thread.NewIDV1(thread.Raw, 32), owner)
	require.NotNil(t, err)
}

func TestThreads_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(ctx, thread.NewIDV1(thread.Raw, 32), owner)
	require.Nil(t, err)

	got, err := col.Get(ctx, created.ID, owner)
	require.Nil(t, err)
	assert.Equal(t, created.Owner, got.Owner)
	assert.Equal(t, created.ID, got.ID)
}

func TestThreads_GetByName(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(common.NewThreadNameContext(ctx, "db1"), thread.NewIDV1(thread.Raw, 32), owner)
	require.Nil(t, err)

	got, err := col.GetByName(ctx, "db1", owner)
	require.Nil(t, err)
	assert.Equal(t, created.Owner, got.Owner)
	assert.Equal(t, created.ID, got.ID)
}

func TestThreads_List(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.Nil(t, err)

	_, owner1, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, err = col.Create(ctx, thread.NewIDV1(thread.Raw, 32), owner1)
	require.Nil(t, err)
	_, err = col.Create(ctx, thread.NewIDV1(thread.Raw, 32), owner1)
	require.Nil(t, err)

	list1, err := col.List(ctx, owner1)
	require.Nil(t, err)
	assert.Equal(t, 2, len(list1))

	_, owner2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	list2, err := col.List(ctx, owner2)
	require.Nil(t, err)
	assert.Equal(t, 0, len(list2))
}

func TestThreads_ListByKey(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.Nil(t, err)

	key := util.MakeToken(12)
	_, owner1, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, err = col.Create(common.NewAPIKeyContext(ctx, key), thread.NewIDV1(thread.Raw, 32), owner1)
	require.Nil(t, err)
	_, owner2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	_, err = col.Create(common.NewAPIKeyContext(ctx, key), thread.NewIDV1(thread.Raw, 32), owner2)
	require.Nil(t, err)

	list1, err := col.ListByKey(ctx, key)
	require.Nil(t, err)
	assert.Equal(t, 2, len(list1))

	_, owner3, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	list2, err := col.List(ctx, owner3)
	require.Nil(t, err)
	assert.Equal(t, 0, len(list2))
}

func TestThreads_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	ctx := context.Background()
	col, err := NewThreads(ctx, db)
	require.Nil(t, err)

	_, owner, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(ctx, thread.NewIDV1(thread.Raw, 32), owner)
	require.Nil(t, err)

	err = col.Delete(ctx, created.ID, owner)
	require.Nil(t, err)
	_, err = col.Get(ctx, created.ID, owner)
	require.NotNil(t, err)
}
