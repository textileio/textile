package collections_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestTeams_Create(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewTeams(context.Background(), db)
	require.Nil(t, err)

	t.Run("test create", func(t *testing.T) {
		ownerID := primitive.NewObjectID()
		created, err := col.Create(context.Background(), ownerID, "test")
		require.Nil(t, err)
		assert.Equal(t, created.Name, "test")
		_, err = col.Create(context.Background(), ownerID, "test")
		require.NotNil(t, err)
	})
}

func TestTeams_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewTeams(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test")
	require.Nil(t, err)

	t.Run("test get", func(t *testing.T) {
		got, err := col.Get(context.Background(), created.ID)
		require.Nil(t, err)
		assert.Equal(t, created.ID, got.ID)
	})
}

func TestTeams_List(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewTeams(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test")
	require.Nil(t, err)
	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.ID, memberID)
	require.Nil(t, err)

	t.Run("test list", func(t *testing.T) {
		list, err := col.List(context.Background(), memberID)
		require.Nil(t, err)
		require.Equal(t, len(list), 1)
		assert.Equal(t, list[0].Name, "test")
	})
}

func TestTeams_HasMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewTeams(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test")
	require.Nil(t, err)
	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.ID, memberID)
	require.Nil(t, err)

	t.Run("test has member", func(t *testing.T) {
		has, err := col.HasMember(context.Background(), created.ID, memberID)
		require.Nil(t, err)
		assert.True(t, has)
		err = col.RemoveMember(context.Background(), created.ID, memberID)
		require.Nil(t, err)
		has, err = col.HasMember(context.Background(), created.ID, memberID)
		require.Nil(t, err)
		assert.False(t, has)
	})
}

func TestTeams_AddMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewTeams(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test")
	require.Nil(t, err)

	t.Run("test add member", func(t *testing.T) {
		memberID := primitive.NewObjectID()
		err = col.AddMember(context.Background(), created.ID, memberID)
		require.Nil(t, err)
		err = col.AddMember(context.Background(), created.ID, memberID) // Add again should fail
		require.NotNil(t, err)
	})
}

func TestTeams_RemoveMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewTeams(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test")
	require.Nil(t, err)
	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.ID, memberID)
	require.Nil(t, err)

	t.Run("test remove member", func(t *testing.T) {
		err = col.RemoveMember(context.Background(), created.ID, memberID)
		require.Nil(t, err)
		list, err := col.List(context.Background(), memberID)
		require.Nil(t, err)
		require.Equal(t, len(list), 0)
		err = col.AddMember(context.Background(), created.ID, memberID) // Add back should succeed
		require.Nil(t, err)
	})
}

func TestTeams_Delete(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewTeams(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test")
	require.Nil(t, err)

	t.Run("test delete", func(t *testing.T) {
		err := col.Delete(context.Background(), created.ID)
		require.Nil(t, err)
		_, err = col.Get(context.Background(), created.ID)
		require.NotNil(t, err)
	})
}
