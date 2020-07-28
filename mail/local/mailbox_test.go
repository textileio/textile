package local_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMailbox_Identity(t *testing.T) {
	mail, key, secret := setup(t)

	conf := getConf(t, key, secret)
	box, err := mail.NewMailbox(context.Background(), conf)
	require.NoError(t, err)
	assert.Equal(t, conf.Identity.GetPublic(), box.Identity().GetPublic())
}

func TestMailbox_SendMessage(t *testing.T) {
	mail, key, secret := setup(t)

	conf1 := getConf(t, key, secret)
	box1, err := mail.NewMailbox(context.Background(), conf1)
	require.NoError(t, err)
	conf2 := getConf(t, key, secret)
	box2, err := mail.NewMailbox(context.Background(), conf2)
	require.NoError(t, err)

	m1, err := box1.SendMessage(context.Background(), box2.Identity().GetPublic(), []byte("howdy"))
	require.NoError(t, err)
	assert.NotEmpty(t, m1.ID)
	assert.Equal(t, box1.Identity().GetPublic(), m1.From)
	assert.Equal(t, box2.Identity().GetPublic(), m1.To)
	ok, err := box1.Identity().GetPublic().Verify(m1.Body, m1.Signature)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, false, m1.Read())
	assert.NotEmpty(t, m1.CreatedAt)

	b1, err := m1.Open(context.Background(), box1.Identity())
	require.NoError(t, err)
	assert.Equal(t, "howdy", string(b1))

	list2, err := box2.ListInboxMessages(context.Background())
	require.NoError(t, err)
	assert.Len(t, list2, 1)
	m2 := list2[0]
	b2, err := m2.Open(context.Background(), box2.Identity())
	require.NoError(t, err)
	assert.Equal(t, "howdy", string(b2))
}
