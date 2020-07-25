package threaddb

import (
	"context"

	"github.com/alecthomas/jsonschema"
	dbc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	db "github.com/textileio/go-threads/db"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/messages"
)

var (
	messagesSchema  *jsonschema.Schema
	messagesIndexes = []db.Index{{
		Path: "from",
	}, {
		Path: "to",
	}, {
		Path: "created_at",
	}, {
		Path: "read_at",
	}}
	messagesConfig db.CollectionConfig
)

// Message represents the messages threaddb collection schema.
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
	messagesSchema = reflector.Reflect(&Message{})
	messagesConfig = db.CollectionConfig{
		Name:    messages.CollectionName,
		Schema:  messagesSchema,
		Indexes: messagesIndexes,
	}
}

// Messages is a wrapper around a threaddb collection for sending messages between users.
type Messages struct {
	collection
}

// NewMessages returns a new messages collection mananger.
func NewMessages(tc *dbc.Client) (*Messages, error) {
	return &Messages{
		collection: collection{
			c:      tc,
			config: messagesConfig,
		},
	}, nil
}

// NewMailbox creates a new threaddb message box.
func (m *Messages) NewMailbox(ctx context.Context, name string, opts ...Option) (thread.ID, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	id := thread.NewIDV1(thread.Raw, 32)
	ctx = common.NewThreadNameContext(ctx, name)
	err := m.c.NewDB(ctx, id, db.WithNewManagedName(name), db.WithNewManagedCollections(messagesConfig), db.WithNewManagedToken(args.Token))
	return id, err
}
