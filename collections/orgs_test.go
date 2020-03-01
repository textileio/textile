package collections_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
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
	created, err := col.Create(context.Background(), ownerID, "test", uuid.New().String())
	require.Nil(t, err)
	assert.Equal(t, created.Name, "test")
	_, err = col.Create(context.Background(), ownerID, "test", uuid.New().String())
	require.NotNil(t, err)
}

func TestOrgs_Get(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test", uuid.New().String())
	require.Nil(t, err)

	got, err := col.Get(context.Background(), created.ID)
	require.Nil(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestOrgs_List(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test", uuid.New().String())
	require.Nil(t, err)
	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.ID, memberID)
	require.Nil(t, err)

	list, err := col.List(context.Background(), memberID)
	require.Nil(t, err)
	require.Equal(t, len(list), 1)
	assert.Equal(t, list[0].Name, created.Name)
}

func TestOrgs_HasMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test", uuid.New().String())
	require.Nil(t, err)
	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.ID, memberID)
	require.Nil(t, err)

	has, err := col.HasMember(context.Background(), created.ID, memberID)
	require.Nil(t, err)
	assert.True(t, has)
	err = col.RemoveMember(context.Background(), created.ID, memberID)
	require.Nil(t, err)
	has, err = col.HasMember(context.Background(), created.ID, memberID)
	require.Nil(t, err)
	assert.False(t, has)
}

func TestOrgs_AddMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test", uuid.New().String())
	require.Nil(t, err)

	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.ID, memberID)
	require.Nil(t, err)
	err = col.AddMember(context.Background(), created.ID, memberID) // Add again should not duplicate entry
	require.Nil(t, err)
	got, err := col.Get(context.Background(), created.ID)
	require.Nil(t, err)
	assert.Equal(t, len(got.MemberIDs), 1)
}

func TestOrgs_RemoveMember(t *testing.T) {
	t.Parallel()
	db := newDB(t)

	col, err := NewOrgs(context.Background(), db)
	require.Nil(t, err)
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test", uuid.New().String())
	require.Nil(t, err)
	memberID := primitive.NewObjectID()
	err = col.AddMember(context.Background(), created.ID, memberID)
	require.Nil(t, err)

	err = col.RemoveMember(context.Background(), created.ID, memberID)
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
	created, err := col.Create(context.Background(), primitive.NewObjectID(), "test", uuid.New().String())
	require.Nil(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.Nil(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.NotNil(t, err)
}
