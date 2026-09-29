//go:build unix

package etc

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"
)

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
