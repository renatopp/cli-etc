//go:build unix

package etc

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// KillDelay is the time given to a process group to exit after SIGTERM,
// before SIGKILL is sent.
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

// runProcess runs cmd in its own process group, killing the whole group when
// ctx is cancelled.
func runProcess(ctx context.Context, cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		pgid := -cmd.Process.Pid
		_ = syscall.Kill(pgid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(KillDelay):
			_ = syscall.Kill(pgid, syscall.SIGKILL)
			<-done
		}
		// Leftovers of the group that ignored SIGTERM
		_ = syscall.Kill(pgid, syscall.SIGKILL)
		return ctx.Err()
	}
}
