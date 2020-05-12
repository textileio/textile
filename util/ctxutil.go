package util

import (
	"context"
	"time"
)

// NewClonedContext returns a context with the same Values
// but not inherited cancelation.
func NewClonedContext(ctx context.Context) context.Context {
	return valueOnlyContext{Context: ctx}
}

type valueOnlyContext struct{ context.Context }

func (valueOnlyContext) Deadline() (deadline time.Time, ok bool) { return }
func (valueOnlyContext) Done() <-chan struct{}                   { return nil }
func (valueOnlyContext) Err() error                              { return nil }
