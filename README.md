# cli-etc

Watch files and directories, execute commands over them.

`cli-etc` is a CLI tool and library that watches files matching glob patterns and executes commands on changes, once per base directory, parent directory or file.

## Installation

To install `etc` CLI tool, just run:

```bash
go install github.com/renatopp/cli-etc/cmd/etc@latest
```

To install `etcgo`, the live reload for Go programs:

```bash
go install github.com/renatopp/cli-etc/cmd/etcgo@latest
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

  `etc -i -k -p base '**/*.go' -- go run .`

Quote the command to prevent your shell from expanding `$1`. On Windows, `bash` must be in the `PATH` (e.g. Git Bash).

## etcgo Usage

`etcgo [package] <options...> [-- args...]`

Builds and runs the package (default `.`), rebuilding on changes to `**/*.go`, `go.mod` and `go.sum`. The program is only restarted when the build succeeds, so a compilation error keeps the last good version running. Tests, `.git`, `vendor`, `testdata` and `node_modules` are ignored. Arguments after `--` are passed to the program.

| Option                    | Description                                                    |
|---------------------------|----------------------------------------------------------------|
| `--watch, -w <pattern>`   | Extra patterns to watch, repeatable.                           |
| `--exclude, -e <pattern>` | Extra patterns to be ignored, repeatable.                      |
| `--build, -B <flags>`     | Extra flags passed to `go build`.                              |
| `--delay, -d <int>`       | Window of accumulating changes in milliseconds, default 500.   |

- Run the package in the current directory:

  `etcgo`

- Run a specific package, passing arguments to it:

  `etcgo ./cmd/server -- --port 8080`

- Also restart on template changes, building with the race detector:

  `etcgo -w '**/*.html' -B '-race'`

The program is stopped with `SIGTERM` (`CTRL_BREAK` on Windows, received as `os.Interrupt` by Go programs) and killed after 5 seconds.

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

Kill is done by cancelling the command context, `Shell` and `Exec` kill the whole process group (a job object on Windows). Custom commands must honor it.
