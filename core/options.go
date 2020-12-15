package core

type Options struct {
	ThreadsBadgerRepoPath string
	ThreadsMongoUri       string
	ThreadsMongoDB        string
}

type Option func(*Options)

func WithBadgerThreadsPersistence(repoPath string) Option {
	return func(o *Options) {
		o.ThreadsBadgerRepoPath = repoPath
	}
}

func WithMongoThreadsPersistence(uri, db string) Option {
	return func(o *Options) {
		o.ThreadsMongoUri = uri
		o.ThreadsMongoDB = db
	}
}
