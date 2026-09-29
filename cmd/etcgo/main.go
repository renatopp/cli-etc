package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/renatopp/cli-etc/etc"
	"github.com/renatopp/go-cli"
	"github.com/renatopp/go-x/fmtx"
	"github.com/renatopp/go-x/fsx"
)

const cliDescription = `
Build, run and restart a Go program on changes.

Watches go files, go.mod and go.sum under the current directory, ignoring
tests, .git, vendor, testdata and node_modules. On changes, the program is
rebuilt and only restarted if the build succeeds, so a compilation error keeps
the last good version running.

Arguments after '--' are passed to the program.

More information, issues and contributions: http://github.com/renatopp/cli-etc
`

var (
	defaultPatterns = []string{"**/*.go", "go.mod", "go.sum"}
	defaultExcludes = []string{"**/*_test.go", ".git", "vendor", "**/testdata", "**/node_modules"}
)

// keep is passed to etc.Exec so `{}` in the arguments is left untouched.
var keep = etc.Event{Target: "{}"}

func main() {
	args, appArgs := splitArgs(os.Args[1:])

	cli.Name("etcgo")
	cli.Description(cliDescription)
	cli.AutoHelp(true)
	cli.Example("etcgo", "Build, run and restart the package in the current directory.")
	cli.Example("etcgo ./cmd/server -- --port 8080", "Run a specific package, passing arguments to it.")
	cli.Example("etcgo -w '**/*.html' -w .env", "Also restart when templates or the env file change.")
	cli.Example("etcgo -B '-race -tags dev'", "Pass extra flags to go build.")

	pkg := cli.Pos("package", "Package to build and run.").WithDefault(".")
	watch := cli.Flag("watch", "w", "Extra patterns to watch. Repeatable.").AsRepeatable()
	exclude := cli.Flag("exclude", "e", "Extra patterns to be ignored. Repeatable.").AsRepeatable()
	build := cli.Flag("build", "B", "Extra flags passed to go build.").WithDefault("")
	delay := cli.FlagInt("delay", "d", "Window of accumulating changes before rebuilding, in milliseconds.").WithDefault(500)
	cli.ParseArgs(args)

	dir, err := os.MkdirTemp("", "etcgo-")
	cli.FatalIf(err)
	defer os.RemoveAll(dir)

	logger := log.New(os.Stderr, fmtx.Dim("[etcgo] "), 0)
	r := &runner{
		pkg:    pkg.Value(),
		flags:  strings.Fields(build.Value()),
		args:   appArgs,
		dir:    dir,
		name:   binName(pkg.Value()),
		logger: logger,
	}

	w := etc.Watcher{
		Parallel: 1,
		Logger:   logger,
		Rules: []etc.Rule{{
			Patterns: slices.Concat(defaultPatterns, watch.Values()),
			Exclude:  slices.Concat(defaultExcludes, exclude.Values()),
			Target:   etc.BaseTarget,
			Delay:    time.Duration(delay.Value()) * time.Millisecond,
			Initial:  true,
			Kill:     true,
			Command:  r,
		}},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Printf("watching %s", strings.Join(w.Rules[0].Patterns, " "))
	err = w.Run(ctx)
	r.stop()
	cli.FatalIf(err)
}

// runner builds the package on each execution and restarts the program when
// the build succeeds. The watcher never runs it concurrently, since there is a
// single rule with a single target.
type runner struct {
	pkg    string
	flags  []string // extra go build flags
	args   []string // program arguments
	dir    string   // where binaries are built
	name   string
	logger *log.Logger
	builds int

	// Running program, nil if none
	bin    string
	cancel context.CancelFunc
	done   chan struct{}
}

// Run builds the package, killed by the watcher if new changes arrive while
// building. Building to a new file each time keeps the running binary intact,
// which is required on windows.
func (r *runner) Run(ctx context.Context, ev etc.Event) error {
	r.builds++
	bin := filepath.Join(r.dir, fmt.Sprintf("%s-%d%s", r.name, r.builds, exeSuffix()))
	args := slices.Concat([]string{"build", "-o", bin}, r.flags, []string{r.pkg})

	err := etc.Exec("go", args...).Run(ctx, keep)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		_ = os.Remove(bin)
		return err
	}

	r.restart(bin)
	return nil
}

// restart stops the running program, if any, and starts bin.
func (r *runner) restart(bin string) {
	r.stop()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.bin, r.cancel, r.done = bin, cancel, done

	r.logger.Printf("%s %s", fmtx.Cyan("start"), r.pkg)
	go func() {
		defer close(done)
		err := etc.Exec(bin, r.args...).Run(ctx, keep)
		switch {
		case ctx.Err() != nil:
			// Stopped for a restart or shutdown
		case err != nil:
			r.logger.Printf("%s %s %s", fmtx.Red("exited"), r.pkg, fmtx.Dim(err.Error()))
		default:
			r.logger.Printf("%s %s", fmtx.Yellow("exited"), r.pkg)
		}
	}()
}

// stop kills the running program, waiting for it to exit.
func (r *runner) stop() {
	if r.cancel == nil {
		return
	}
	r.cancel()
	<-r.done
	_ = os.Remove(r.bin)
	r.bin, r.cancel, r.done = "", nil, nil
}

// splitArgs splits the arguments at the first `--`, returning the options
// before it and the program arguments after it.
func splitArgs(args []string) ([]string, []string) {
	i := slices.Index(args, "--")
	if i < 0 {
		return args, nil
	}
	return args[:i], args[i+1:]
}

// binName names the binary after the package directory, like go build does.
func binName(pkg string) string {
	if fsx.IsDir(pkg) {
		if abs, err := filepath.Abs(pkg); err == nil {
			return filepath.Base(abs)
		}
	}
	return path.Base(pkg)
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
