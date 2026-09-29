package etc

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/renatopp/go-x/fmtx"
	"github.com/renatopp/go-x/mapx"
)

// slotKey identifies a (rule, target) pair. Requeue and kill happen per slot.
type slotKey struct {
	rule   int
	target string
}

type slot struct {
	key     slotKey
	rule    *Rule
	pending map[string]struct{} // files accumulated for the next execution
	timer   *time.Timer         // delay timer, nil if not scheduled
	gen     int                 // invalidates timers replaced by newer changes
	waiting bool                // delay elapsed while the command was running
	running bool
	cancel  context.CancelFunc
}

// runner holds the state of a single Watcher.Run call.
type runner struct {
	rules  []Rule
	base   string // base as given by the user, used to build targets
	fail   bool
	logger *log.Logger

	ctx  context.Context
	stop context.CancelCauseFunc
	sem  chan struct{}

	mu     sync.Mutex
	slots  map[slotKey]*slot
	closed bool
	err    error // first command error when fail is set
	wg     sync.WaitGroup
}

// change registers a changed file (slash path relative to base) on every
// matching rule, restarting their delay timers.
func (r *runner) change(rel string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}

	for i := range r.rules {
		rule := &r.rules[i]
		if !rule.matches(rel) {
			continue
		}

		s := r.slotFor(i, rel)
		s.gen++
		gen := s.gen
		if s.timer != nil {
			s.timer.Stop()
		}
		s.timer = time.AfterFunc(delayOf(rule), func() { r.fire(s, gen) })
	}
}

// initial triggers the rules with Initial set immediately, using all matching
// files.
func (r *runner) initial(files []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	slots := []*slot{}
	for i := range r.rules {
		rule := &r.rules[i]
		if !rule.Initial {
			continue
		}
		for _, rel := range files {
			if !rule.matches(rel) {
				continue
			}
			s := r.slotFor(i, rel)
			if !slices.Contains(slots, s) {
				slots = append(slots, s)
			}
		}
	}

	for _, s := range slots {
		r.trigger(s)
	}
}

// shutdown stops the timers and waits for the running commands. The context
// must be already cancelled.
func (r *runner) shutdown() {
	r.mu.Lock()
	r.closed = true
	for _, s := range r.slots {
		if s.timer != nil {
			s.timer.Stop()
		}
	}
	r.mu.Unlock()
	r.wg.Wait()
}

// slotFor returns the slot of rule i for the file rel, adding rel to its
// pending files. Must be called with the lock held.
func (r *runner) slotFor(i int, rel string) *slot {
	rule := &r.rules[i]
	key := slotKey{rule: i, target: rule.targetOf(r.base, rel)}
	s, ok := r.slots[key]
	if !ok {
		s = &slot{key: key, rule: rule, pending: map[string]struct{}{}}
		r.slots[key] = s
	}
	s.pending[filepath.Join(r.base, rel)] = struct{}{}
	return s
}

// fire is called when the delay timer of the slot elapses.
func (r *runner) fire(s *slot, gen int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || gen != s.gen {
		return
	}
	s.timer = nil
	r.trigger(s)
}

// trigger dispatches the slot, or marks it as waiting if its command is still
// running, killing it when the rule allows. Must be called with the lock held.
func (r *runner) trigger(s *slot) {
	if len(s.pending) == 0 {
		r.gc(s)
		return
	}
	if s.running {
		s.waiting = true
		if s.rule.Kill {
			s.cancel()
		}
		return
	}
	r.dispatch(s)
}

// dispatch starts the execution with the pending files. Must be called with
// the lock held.
func (r *runner) dispatch(s *slot) {
	files := mapx.Keys(s.pending)
	slices.Sort(files)
	s.pending = map[string]struct{}{}
	s.waiting = false
	s.running = true

	ctx, cancel := context.WithCancel(r.ctx)
	s.cancel = cancel

	r.wg.Add(1)
	go r.execute(ctx, cancel, s, Event{Rule: s.rule, Target: s.key.target, Files: files})
}

func (r *runner) execute(ctx context.Context, cancel context.CancelFunc, s *slot, ev Event) {
	defer r.wg.Done()
	defer cancel()

	var err error
	var elapsed time.Duration
	select {
	case r.sem <- struct{}{}:
		r.logger.Printf("%s %s", fmtx.Cyan("run"), ev.Target)
		start := time.Now()
		err = s.rule.Command.Run(ctx, ev)
		elapsed = time.Since(start)
		<-r.sem
	case <-ctx.Done():
		err = ctx.Err()
	}
	killed := ctx.Err() != nil

	r.mu.Lock()
	defer r.mu.Unlock()
	s.running = false

	switch {
	case r.closed || r.ctx.Err() != nil:
		return

	case killed:
		// Files of the killed execution were not fully processed
		r.logger.Printf("%s %s", fmtx.Yellow("killed"), ev.Target)
		for _, f := range ev.Files {
			s.pending[f] = struct{}{}
		}

	case err != nil:
		r.logger.Printf("%s %s %s", fmtx.Red("fail"), ev.Target, fmtx.Dim(err.Error()))
		if r.fail {
			if r.err == nil {
				r.err = fmt.Errorf("command failed on %s: %w", ev.Target, err)
			}
			r.stop(r.err)
			return
		}

	default:
		r.logger.Printf("%s %s %s", fmtx.Green("done"), ev.Target, fmtx.Dim(elapsed.Round(time.Millisecond).String()))
	}

	if s.waiting {
		r.dispatch(s)
		return
	}
	r.gc(s)
}

// gc removes idle slots. Must be called with the lock held.
func (r *runner) gc(s *slot) {
	if !s.running && !s.waiting && s.timer == nil && len(s.pending) == 0 {
		delete(r.slots, s.key)
	}
}

func delayOf(rule *Rule) time.Duration {
	if rule.Delay <= 0 {
		return DefaultDelay
	}
	return rule.Delay
}
