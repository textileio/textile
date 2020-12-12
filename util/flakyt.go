package util

import (
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestingTWithCleanup is an augmented require.TestingT with a Cleanup function.
type TestingTWithCleanup interface {
	require.TestingT
	TempDir() string
	Cleanup(func())
}

// FlakyT provides retry mechanisms to test.
type FlakyT struct {
	t      *testing.T
	failed bool
	cls    []func()
}

// NewFlakyT creates a new FlakyT.
func NewFlakyT(t *testing.T) *FlakyT {
	return &FlakyT{
		t: t,
	}
}

var _ require.TestingT = (*FlakyT)(nil)

// Errorf registers an error message.
func (ft *FlakyT) Errorf(format string, args ...interface{}) {
	ft.t.Logf(format, args...)
}

// FailNow indicates to fail the test.
func (ft *FlakyT) FailNow() {
	ft.failed = true
	runtime.Goexit()
}

// TempDir returns a temporary directory for the test to use.
// The directory is automatically removed by Cleanup when the test and
// all its subtests complete.
func (ft *FlakyT) TempDir() string {
	return ft.t.TempDir()
}

// Cleanup registers a cleanup function.
func (ft *FlakyT) Cleanup(cls func()) {
	ft.cls = append([]func(){cls}, ft.cls...)
}

var numRetries = 10

// RunFlaky runs a flaky test with retries.
func RunFlaky(t *testing.T, f func(ft *FlakyT)) {
	for i := 0; i < numRetries; i++ {
		var wg sync.WaitGroup
		wg.Add(1)
		ft := NewFlakyT(t)
		go func() {
			defer wg.Done()
			f(ft)
		}()
		wg.Wait()
		for _, f := range ft.cls {
			f()
		}
		if !ft.failed {
			return
		}
		ft.t.Logf("test %s attempt %d/%d failed, retrying...", t.Name(), i+1, numRetries)
	}
	t.Fatalf("test failed after %d retries", numRetries)
}
