package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renatopp/cli-etc/etc"
)

func TestShellArgs(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	cmd := etc.Shell(`echo "$0|$1" > ` + filepath.ToSlash(out))

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
