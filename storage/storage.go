package storage

import (
	"context"
	"io"

	fc "github.com/textileio/filecoin/api/client"
)

// Storage provides the storage API
type Storage struct {
	fcClient *fc.Client
}

type config struct {
	fcClient *fc.Client
}

// Option configures Storage
type Option func(*config) error

// FcClient sets the address to connect to for the filecoin servive
func FcClient(client *fc.Client) Option {
	return func(c *config) error {
		c.fcClient = client
		return nil
	}
}

type filecoinStoreConfig struct {
	address  string
	duration int64
}

type storeConfig struct {
	fcStoreConfig *filecoinStoreConfig
}

// StoreOption configures the Store method behavior
type StoreOption func(*storeConfig) error

// StoreToFilecoin specifies that data should be stored in the filecoin network
// ToDo: maybe don't need to pass in address here because it can come from the project context
func StoreToFilecoin(address string, duration int64) StoreOption {
	return func(c *storeConfig) error {
		c.fcStoreConfig = &filecoinStoreConfig{
			address:  address,
			duration: duration,
		}
		return nil
	}
}

// NewStorage creates a storage manager
func NewStorage(opts ...Option) (*Storage, error) {
	conf := &config{}

	for _, opt := range opts {
		opt(conf)
	}

	s := &Storage{
		fcClient: conf.fcClient,
	}

	return s, nil
}

// Store stores a blob of data from a Reader
func (s *Storage) Store(ctx context.Context, data io.Reader, opts ...StoreOption) {
	conf := &storeConfig{}
	for _, opt := range opts {
		opt(conf)
	}

	if conf.fcStoreConfig != nil {
		// store in filecoin
	}

	// handle other storage options/methods/destinations
}
