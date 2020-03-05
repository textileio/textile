package collections_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestOrgs_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)

	ownerID := primitive.NewObjectID()
	created := &Org{
		Name: "test",
		Members: []Member{{
			ID:       ownerID,
			Username: "test",
			Role:     OrgOwner,
		}},
	}
	err = col.Create(context.Background(), created)
	require.Nil(t, err)
	assert.Equal(t, created.Name, "test")
	assert.NotNil(t, created.ID)
	assert.True(t, created.CreatedAt.Unix() > 0)
	err = col.Create(context.Background(), &Org{Name: "test"})
	require.NotNil(t, err)
}

func TestOrgs_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	created := &Org{Name: "test"}
	err = col.Create(context.Background(), created)
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.Name)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestOrgs_List(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	ownerID := primitive.NewObjectID()
	created := &Org{
		Name: "test",
		Members: []Member{{
			ID:       ownerID,
			Username: "test",
			Role:     OrgOwner,
		}},
	}
	err = col.Create(context.Background(), created)
	require.Nil(t, err)

	list, err := col.List(context.Background(), ownerID)
	require.Nil(t, err)
	require.Equal(t, len(list), 1)
	assert.Equal(t, list[0].Name, created.Name)
}

func TestOrgs_IsOwner(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	ownerID := primitive.NewObjectID()
	created := &Org{
		Name: "test",
		Members: []Member{{
			ID:       ownerID,
			Username: "test",
			Role:     OrgOwner,
		}},
	}
	err = col.Create(context.Background(), created)
	require.Nil(t, err)
	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.Name, Member{
		ID:       memberID,
		Username: "member",
		Role:     OrgMember,
	})
	require.Nil(t, err)

	is, err := col.IsOwner(context.Background(), created.Name, ownerID)
	require.Nil(t, err)
	assert.True(t, is)
	is, err = col.IsOwner(context.Background(), created.Name, memberID)
	require.Nil(t, err)
	assert.False(t, is)
}

func TestOrgs_IsMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	created := &Org{
		Name: "test",
		Members: []Member{{
			ID:       primitive.NewObjectID(),
			Username: "test",
			Role:     OrgOwner,
		}},
	}
	err = col.Create(context.Background(), created)
	require.Nil(t, err)
	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.Name, Member{
		ID:       memberID,
		Username: "member",
		Role:     OrgMember,
	})
	require.Nil(t, err)

	is, err := col.IsMember(context.Background(), created.Name, memberID)
	require.Nil(t, err)
	assert.True(t, is)
	err = col.RemoveMember(context.Background(), created.Name, memberID)
	require.Nil(t, err)
	is, err = col.IsMember(context.Background(), created.Name, memberID)
	require.Nil(t, err)
	assert.False(t, is)
}

func TestOrgs_AddMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	created := &Org{Name: "test"}
	err = col.Create(context.Background(), created)
	require.Nil(t, err)

	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.Name, Member{
		ID:       memberID,
		Username: "member",
		Role:     OrgMember,
	})
	require.Nil(t, err)
	err = col.AddMember(context.Background(), created.Name, Member{ // Add again should not duplicate entry
		ID:       memberID,
		Username: "member",
		Role:     OrgMember,
	})
	require.Nil(t, err)
	got, err := col.Get(context.Background(), created.Name)
	require.Nil(t, err)
	assert.Equal(t, len(got.Members), 1)
}

func TestOrgs_RemoveMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	created := &Org{Name: "test"}
	err = col.Create(context.Background(), created)
	require.Nil(t, err)
	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.Name, Member{
		ID:       memberID,
		Username: "member",
		Role:     OrgMember,
	})
	require.Nil(t, err)

	err = col.RemoveMember(context.Background(), created.Name, memberID)
	require.Nil(t, err)
	list, err := col.List(context.Background(), memberID)
	require.Nil(t, err)
	require.Equal(t, len(list), 0)
}

func TestOrgs_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	created := &Org{Name: "test"}
	err = col.Create(context.Background(), created)
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.Name)
	require.NotNil(t, err)
}
