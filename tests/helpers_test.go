package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/renatopp/cli-etc/etc"
)

const testDelay = 50 * time.Millisecond

// newBase creates a temporary base directory with the given files.
func newBase(t *testing.T, files ...string) string {
	t.Helper()
	base := t.TempDir()
	for _, f := range files {
		touch(t, base, f)
	}
	return base
}

// touch creates or writes the file rel under base.
func touch(t *testing.T, base, rel string) {
	t.Helper()
	p := filepath.Join(base, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(time.Now().String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// recorder is a command that records the received events.
type recorder struct {
	mu     sync.Mutex
	events []etc.Event
	ch     chan etc.Event
	fn     func(ctx context.Context, ev etc.Event) error
}

func newRecorder() *recorder {
	return &recorder{ch: make(chan etc.Event, 100)}
}

func (r *recorder) Run(ctx context.Context, ev etc.Event) error {
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
	r.ch <- ev
	if r.fn != nil {
		return r.fn(ctx, ev)
	}
	return nil
}

// wait waits for n events, failing on timeout.
func (r *recorder) wait(t *testing.T, n int) []etc.Event {
	t.Helper()
	events := []etc.Event{}
	timeout := time.After(3 * time.Second)
	for len(events) < n {
		select {
		case ev := <-r.ch:
			events = append(events, ev)
		case <-timeout:
			t.Fatalf("expected %d events, got %d: %+v", n, len(events), events)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Target < events[j].Target })
	return events
}

// none asserts no event is received during d.
func (r *recorder) none(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case ev := <-r.ch:
		t.Fatalf("unexpected event %+v", ev)
	case <-time.After(d):
	}
}

// start runs the watcher in background, returning a function that stops it and
// returns the Run error.
func start(t *testing.T, w *etc.Watcher) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() { errs <- w.Run(ctx) }()

	// Give time for the watches to be attached
	time.Sleep(100 * time.Millisecond)

	stopped := false
	var err error
	stop = func() error {
		if !stopped {
			stopped = true
			cancel()
			err = <-errs
		}
		return err
	}
	t.Cleanup(func() { stop() })
	return stop
}

func equal(t *testing.T, expected, actual any) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func join(base string, rels ...string) []string {
	res := make([]string, len(rels))
	for i, r := range rels {
		res[i] = filepath.Join(base, r)
	}
	return res
}
