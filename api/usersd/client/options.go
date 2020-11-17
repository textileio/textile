package client

type listOptions struct {
	seek      string
	limit     int
	ascending bool
	status    Status
}

type ListOption func(*listOptions)

// WithSeek starts listing from the given ID.
func WithSeek(id string) ListOption {
	return func(args *listOptions) {
		args.seek = id
	}
}

// WithLimit limits the number of list messages results.
func WithLimit(limit int) ListOption {
	return func(args *listOptions) {
		args.limit = limit
	}
}

// WithAscending lists messages by ascending order.
func WithAscending(asc bool) ListOption {
	return func(args *listOptions) {
		args.ascending = asc
	}
}

// Status indicates message read status.
type Status int

const (
	// All includes read and unread messages.
	All Status = iota
	// Read is only read messages.
	Read
	// Unread is only unread messages.
	Unread
)

// WithStatus filters messages by read status.
// Note: Only applies to inbox messages.
func WithStatus(s Status) ListOption {
	return func(args *listOptions) {
		args.status = s
	}
}

type usageOptions struct {
	key string
}

type UsageOption func(*usageOptions)

// WithPubKey returns usage info for the public key.
func WithPubKey(key string) UsageOption {
	return func(args *usageOptions) {
		args.key = key
	}
}
