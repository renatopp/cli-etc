package etc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/renatopp/go-x/fsx"
)

const (
	DefaultParallel = 4
	DefaultDelay    = 500 * time.Millisecond
)

// Watcher watches Base and executes the rules commands on changes.
type Watcher struct {
	Base     string      // base directory, default current PWD. Rule patterns are relative to it
	Parallel int         // max parallel execution across all rules, default 4
	Fail     bool        // stop the watcher if any command fails
	Rules    []Rule      // pattern -> command rules
	Logger   *log.Logger // optional logger for executions, silent by default
}

// Run blocks until ctx is cancelled or a command fails with Fail set, in which
// case that error is returned. Before returning, running commands are
// cancelled and waited for.
func (w *Watcher) Run(ctx context.Context) error {
	if err := w.validate(); err != nil {
		return err
	}

	base := w.Base
	if base == "" {
		base = "."
	}
	base = filepath.Clean(base)
	abs, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	if !fsx.IsDir(abs) {
		return fmt.Errorf("base %q is not a directory", w.Base)
	}

	parallel := w.Parallel
	if parallel <= 0 {
		parallel = DefaultParallel
	}
	logger := w.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)

	r := &runner{
		rules:  w.Rules,
		base:   base,
		fail:   w.Fail,
		logger: logger,
		ctx:    ctx,
		stop:   stop,
		sem:    make(chan struct{}, parallel),
		slots:  map[slotKey]*slot{},
	}

	fw, err := fsx.NewWatcher()
	if err != nil {
		return err
	}
	defer fw.Close()

	t := &tree{abs: abs, fw: fw, rules: w.Rules, dirs: map[string]bool{}}
	files, err := t.add(abs)
	if err != nil {
		return err
	}

	r.initial(files)
	err = fw.Watch(ctx, func(ev fsx.Event) { t.handle(ev, r, logger) })

	stop(nil)
	r.shutdown()
	if r.err != nil {
		return r.err
	}
	return err
}

func (w *Watcher) validate() error {
	if len(w.Rules) == 0 {
		return errors.New("no rules defined")
	}
	for i, rule := range w.Rules {
		if rule.Command == nil {
			return fmt.Errorf("rule %d has no command", i)
		}
		if len(rule.Patterns) == 0 {
			return fmt.Errorf("rule %d has no patterns", i)
		}
		for _, p := range slices.Concat(rule.Patterns, rule.Exclude) {
			if !fsx.IsPatternValid(p) {
				return fmt.Errorf("rule %d has invalid pattern %q", i, p)
			}
		}
	}
	return nil
}

// tree keeps the fsnotify watches in sync with the directories under abs.
// fsnotify is not recursive, so every directory is watched individually.
type tree struct {
	abs   string
	fw    *fsx.Watcher
	rules []Rule
	dirs  map[string]bool // watched directories, absolute
}

// add watches dir and all its subdirectories, skipping the ones excluded by
// every rule. It returns the files found, as slash paths relative to abs.
func (t *tree) add(dir string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Removed while walking
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}

		rel := t.rel(p)
		if !d.IsDir() {
			files = append(files, rel)
			return nil
		}
		if rel != "." && t.excluded(rel) {
			return filepath.SkipDir
		}
		if t.dirs[p] {
			return nil
		}
		if err := t.fw.Add(p); err != nil {
			return err
		}
		t.dirs[p] = true
		return nil
	})
	return files, err
}

// remove stops watching dir and its subdirectories.
func (t *tree) remove(dir string) {
	prefix := dir + string(filepath.Separator)
	for p := range t.dirs {
		if p == dir || strings.HasPrefix(p, prefix) {
			_ = t.fw.Remove(p)
			delete(t.dirs, p)
		}
	}
}

func (t *tree) handle(ev fsx.Event, r *runner, logger *log.Logger) {
	if ev.Has(fsx.EvtError) {
		logger.Printf("watch error: %v", ev.Err)
		return
	}
	if !ev.Has(fsx.EvtCreate) && !ev.Has(fsx.EvtWrite) && !ev.Has(fsx.EvtRemove) && !ev.Has(fsx.EvtRename) {
		return
	}

	if t.dirs[ev.Path] {
		if ev.Has(fsx.EvtRemove) || ev.Has(fsx.EvtRename) {
			t.remove(ev.Path)
		}
		return
	}

	if fsx.IsDir(ev.Path) {
		if ev.Has(fsx.EvtCreate) && !t.excluded(t.rel(ev.Path)) {
			// Files may be created before the watch is attached
			files, err := t.add(ev.Path)
			if err != nil {
				logger.Printf("watch error: %v", err)
			}
			for _, f := range files {
				r.change(f)
			}
		}
		return
	}

	r.change(t.rel(ev.Path))
}

// excluded reports if the directory rel is excluded by every rule.
func (t *tree) excluded(rel string) bool {
	for i := range t.rules {
		if !t.rules[i].excludes(rel) {
			return false
		}
	}
	return true
}

func (t *tree) rel(p string) string {
	rel, err := filepath.Rel(t.abs, p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(rel)
}
