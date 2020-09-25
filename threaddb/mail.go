package threaddb

import (
	"context"
	"errors"
	"strings"

	"github.com/alecthomas/jsonschema"
	dbc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	db "github.com/textileio/go-threads/db"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/mail"
	mdb "github.com/textileio/textile/v2/mongodb"
)

var (
	inboxSchema  *jsonschema.Schema
	inboxIndexes = []db.Index{{
		Path: "from",
	}, {
		Path: "to",
	}, {
		Path: "created_at",
	}, {
		Path: "read_at",
	}}
	inboxConfig    db.CollectionConfig
	sentboxSchema  *jsonschema.Schema
	sentboxIndexes = []db.Index{{
		Path: "from",
	}, {
		Path: "to",
	}, {
		Path: "created_at",
	}}
	sentboxConfig db.CollectionConfig

	// ErrMailboxExists indicates that a mailbox with the same name and owner already exists.
	ErrMailboxExists = errors.New("mailbox already exists")
)

// InboxMessage represents the inbox threaddb collection schema.
type InboxMessage struct {
	ID        string `json:"_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Body      string `json:"body"`
	Signature string `json:"signature"`
	CreatedAt int64  `json:"created_at"`
	ReadAt    int64  `json:"read_at"`
}

// SentboxMessage represents the sentbox threaddb collection schema.
type SentboxMessage struct {
	ID        string `json:"_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Body      string `json:"body"`
	Signature string `json:"signature"`
	CreatedAt int64  `json:"created_at"`
}

func init() {
	reflector := jsonschema.Reflector{ExpandedStruct: true}
	inboxSchema = reflector.Reflect(&InboxMessage{})
	inboxConfig = db.CollectionConfig{
		Name:    mail.InboxCollectionName,
		Schema:  inboxSchema,
		Indexes: inboxIndexes,
	}
	sentboxSchema = reflector.Reflect(&SentboxMessage{})
	sentboxConfig = db.CollectionConfig{
		Name:    mail.SentboxCollectionName,
		Schema:  sentboxSchema,
		Indexes: sentboxIndexes,
	}
}

// Mail is a wrapper around a threaddb collection for sending mail between users.
type Mail struct {
	c       *dbc.Client
	Inbox   Collection
	Sentbox Collection
}

// NewMail returns a new mail collection mananger.
func NewMail(tc *dbc.Client) (*Mail, error) {
	return &Mail{
		c: tc,
		Inbox: Collection{
			c:      tc,
			config: inboxConfig,
		},
		Sentbox: Collection{
			c:      tc,
			config: sentboxConfig,
		},
	}, nil
}

// NewMailbox creates a new threaddb mail box.
func (m *Mail) NewMailbox(ctx context.Context, opts ...Option) (thread.ID, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	id := thread.NewIDV1(thread.Raw, 32)
	ctx = common.NewThreadNameContext(ctx, mail.ThreadName)
	err := m.c.NewDB(
		ctx,
		id,
		db.WithNewManagedName(mail.ThreadName),
		db.WithNewManagedCollections(inboxConfig, sentboxConfig),
		db.WithNewManagedToken(args.Token))
	if err != nil && strings.Contains(err.Error(), mdb.DuplicateErrMsg) {
		return id, ErrMailboxExists
	}
	return id, err
}
