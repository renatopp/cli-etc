If I'am watching `**/*.png` inside `./assets`, and multiple files have changed (let's say `./assets/images/player.png` and `./assets/bg/back.png`), I may want to exec commands:

- Only once per base directory (ex `./assets`)
- Once per parent directory (ex `./assets/images` and `./assets/bg`)
- Once per file (ex `./assets/images/player.png` and `./assets/bg/back.png`)
- All of the above

Thus, every rule should have:

- Pattern
- Base command
- Dir command
- File command
- Debounce time

Open problems:

- Feedback loops

---

Cli perspective:

```bash
wtc \
  'src/assets/**/*.png' \ # allow multiple glob
  'public/**/*.jpg' \
  --base '...' \ # command to be run once per base directory
  --dir '...' \ # command to be run once per parent directory
  --file '...' \ # command to be run once per file
  --debounce 1000 \ # debounce time in ms, default 500ms
  --initial \ # run commands on initial scan
```

Library perspective:

```go
import "github.com/renatopp/cli-wtc/watcher"

func main() {
	watcher.NewWatcher(watcher.Options{
    Patterns: []string{"src/assets/**/*.png", "public/**/*.jpg"},
    BaseCommand: func(ev watcher.DirEvent) ...,
    DirCommand:  func(ev watcher.DirEvent) ...,
    FileCommand: func(ev watcher.FileEvent) ...,
    Debounce:    1000*time.Millisecond,
    Initial:     true,
  }).Start()
}
```

Notes:

- If multiple files change in the same directory within debounce window, `base` and `dir` commands will execute only once
