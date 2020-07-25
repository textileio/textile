package client

type listOptions struct {
	seek   string
	limit  int
	status Status
}

type ListOption func(*listOptions)

// WithSeek starts listing from the given ID.
func WithSeek(id string) ListOption {
	return func(args *listOptions) {
		args.seek = id
	}
}

// WithLimit limits the number of list messages results.
func WithLimit(l int) ListOption {
	return func(args *listOptions) {
		args.limit = l
	}
}

// WithStatus filters messages by read status.
// Note: Only applies to inbox messages.
func WithStatus(s Status) ListOption {
	return func(args *listOptions) {
		args.status = s
	}
}
