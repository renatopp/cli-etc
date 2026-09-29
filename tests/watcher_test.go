package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/renatopp/cli-etc/etc"
)

func TestFileTarget(t *testing.T) {
	base := newBase(t)
	rec := newRecorder()
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"**/*.png"}, Delay: testDelay, Command: rec},
	}})

	touch(t, base, "images/player.png")
	touch(t, base, "bg/back.png")

	events := rec.wait(t, 2)
	equal(t, join(base, "bg/back.png"), []string{events[0].Target})
	equal(t, join(base, "bg/back.png"), events[0].Files)
	equal(t, join(base, "images/player.png"), []string{events[1].Target})
	rec.none(t, 2*testDelay)
}

func TestDirTarget(t *testing.T) {
	base := newBase(t, "images/.keep", "bg/.keep")
	rec := newRecorder()
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"**/*.png"}, Target: etc.DirTarget, Delay: testDelay, Command: rec},
	}})

	touch(t, base, "images/player.png")
	touch(t, base, "images/enemy.png")
	touch(t, base, "bg/back.png")

	events := rec.wait(t, 2)
	equal(t, join(base, "bg"), []string{events[0].Target})
	equal(t, join(base, "bg/back.png"), events[0].Files)
	equal(t, join(base, "images"), []string{events[1].Target})
	equal(t, join(base, "images/enemy.png", "images/player.png"), events[1].Files)
	rec.none(t, 2*testDelay)
}

func TestBaseTarget(t *testing.T) {
	base := newBase(t, "images/.keep", "bg/.keep")
	rec := newRecorder()
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"**/*.png"}, Target: etc.BaseTarget, Delay: testDelay, Command: rec},
	}})

	touch(t, base, "images/player.png")
	touch(t, base, "bg/back.png")
	touch(t, base, "bg/ignored.txt")

	events := rec.wait(t, 1)
	equal(t, base, events[0].Target)
	equal(t, join(base, "bg/back.png", "images/player.png"), events[0].Files)
	rec.none(t, 2*testDelay)
}

func TestExclude(t *testing.T) {
	base := newBase(t, "images/.keep", "bg/.keep")
	rec := newRecorder()
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"**/*.png"}, Exclude: []string{"bg", "**/tmp_*"}, Delay: testDelay, Command: rec},
	}})

	touch(t, base, "images/player.png")
	touch(t, base, "images/tmp_player.png")
	touch(t, base, "bg/back.png")

	events := rec.wait(t, 1)
	equal(t, join(base, "images/player.png"), events[0].Files)
	rec.none(t, 2*testDelay)
}

func TestInitial(t *testing.T) {
	base := newBase(t, "images/player.png", "images/enemy.png", "bg/back.png", "bg/readme.txt")
	rec := newRecorder()
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"**/*.png"}, Target: etc.DirTarget, Initial: true, Command: rec},
	}})

	events := rec.wait(t, 2)
	equal(t, join(base, "bg/back.png"), events[0].Files)
	equal(t, join(base, "images/enemy.png", "images/player.png"), events[1].Files)
}

func TestDelayAccumulates(t *testing.T) {
	base := newBase(t)
	rec := newRecorder()
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"*.txt"}, Target: etc.BaseTarget, Delay: 200 * time.Millisecond, Command: rec},
	}})

	for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
		touch(t, base, f)
		time.Sleep(50 * time.Millisecond)
	}

	events := rec.wait(t, 1)
	equal(t, join(base, "a.txt", "b.txt", "c.txt"), events[0].Files)
	rec.none(t, 300*time.Millisecond)
}

func TestNewDirectory(t *testing.T) {
	base := newBase(t)
	rec := newRecorder()
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"**/*.png"}, Delay: testDelay, Command: rec},
	}})

	touch(t, base, "new/deep/player.png")
	events := rec.wait(t, 1)
	equal(t, join(base, "new/deep/player.png"), events[0].Files)

	// The new directory must be watched as well
	touch(t, base, "new/deep/enemy.png")
	events = rec.wait(t, 1)
	equal(t, join(base, "new/deep/enemy.png"), events[0].Files)
}

func TestRequeue(t *testing.T) {
	base := newBase(t)
	rec := newRecorder()
	var running, overlaps atomic.Int32
	rec.fn = func(ctx context.Context, ev etc.Event) error {
		if running.Add(1) > 1 {
			overlaps.Add(1)
		}
		defer running.Add(-1)
		time.Sleep(300 * time.Millisecond)
		return nil
	}
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"*.txt"}, Target: etc.BaseTarget, Delay: testDelay, Command: rec},
	}})

	touch(t, base, "a.txt")
	rec.wait(t, 1)
	touch(t, base, "b.txt")

	began := time.Now()
	events := rec.wait(t, 1)
	if time.Since(began) < 150*time.Millisecond {
		t.Fatalf("requeued command started before the previous one finished")
	}
	equal(t, join(base, "b.txt"), events[0].Files)
	equal(t, int32(0), overlaps.Load())
}

func TestKill(t *testing.T) {
	base := newBase(t)
	rec := newRecorder()
	var killed atomic.Int32
	rec.fn = func(ctx context.Context, ev etc.Event) error {
		select {
		case <-ctx.Done():
			killed.Add(1)
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	}
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"*.txt"}, Target: etc.BaseTarget, Kill: true, Delay: testDelay, Command: rec},
	}})

	touch(t, base, "a.txt")
	rec.wait(t, 1)
	touch(t, base, "b.txt")

	// The killed files are requeued with the new ones
	events := rec.wait(t, 1)
	equal(t, int32(1), killed.Load())
	equal(t, join(base, "a.txt", "b.txt"), events[0].Files)
}

func TestKillIsPerTarget(t *testing.T) {
	base := newBase(t)
	rec := newRecorder()
	var killed atomic.Int32
	rec.fn = func(ctx context.Context, ev etc.Event) error {
		select {
		case <-ctx.Done():
			killed.Add(1)
		case <-time.After(500 * time.Millisecond):
		}
		return nil
	}
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"*.txt"}, Kill: true, Delay: testDelay, Command: rec},
	}})

	touch(t, base, "a.txt")
	rec.wait(t, 1)
	touch(t, base, "b.txt")
	rec.wait(t, 1)
	equal(t, int32(0), killed.Load())
}

func TestParallel(t *testing.T) {
	base := newBase(t)
	rec := newRecorder()
	var running, peak atomic.Int32
	rec.fn = func(ctx context.Context, ev etc.Event) error {
		n := running.Add(1)
		defer running.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
		return nil
	}
	start(t, &etc.Watcher{Base: base, Parallel: 2, Rules: []etc.Rule{
		{Patterns: []string{"*.txt"}, Delay: testDelay, Command: rec},
	}})

	for _, f := range []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt"} {
		touch(t, base, f)
	}

	rec.wait(t, 5)
	equal(t, int32(2), peak.Load())
}

func TestMultipleRules(t *testing.T) {
	base := newBase(t)
	slow := newRecorder()
	slow.fn = func(ctx context.Context, ev etc.Event) error {
		<-ctx.Done()
		return nil
	}
	fast := newRecorder()
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"*.txt"}, Target: etc.BaseTarget, Delay: testDelay, Command: slow},
		{Patterns: []string{"*"}, Target: etc.BaseTarget, Delay: testDelay, Command: fast},
	}})

	touch(t, base, "a.txt")
	slow.wait(t, 1)
	fast.wait(t, 1)

	// Slow rule is still running, but fast rule is not blocked by it
	touch(t, base, "b.txt")
	events := fast.wait(t, 1)
	equal(t, join(base, "b.txt"), events[0].Files)
}

func TestFail(t *testing.T) {
	base := newBase(t)
	errBoom := errors.New("boom")
	w := &etc.Watcher{Base: base, Fail: true, Rules: []etc.Rule{
		{Patterns: []string{"*.txt"}, Delay: testDelay, Command: etc.CommandFunc(func(ctx context.Context, ev etc.Event) error {
			return errBoom
		})},
	}}

	errs := make(chan error, 1)
	go func() { errs <- w.Run(context.Background()) }()
	time.Sleep(100 * time.Millisecond)
	touch(t, base, "a.txt")

	select {
	case err := <-errs:
		if !errors.Is(err, errBoom) {
			t.Fatalf("expected boom error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not stop on failure")
	}
}

func TestNoFailKeepsWatching(t *testing.T) {
	base := newBase(t)
	rec := newRecorder()
	rec.fn = func(ctx context.Context, ev etc.Event) error { return errors.New("boom") }
	start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"*.txt"}, Delay: testDelay, Command: rec},
	}})

	touch(t, base, "a.txt")
	rec.wait(t, 1)
	touch(t, base, "b.txt")
	rec.wait(t, 1)
}

func TestRunCancelsCommands(t *testing.T) {
	base := newBase(t, "a.txt")
	rec := newRecorder()
	var cancelled atomic.Int32
	rec.fn = func(ctx context.Context, ev etc.Event) error {
		<-ctx.Done()
		cancelled.Add(1)
		return nil
	}
	stop := start(t, &etc.Watcher{Base: base, Rules: []etc.Rule{
		{Patterns: []string{"*.txt"}, Initial: true, Command: rec},
	}})

	rec.wait(t, 1)
	if err := stop(); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	equal(t, int32(1), cancelled.Load())
}

func TestValidation(t *testing.T) {
	cases := map[string]etc.Watcher{
		"no rules":    {},
		"no command":  {Rules: []etc.Rule{{Patterns: []string{"*"}}}},
		"no patterns": {Rules: []etc.Rule{{Command: newRecorder()}}},
		"bad pattern": {Rules: []etc.Rule{{Patterns: []string{"[a"}, Command: newRecorder()}}},
		"bad base":    {Base: "/does/not/exist", Rules: []etc.Rule{{Patterns: []string{"*"}, Command: newRecorder()}}},
	}
	for name, w := range cases {
		if err := w.Run(context.Background()); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
