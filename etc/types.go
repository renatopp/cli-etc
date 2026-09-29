package etc

import (
	"context"
	"fmt"
	"time"
)

// Target defines which path is used as the target of an execution.
type Target int

const (
	FileTarget Target = iota // once per changed file
	DirTarget                // once per parent directory of the changed files
	BaseTarget               // once per base directory
)

func (t Target) String() string {
	switch t {
	case FileTarget:
		return "file"
	case DirTarget:
		return "dir"
	case BaseTarget:
		return "base"
	}
	return fmt.Sprintf("Target(%d)", int(t))
}

// ParseTarget converts "base", "dir" or "file" into a Target.
func ParseTarget(s string) (Target, error) {
	switch s {
	case "file":
		return FileTarget, nil
	case "dir":
		return DirTarget, nil
	case "base":
		return BaseTarget, nil
	}
	return 0, fmt.Errorf("invalid target %q, expected base, dir or file", s)
}

// Event is passed to the command on each execution.
type Event struct {
	Rule   *Rule
	Target string   // base, dir or file, depending on Rule.Target
	Files  []string // changed files under Target accumulated during Delay
}

// Command is executed when a rule is triggered. Commands must honor ctx
// cancellation, which is used to kill them.
type Command interface {
	Run(ctx context.Context, ev Event) error
}

// CommandFunc adapts a function to the Command interface.
type CommandFunc func(ctx context.Context, ev Event) error

func (f CommandFunc) Run(ctx context.Context, ev Event) error { return f(ctx, ev) }

// Rule maps a set of patterns to a command.
type Rule struct {
	Patterns []string      // globs relative to Base, `**` supported
	Exclude  []string      // globs to be ignored
	Target   Target        // default FileTarget
	Delay    time.Duration // window of accumulating changes, default 500ms
	Initial  bool          // execute upon start
	Kill     bool          // cancel the running command of the same target instead of requeuing
	Command  Command       // what to run
}
