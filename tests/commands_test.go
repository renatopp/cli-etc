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

func TestShellArgs(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	cmd := etc.Shell(`echo "$0|$1" > ` + out)

	if err := cmd.Run(context.Background(), etc.Event{Target: "some/target"}); err != nil {
		t.Fatal(err)
	}

	pwd, _ := os.Getwd()
	data, _ := os.ReadFile(out)
	equal(t, pwd+"|some/target", strings.TrimSpace(string(data)))
}

func TestShellError(t *testing.T) {
	if err := etc.Shell("exit 3").Run(context.Background(), etc.Event{}); err == nil {
		t.Fatal("expected error")
	}
}

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

func TestExecReplacesTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "created")
	if err := etc.Exec("touch", "{}").Run(context.Background(), etc.Event{Target: target}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}
}

func TestParseTarget(t *testing.T) {
	for _, target := range []etc.Target{etc.BaseTarget, etc.DirTarget, etc.FileTarget} {
		parsed, err := etc.ParseTarget(target.String())
		if err != nil || parsed != target {
			t.Fatalf("expected %v, got %v (%v)", target, parsed, err)
		}
	}
	if _, err := etc.ParseTarget("other"); err == nil {
		t.Fatal("expected error")
	}
}
