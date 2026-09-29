//go:build unix

package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/renatopp/cli-etc/etc"
)

func TestShellKillsProcessGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	cmd := etc.Shell(`sleep 30 & echo $! > ` + pidFile + `; wait`)

	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() { errs <- cmd.Run(ctx, etc.Event{}) }()

	var pid int
	for i := 0; i < 50 && pid == 0; i++ {
		time.Sleep(20 * time.Millisecond)
		data, _ := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}
	if pid == 0 {
		t.Fatal("child pid not written")
	}

	cancel()
	select {
	case err := <-errs:
		if err == nil {
			t.Fatal("expected context error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("command was not killed")
	}

	// The background child must be gone as well
	for i := 0; i < 50; i++ {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child process %d still running", pid)
}
