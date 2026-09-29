# SPEC

This tool is a CLI utility to watch files and execute commands on changes. It also provides a library to be used in Go programs.

Consider that we're watching `**/*.png` inside `./assets`, and multiple files have changed (let's say `./assets/images/player.png` and `./assets/bg/back.png`), We may want to exec commands:

- Only once per base directory (ex `./assets`)
- Once per parent directory (ex `./assets/images` and `./assets/bg`)
- Once per file (ex `./assets/images/player.png` and `./assets/bg/back.png`)

## CLI

`etc [patterns...] <options...> -- <command>`, where patterns is a variadic list of globs, and options:

- `--base, -b <string>` the base directory, default current PWD
- `--per, -p <base|dir|file>` target of the execution, default is file. Dir is the parent directory of the file.
- `--delay, -d <int>` window of accumulating changes before execution, in milliseconds, default 500ms.
- `--exclude, -e <pattern>` patterns to be ignored, repeated allowed, default empty
- `--initial, -i` boolean flag to execute upon first call, false by default
- `--kill, -k` boolean to allow kill processes running from previous execution, false by default.
- `--parallel, -j <int>` max parallel execution, default 4
- `--fail, -F` boolean to interrupt the watcher if any command fail.

Commands will be executed internally as `bash -c <command> <pwd> <base/dir/file changed>`, so user can use $1 to access the target file or folder.

Using `-k`, the watcher will kill the previous process. Notice that if target is dir or file, multiple processed may run in parallel. In this case, the kill will happen only on the dir or file attached to the event. Without -k, if the process is still running, the command will be requeued waiting for previous process termination.

## LIBRARY

Library should be a bit more flexible, I want to allow multiple rules (pattern -> commands). The CLI is a thin wrapper that builds a single rule from its flags.

### Watcher

Global settings, shared by all rules:

- `Base string` the base directory, default current PWD. Rule patterns are relative to it.
- `Parallel int` max parallel execution across all rules, default 4.
- `Fail bool` stop the watcher if any command fails.
- `Rules []Rule`

`Run(ctx context.Context) error` blocks until ctx is cancelled or a command fails with `Fail` set, in which case that error is returned. Before returning, running commands are cancelled and waited for.

### Rule

- `Patterns []string` globs relative to Base, `**` supported.
- `Exclude []string` globs to be ignored.
- `Target Target` (`BaseTarget`, `DirTarget`, `FileTarget`), default `FileTarget`.
- `Delay time.Duration` window of accumulating changes, default 500ms.
- `Initial bool` execute upon start.
- `Kill bool` cancel the running command of the same target instead of requeuing.
- `Command Command` what to run.

### Command

```go
type Command interface {
    Run(ctx context.Context, ev Event) error
}

type CommandFunc func(ctx context.Context, ev Event) error
```

Built-in commands:

- `Shell(cmd string) Command` runs `bash -c <cmd>` the same way the CLI does, with the target as `$1`.
- `Exec(name string, args ...string) Command` runs the program directly, without shell. `{}` in args is replaced by the target.

Kill is done by cancelling ctx. Commands must honor it; `Shell` and `Exec` kill the whole process group.

### Event

```go
type Event struct {
    Rule   *Rule
    Target string   // base, dir or file, depending on Rule.Target
    Files  []string // changed files under Target accumulated during Delay
}
```

On initial execution, `Files` contains all files matching the rule under Target.

### Multiple rules

- A change is matched against every rule. Each matching rule schedules its own execution, rules don't block each other.
- Requeue and kill happen per (rule, target) pair.
- The `Parallel` limit is shared by all rules.

### Example

```go
w := etc.Watcher{
    Base: "./assets",
    Rules: []etc.Rule{
        {
            Patterns: []string{"**/*.png"},
            Target:   etc.DirTarget,
            Command:  etc.Shell("pack-atlas $1"),
        },
        {
            Patterns: []string{"**/*.json"},
            Kill:     true,
            Command:  etc.CommandFunc(reloadConfig),
        },
    },
}

err := w.Run(ctx)
```
