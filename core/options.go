package core

type Options struct {
	ThreadsBadgerRepoPath string
	ThreadsMongoUri       string
	ThreadsMongoDB        string
}

type Option func(*Options) error

func WithBadgerThreadsPersistence(repoPath string) Option {
	return func(o *Options) error {
		o.ThreadsBadgerRepoPath = repoPath
		return nil
	}
}

func WithMongoThreadsPersistence(uri, db string) Option {
	return func(o *Options) error {
		o.ThreadsMongoUri = uri
		o.ThreadsMongoDB = db
		return nil
	}
}
