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

func TestOrgs_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)

	_, mem, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), "test", []Member{{
		Key:      mem,
		Username: "test",
		Role:     OrgOwner,
	}})
	require.Nil(t, err)
	assert.Equal(t, created.Name, "test")
	assert.NotNil(t, created.Key)
	assert.True(t, created.CreatedAt.Unix() > 0)

	_, err = col.Create(context.Background(), "test", []Member{{
		Key:      mem,
		Username: "test",
		Role:     OrgOwner,
	}})
	require.NotNil(t, err)

	_, err = col.Create(context.Background(), "empty", []Member{})
	require.NotNil(t, err)
}

func TestOrgs_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)

	_, mem, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), "test", []Member{{
		Key:      mem,
		Username: "test",
		Role:     OrgOwner,
	}})
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Name)
	require.Nil(t, err)
	assert.Equal(t, created.Key, got.Key)
}

func TestOrgs_SetToken(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)

	_, mem, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), "test", []Member{{
		Key:      mem,
		Username: "test",
		Role:     OrgOwner,
	}})
	require.Nil(t, err)

	iss, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	tok, err := thread.NewToken(iss, thread.NewLibp2pPubKey(created.Key))
	require.Nil(t, err)
	err = col.SetToken(context.Background(), created.Name, tok)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Name)
	require.Nil(t, err)
	assert.NotEmpty(t, got.Token)
}

func TestOrgs_List(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)

	_, mem, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), "test", []Member{{
		Key:      mem,
		Username: "test",
		Role:     OrgOwner,
	}})
	require.Nil(t, err)

	list, err := col.List(context.Background(), mem)
	require.Nil(t, err)
	require.Equal(t, 1, len(list))
	assert.Equal(t, created.Name, list[0].Name)
}

func TestOrgs_IsOwner(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)

	_, mem1, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), "test", []Member{{
		Key:      mem1,
		Username: "test",
		Role:     OrgOwner,
	}})
	require.Nil(t, err)

	_, mem2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	err = col.AddMember(context.Background(), created.Name, Member{
		Key:      mem2,
		Username: "member",
		Role:     OrgMember,
	})
	require.Nil(t, err)

	is, err := col.IsOwner(context.Background(), created.Name, mem1)
	require.Nil(t, err)
	assert.True(t, is)
	is, err = col.IsOwner(context.Background(), created.Name, mem2)
	require.Nil(t, err)
	assert.False(t, is)
}

func TestOrgs_IsMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)

	_, mem1, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), "test", []Member{{
		Key:      mem1,
		Username: "test",
		Role:     OrgOwner,
	}})
	require.Nil(t, err)

	_, mem2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	err = col.AddMember(context.Background(), created.Name, Member{
		Key:      mem2,
		Username: "member",
		Role:     OrgMember,
	})
	require.Nil(t, err)

	is, err := col.IsMember(context.Background(), created.Name, mem2)
	require.Nil(t, err)
	assert.True(t, is)
	err = col.RemoveMember(context.Background(), created.Name, mem2)
	require.Nil(t, err)
	is, err = col.IsMember(context.Background(), created.Name, mem2)
	require.Nil(t, err)
	assert.False(t, is)
}

func TestOrgs_AddMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)

	_, mem1, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), "test", []Member{{
		Key:      mem1,
		Username: "test",
		Role:     OrgOwner,
	}})
	require.Nil(t, err)

	_, mem2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	err = col.AddMember(context.Background(), created.Name, Member{
		Key:      mem2,
		Username: "member",
		Role:     OrgMember,
	})
	require.Nil(t, err)
	err = col.AddMember(context.Background(), created.Name, Member{ // Add again should not duplicate entry
		Key:      mem2,
		Username: "member",
		Role:     OrgMember,
	})
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Name)
	require.Nil(t, err)
	assert.Equal(t, 2, len(got.Members))
}

func TestOrgs_RemoveMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)

	_, mem1, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), "test", []Member{{
		Key:      mem1,
		Username: "test",
		Role:     OrgOwner,
	}})
	require.Nil(t, err)

	err = col.RemoveMember(context.Background(), created.Name, mem1)
	require.NotNil(t, err) // Can't remove the sole owner

	_, mem2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	err = col.AddMember(context.Background(), created.Name, Member{
		Key:      mem2,
		Username: "member",
		Role:     OrgMember,
	})
	require.Nil(t, err)

	err = col.RemoveMember(context.Background(), created.Name, mem2)
	require.Nil(t, err)
	list, err := col.List(context.Background(), mem2)
	require.Nil(t, err)
	require.Equal(t, 0, len(list))
}

func TestOrgs_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)

	_, mem, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), "test", []Member{{
		Key:      mem,
		Username: "test",
		Role:     OrgOwner,
	}})
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.Name)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.Name)
	require.NotNil(t, err)
}
