# SPEC

This tool is a CLI utility to watch files and execute commands on changes. It also provides a library to be used in Go programs.

Consider that we're watching `**/*.png` inside `./assets`, and multiple files have changed (let's say `./assets/images/player.png` and `./assets/bg/back.png`), We may want to exec commands:

- Only once per base directory (ex `./assets`)
- Once per parent directory (ex `./assets/images` and `./assets/bg`)
- Once per file (ex `./assets/images/player.png` and `./assets/bg/back.png`)
- All of the above

## CLI

`etc [patterns...] <options...>`, where patterns is a variadic list of globs, and options:

- `--base, -b <string>` commands to be run on base target
- `--dir, -d <string>` commands to be run on dir target
- `--file, -f <string>` commands to be run on file target
- `--debounce, -D <int>` window of accumulating changes before execution, in milliseconds, default 500
- `--initial, -i` boolean flag to execute upon first call

Commands will be executed internally as `bash -c <command> <pwd> <base/dir/file changed>`, so user can use $1 to access the target file or folder.

## LIBRARY

Library should be a bit more flexible, I want to allow multiple rules (pattern -> commands).

Each rule associates a set of patterns with its own base/dir/file commands and debounce, allowing multiple independent rules to be registered in the same watcher instance.

```go
import (
  "time"
  "github.com/renatopp/cli-wtc/watcher"
)

func main() {
  w := watcher.NewWatcher(watcher.Options{
    Initial: true, // applies to all rules unless overridden
    Rules: []watcher.Rule{
      {
        Patterns:    []string{"src/assets/**/*.png"},
        BaseCommand: func(ev watcher.BaseEvent) { ... },
        DirCommand:  func(ev watcher.DirEvent) { ... },
        FileCommand: func(ev watcher.FileEvent) { ... },
        Debounce:    500 * time.Millisecond,
      },
      {
        Patterns:    []string{"public/**/*.jpg"},
        FileCommand: func(ev watcher.FileEvent) { ... },
        Debounce:    1000 * time.Millisecond,
      },
    },
  })

  w.Start()
}
```

Notes:

- Each `Rule` behaves independently, with its own debounce window and commands.
- A file event may match multiple rules, in which case all matching rules' commands are triggered.
- `Options.Initial` sets the default for all rules, but each `Rule` may override it with its own `Initial` field.
- Rules can be added or removed dynamically via `w.AddRule(rule)` and `w.RemoveRule(rule)` while the watcher is running.
- If multiple files change in the same directory within debounce window, `base` and `dir` commands will execute only once
