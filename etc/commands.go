package etc

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// KillDelay is the time given to a process group to exit after being asked to
// terminate, before it is forcefully killed.
var KillDelay = 5 * time.Second

// Shell runs `bash -c <cmd> <pwd> <target>`, so the target is accessible as $1.
func Shell(cmd string) Command {
	return CommandFunc(func(ctx context.Context, ev Event) error {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return runProcess(ctx, exec.Command("bash", "-c", cmd, pwd, ev.Target))
	})
}

// Exec runs the program directly, without shell. `{}` in args is replaced by
// the target.
func Exec(name string, args ...string) Command {
	return CommandFunc(func(ctx context.Context, ev Event) error {
		replaced := make([]string, len(args))
		for i, a := range args {
			replaced[i] = strings.ReplaceAll(a, "{}", ev.Target)
		}
		return runProcess(ctx, exec.Command(name, replaced...))
	})
}
