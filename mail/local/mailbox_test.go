package local_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/textile/v2/api/usersd/client"
	. "github.com/textileio/textile/v2/mail/local"
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
	assert.Equal(t, false, m1.IsRead())
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

func TestMailbox_Watch(t *testing.T) {
	mail, key, secret := setup(t)

	conf1 := getConf(t, key, secret)
	box1, err := mail.NewMailbox(context.Background(), conf1)
	require.NoError(t, err)
	conf2 := getConf(t, key, secret)
	box2, err := mail.NewMailbox(context.Background(), conf2)
	require.NoError(t, err)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events1 := make(chan MailboxEvent)
	defer close(events1)
	ec1 := &eventCollector{}
	go ec1.collect(events1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		state, err := box1.WatchInbox(ctx, events1, true)
		require.NoError(t, err)
		for s := range state {
			fmt.Println(fmt.Sprintf("received inbox watch state: %s", s.State))
		}
	}()

	events2 := make(chan MailboxEvent)
	defer close(events2)
	ec2 := &eventCollector{}
	go ec2.collect(events2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		state, err := box2.WatchSentbox(ctx, events2, true)
		require.NoError(t, err)
		for s := range state {
			fmt.Println(fmt.Sprintf("received sentbox watch state: %s", s.State))
		}
	}()

	time.Sleep(time.Second) // Ensure the listener is ready
	sent := make([]client.Message, 3)
	for i := range sent {
		sent[i], err = box2.SendMessage(context.Background(), box1.Identity().GetPublic(), []byte("hi"))
		require.NoError(t, err)
	}
	err = box1.ReadInboxMessage(context.Background(), sent[0].ID)
	require.NoError(t, err)
	err = box1.DeleteInboxMessage(context.Background(), sent[1].ID)
	require.NoError(t, err)
	err = box2.DeleteSentboxMessage(context.Background(), sent[2].ID)
	require.NoError(t, err)

	cancel() // Stop listening
	wg.Wait()
	ec1.check(t, len(sent), 1, 1)
	ec2.check(t, len(sent), 0, 1)
}

type eventCollector struct {
	new     int
	read    int
	deleted int
	sync.Mutex
}

func (c *eventCollector) collect(events chan MailboxEvent) {
	for e := range events {
		c.Lock()
		switch e.Type {
		case NewMessage:
			c.new++
		case MessageRead:
			c.read++
		case MessageDeleted:
			c.deleted++
		}
		c.Unlock()
	}
}

func (c *eventCollector) check(t *testing.T, numNew, numRead, numDeleted int) {
	time.Sleep(5 * time.Second)
	c.Lock()
	defer c.Unlock()
	assert.Equal(t, numNew, c.new)
	assert.Equal(t, numRead, c.read)
	assert.Equal(t, numDeleted, c.deleted)
}
