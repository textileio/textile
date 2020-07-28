package threaddb

import (
	"context"

	"github.com/alecthomas/jsonschema"
	dbc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	db "github.com/textileio/go-threads/db"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/mail"
)

var (
	mailSchema  *jsonschema.Schema
	mailIndexes = []db.Index{{
		Path: "from",
	}, {
		Path: "to",
	}, {
		Path: "created_at",
	}, {
		Path: "read_at",
	}}
	mailConfig db.CollectionConfig
)

// Message represents the mail threaddb collection schema.
type Message struct {
	ID        string `json:"_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Body      string `json:"body"`
	Signature string `json:"signature"`
	CreatedAt int64  `json:"created_at"`
	ReadAt    int64  `json:"read_at"`
}

func init() {
	reflector := jsonschema.Reflector{ExpandedStruct: true}
	mailSchema = reflector.Reflect(&Message{})
	mailConfig = db.CollectionConfig{
		Name:    mail.CollectionName,
		Schema:  mailSchema,
		Indexes: mailIndexes,
	}
}

// Mail is a wrapper around a threaddb collection for sending mail between users.
type Mail struct {
	collection
}

// NewMail returns a new mail collection mananger.
func NewMail(tc *dbc.Client) (*Mail, error) {
	return &Mail{
		collection: collection{
			c:      tc,
			config: mailConfig,
		},
	}, nil
}

// NewMailbox creates a new threaddb mail box.
func (m *Mail) NewMailbox(ctx context.Context, name string, opts ...Option) (thread.ID, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	id := thread.NewIDV1(thread.Raw, 32)
	ctx = common.NewThreadNameContext(ctx, name)
	err := m.c.NewDB(ctx, id, db.WithNewManagedName(name), db.WithNewManagedCollections(mailConfig), db.WithNewManagedToken(args.Token))
	return id, err
}
