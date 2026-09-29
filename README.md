# cli-etc

Watch files and directories, execute commands over them.

`cli-etc` is a CLI tool and library that watches files matching glob patterns and executes commands on changes, once per base directory, parent directory or file.

## Installation

To install `etc` CLI tool, just run:

```bash
go install github.com/renatopp/cli-etc/cmd/etc@latest
```

To install as a library, use:

```bash
go get github.com/renatopp/cli-etc/etc
```

Then, import the package `github.com/renatopp/cli-etc/etc` in your Go code.

## CLI Usage

`etc [patterns...] <options...> -- <command>`

Commands are executed as `bash -c <command> <pwd> <target>`, so the target is accessible as `$1`.

| Option                        | Description                                                        |
|-------------------------------|--------------------------------------------------------------------|
| `--base, -b <string>`         | Base directory, default current directory.                         |
| `--per, -p <base\|dir\|file>` | Target of the execution, default `file`.                           |
| `--delay, -d <int>`           | Window of accumulating changes in milliseconds, default 500.       |
| `--exclude, -e <pattern>`     | Patterns to be ignored, repeatable.                                |
| `--initial, -i`               | Execute upon start.                                                |
| `--kill, -k`                  | Kill the running process of the same target instead of requeuing.  |
| `--parallel, -j <int>`        | Max parallel executions, default 4.                                |
| `--fail, -F`                  | Stop watching if any command fails.                                |

- Run tests when any go file changes:

  `etc '**/*.go' -- go test ./...`

- Pack an atlas per directory with changed images:

  `etc -b assets -p dir '**/*.png' -- 'pack-atlas $1'`

- Optimize each changed image, ignoring the `tmp` directory:

  `etc '**/*.png' -e tmp -- 'optipng $1'`

- Run a program and restart it on changes:

  `etc -i -k '**/*.go' -- go run .`

Quote the command to prevent your shell from expanding `$1`.

## Library Usage

The library supports multiple rules (patterns -> command). A change is matched against every rule, each rule schedules its own execution, requeue and kill happen per (rule, target) pair and the `Parallel` limit is shared by all rules.

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

Built-in commands:

- `etc.Shell(cmd)` runs `bash -c <cmd>` with the target as `$1`.
- `etc.Exec(name, args...)` runs the program directly, `{}` in args is replaced by the target.

Kill is done by cancelling the command context, `Shell` and `Exec` kill the whole process group. Custom commands must honor it.
