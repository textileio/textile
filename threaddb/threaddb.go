package threaddb

import (
	"context"
	"strings"

	dbc "github.com/textileio/go-threads/api/client"
	coredb "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
)

// Collection wraps a ThreadDB collection with some convenience methods.
type Collection struct {
	c      *dbc.Client
	config db.CollectionConfig
}

// Options defines options for interacting with a collection.
type Options struct {
	Token thread.Token
}

// Option holds a collection option.
type Option func(*Options)

// WithToken sets the token.
func WithToken(t thread.Token) Option {
	return func(args *Options) {
		args.Token = t
	}
}

// Create a collection instance.
func (c *Collection) Create(ctx context.Context, dbID thread.ID, instance interface{}, opts ...Option) (coredb.InstanceID, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	ids, err := c.c.Create(ctx, dbID, c.config.Name, dbc.Instances{instance}, db.WithTxnToken(args.Token))
	if isColNotFoundErr(err) {
		if err := c.addCollection(ctx, dbID, args.Token); err != nil {
			return coredb.EmptyInstanceID, err
		}
		return c.Create(ctx, dbID, instance, opts...)
	}
	if isInvalidSchemaErr(err) {
		if err := c.updateCollection(ctx, dbID, args.Token); err != nil {
			return coredb.EmptyInstanceID, err
		}
		return c.Create(ctx, dbID, instance, opts...)
	}
	if err != nil {
		return coredb.EmptyInstanceID, err
	}
	return coredb.InstanceID(ids[0]), nil
}

// Get a collection instance.
func (c *Collection) Get(ctx context.Context, dbID thread.ID, key string, instance interface{}, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	err := c.c.FindByID(ctx, dbID, c.config.Name, key, instance, db.WithTxnToken(args.Token))
	if isColNotFoundErr(err) {
		if err := c.addCollection(ctx, dbID, args.Token); err != nil {
			return err
		}
		return c.Get(ctx, dbID, key, instance, opts...)
	}
	return err
}

// List collection instances.
func (c *Collection) List(ctx context.Context, dbID thread.ID, query *db.Query, instance interface{}, opts ...Option) (interface{}, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	res, err := c.c.Find(ctx, dbID, c.config.Name, query, instance, db.WithTxnToken(args.Token))
	if isColNotFoundErr(err) {
		if err := c.addCollection(ctx, dbID, args.Token); err != nil {
			return nil, err
		}
		return c.List(ctx, dbID, query, instance, opts...)
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Save a collection instance.
func (c *Collection) Save(ctx context.Context, dbID thread.ID, instance interface{}, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	err := c.c.Save(ctx, dbID, c.config.Name, dbc.Instances{instance}, db.WithTxnToken(args.Token))
	if isInvalidSchemaErr(err) {
		if err := c.updateCollection(ctx, dbID, args.Token); err != nil {
			return err
		}
		return c.Save(ctx, dbID, instance, opts...)
	}
	return err
}

// Delete a collection instance.
func (c *Collection) Delete(ctx context.Context, dbID thread.ID, id string, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	err := c.c.Delete(ctx, dbID, c.config.Name, []string{id}, db.WithTxnToken(args.Token))
	if err != nil {
		return err
	}
	return nil
}

func (c *Collection) addCollection(ctx context.Context, dbID thread.ID, token thread.Token) error {
	return c.c.NewCollection(ctx, dbID, c.config, db.WithManagedToken(token))
}

func (c *Collection) updateCollection(ctx context.Context, dbID thread.ID, token thread.Token) error {
	return c.c.UpdateCollection(ctx, dbID, c.config, db.WithManagedToken(token))
}

func isColNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "collection not found")
}

func isInvalidSchemaErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "instance doesn't correspond to schema: (root)")
}
