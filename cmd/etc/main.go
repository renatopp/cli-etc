package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/renatopp/cli-etc/etc"
	"github.com/renatopp/go-cli"
	"github.com/renatopp/go-x/fmtx"
)

const cliDescription = `
Watch files and execute commands on changes.

The command is executed as 'bash -c <command> <pwd> <target>', so the target
(base, parent directory or file, depending on --per) is accessible as $1.

More information, issues and contributions: http://github.com/renatopp/cli-etc
`

func main() {
	args, command := splitCommand(os.Args[1:])

	cli.Name("etc")
	cli.Description(cliDescription)
	cli.AutoHelp(true)
	cli.Example("etc '**/*.go' -- go test ./...", "Run tests when any go file changes.")
	cli.Example("etc -b assets -p dir '**/*.png' -- pack-atlas $1", "Pack each directory with changed images.")
	cli.Example("etc -i -k -p base '**/*.go' -- go run .", "Run and restart the program on changes.")

	patterns := cli.Pos("patterns", "Glob patterns relative to base, ** supported.").AsVariadic().AsRequired()
	base := cli.Flag("base", "b", "Base directory.").WithDefault(".")
	per := cli.FlagFunc("per", "p", "Target of the execution: base, dir or file.", etc.ParseTarget).WithDefault(etc.FileTarget)
	delay := cli.FlagInt("delay", "d", "Window of accumulating changes before execution, in milliseconds.").WithDefault(500)
	exclude := cli.Flag("exclude", "e", "Patterns to be ignored. Repeatable.").AsRepeatable()
	initial := cli.FlagBool("initial", "i", "Execute upon start.").WithDefault(false)
	kill := cli.FlagBool("kill", "k", "Kill the running process of the same target instead of waiting it.").WithDefault(false)
	parallel := cli.FlagInt("parallel", "j", "Max parallel executions.").WithDefault(etc.DefaultParallel)
	fail := cli.FlagBool("fail", "F", "Stop watching if any command fails.").WithDefault(false)
	cli.ParseArgs(args)

	if command == "" {
		cli.Fatal("missing command, use: etc [patterns...] <options...> -- <command>")
	}

	w := etc.Watcher{
		Base:     base.Value(),
		Parallel: parallel.Value(),
		Fail:     fail.Value(),
		Logger:   log.New(os.Stderr, fmtx.Dim("[etc] "), 0),
		Rules: []etc.Rule{{
			Patterns: patterns.Values(),
			Exclude:  exclude.Values(),
			Target:   per.Value(),
			Delay:    time.Duration(delay.Value()) * time.Millisecond,
			Initial:  initial.Value(),
			Kill:     kill.Value(),
			Command:  etc.Shell(command),
		}},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	w.Logger.Printf("watching %s in %s", strings.Join(patterns.Values(), " "), fmtx.Dim(base.Value()))
	cli.FatalIf(w.Run(ctx))
}

// splitCommand splits the arguments at the first `--`, returning the options
// before it and the command after it.
func splitCommand(args []string) ([]string, string) {
	i := slices.Index(args, "--")
	if i < 0 {
		return args, ""
	}
	return args[:i], strings.Join(args[i+1:], " ")
}
