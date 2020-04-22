package client

type options struct {
	progress chan<- int64
}

type Option func(*options)

// WithProgress writes progress updates to the given channel.
func WithProgress(ch chan<- int64) Option {
	return func(args *options) {
		args.progress = ch
	}
}
