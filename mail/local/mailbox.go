package local

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/api/users/client"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/mail"
)

// Mailbox is a local-first messaging library built on ThreadDB and IPFS.
type Mailbox struct {
	cwd     string
	conf    *cmd.Config
	clients *cmd.Clients
	id      thread.Identity
	token   thread.Token
}

// Identity returns the mailbox's identity.
func (m *Mailbox) Identity() thread.Identity {
	return m.id
}

// SendMessage sends the message body to a recipient.
func (m *Mailbox) SendMessage(ctx context.Context, to thread.PubKey, body []byte) (msg client.Message, err error) {
	ctx, err = m.context(ctx)
	if err != nil {
		return
	}
	return m.clients.Users.SendMessage(ctx, m.id, to, body)
}

// ListInboxMessages lists messages from the inbox.
// Use options to paginate with seek and limit,
// and filter by read status.
func (m *Mailbox) ListInboxMessages(ctx context.Context, opts ...client.ListOption) ([]client.Message, error) {
	ctx, err := m.context(ctx)
	if err != nil {
		return nil, err
	}
	return m.clients.Users.ListInboxMessages(ctx, opts...)
}

// ListSentMessages lists messages from the sentbox.
// Use options to paginate with seek and limit.
func (m *Mailbox) ListSentMessages(ctx context.Context, opts ...client.ListOption) ([]client.Message, error) {
	ctx, err := m.context(ctx)
	if err != nil {
		return nil, err
	}
	return m.clients.Users.ListSentMessages(ctx, opts...)
}

const reconnectInterval = time.Second * 5

// MailboxEvent describes an event that occurred in a mailbox.
type MailboxEvent struct {
	// Type of event.
	Type MailboxEventType
	// Message identifier.
	MessageID db.InstanceID
	// Message will contain the full message unless this is a delete event.
	Message client.Message
}

// MailboxEventType is the type of mailbox event.
type MailboxEventType int

const (
	// NewMessage indicates a mailbox has a new message.
	NewMessage MailboxEventType = iota
	// MessageDeleted indicates a message was deleted from a mailbox.
	MessageDeleted
)

// Listen for mailbox events.
// Use the WithOffline option to keep watching during network interruptions.
// Returns a channel of watch connectivity states.
// Cancel context to stop watching.
func (m *Mailbox) Listen(ctx context.Context, mevents chan<- MailboxEvent, offline bool) (<-chan cmd.WatchState, error) {
	ctx, err := m.context(ctx)
	if err != nil {
		return nil, err
	}
	if !offline {
		return m.listenWhileConnected(ctx, mevents)
	}
	return cmd.Watch(ctx, func(ctx context.Context) (<-chan cmd.WatchState, error) {
		return m.listenWhileConnected(ctx, mevents)
	}, reconnectInterval)
}

// listenWhileConnected will listen until context is canceled or an error occurs.
func (m *Mailbox) listenWhileConnected(ctx context.Context, mevents chan<- MailboxEvent) (<-chan cmd.WatchState, error) {
	inbox, _, err := m.clients.Users.SetupMail(ctx)
	if err != nil {
		return nil, err
	}

	state := make(chan cmd.WatchState)
	go func() {
		defer close(state)

		// Start listening for remote changes
		events, err := m.clients.Threads.Listen(ctx, inbox, []tc.ListenOption{{
			Type:       tc.ListenAll,
			Collection: mail.CollectionName,
		}})
		if err != nil {
			state <- cmd.WatchState{Err: err, Fatal: !cmd.IsConnectionError(err)}
			return
		}
		errs := make(chan error)
		go func() {
			for e := range events {
				if e.Err != nil {
					errs <- e.Err // events will close on error
					continue
				}
				switch e.Action.Type {
				case tc.ActionCreate:
					var msg client.Message
					if err := json.Unmarshal(e.Action.Instance, &msg); err != nil {
						errs <- err
						return
					}
					mevents <- MailboxEvent{
						Type:      NewMessage,
						MessageID: db.InstanceID(e.Action.InstanceID),
						Message:   msg,
					}
				case tc.ActionDelete:
					mevents <- MailboxEvent{
						Type:      MessageDeleted,
						MessageID: db.InstanceID(e.Action.InstanceID),
					}
				}
			}
		}()

		// If we made it here, we must be online
		state <- cmd.WatchState{State: cmd.Online}

		for {
			select {
			case err := <-errs:
				state <- cmd.WatchState{Err: err, Fatal: !cmd.IsConnectionError(err)}
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return state, nil
}

// ReadInboxMessage marks a message as read by ID.
func (m *Mailbox) ReadInboxMessage(ctx context.Context, id string) error {
	ctx, err := m.context(ctx)
	if err != nil {
		return err
	}
	return m.clients.Users.ReadInboxMessage(ctx, id)
}

// DeleteInboxMessage deletes an inbox message by ID.
func (m *Mailbox) DeleteInboxMessage(ctx context.Context, id string) error {
	ctx, err := m.context(ctx)
	if err != nil {
		return err
	}
	return m.clients.Users.DeleteInboxMessage(ctx, id)
}

// DeleteSentMessage deletes a sent message by ID.
func (m *Mailbox) DeleteSentMessage(ctx context.Context, id string) error {
	ctx, err := m.context(ctx)
	if err != nil {
		return err
	}
	return m.clients.Users.DeleteSentMessage(ctx, id)
}

// Identity returns the mailbox's user identity.
func (m *Mailbox) loadIdentity() error {
	ids := m.conf.Viper.GetString("identity")
	if ids == "" {
		return fmt.Errorf("identity not found")
	}
	idb, err := base64.StdEncoding.DecodeString(ids)
	if err != nil {
		return fmt.Errorf("loading identity: %v", err)
	}
	m.id = &thread.Libp2pIdentity{}
	if err = m.id.UnmarshalBinary(idb); err != nil {
		return fmt.Errorf("unmarshalling identity: %v", err)
	}
	return nil
}

func (m *Mailbox) context(ctx context.Context) (context.Context, error) {
	ctx = common.NewAPIKeyContext(ctx, m.conf.Viper.GetString("api_key"))
	secret := m.conf.Viper.GetString("api_secret")
	if secret != "" {
		var err error
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Hour), secret)
		if err != nil {
			return nil, err
		}
	}
	if m.token == "" {
		tok, err := m.clients.Threads.GetToken(ctx, m.id)
		if err != nil {
			return nil, err
		}
		m.token = tok
	}
	ctx = thread.NewTokenContext(ctx, m.token)
	return ctx, nil
}
