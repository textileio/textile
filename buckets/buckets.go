package buckets

import (
	"errors"
	"fmt"
)

const (
	// CollectionName is the name of the threaddb collection used for buckets.
	CollectionName = "buckets"
	// SeedName is the file name reserved for a random bucket seed.
	SeedName = ".textileseed"
)

// Role describes an access role for a bucket item.
type Role int

const (
	None Role = iota
	Reader
	Writer
	Admin
)

var (
	// ErrNonFastForward is returned when an update in non-fast-forward.
	ErrNonFastForward = fmt.Errorf("update is non-fast-forward")

	// ErrNoCurrentArchive is returned when not status about the last archive
	// can be retrieved, since the bucket was never archived.
	ErrNoCurrentArchive = fmt.Errorf("the bucket was never archived")

	// ErrZeroBalance is returned when archiving a bucket which
	// underlying Account/User FFS instance balance is zero.
	ErrZeroBalance = errors.New("powergate wallet FIL balance is zero, if recently created wait 30s")
)
