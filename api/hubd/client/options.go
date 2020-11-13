package client

type listOptions struct {
	offset int64
	limit  int64
}

type ListOption func(*listOptions)

// WithOffset is used to fetch the next page when paginating.
func WithOffset(offset int64) ListOption {
	return func(args *listOptions) {
		args.offset = offset
	}
}

// WithLimit is used to set a page size when paginating.
func WithLimit(limit int64) ListOption {
	return func(args *listOptions) {
		args.limit = limit
	}
}
