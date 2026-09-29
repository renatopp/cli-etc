//go:build windows

package etc

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runProcess runs cmd in its own process group and job object, killing the
// whole tree when ctx is cancelled. Cancellation sends CTRL_BREAK first, which
// Go programs receive as os.Interrupt.
func runProcess(ctx context.Context, cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}

	job, err := newJob()
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)

	if err := cmd.Start(); err != nil {
		return err
	}
	// Children spawned before the assignment escape the job, which is
	// acceptable as processes rarely fork right at startup
	_ = assignJob(job, cmd.Process.Pid)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		pid := uint32(cmd.Process.Pid)
		if windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, pid) != nil {
			_ = windows.TerminateJobObject(job, 1)
		}
		select {
		case <-done:
		case <-time.After(KillDelay):
			_ = windows.TerminateJobObject(job, 1)
			<-done
		}
		// Leftovers of the tree that ignored CTRL_BREAK
		_ = windows.TerminateJobObject(job, 1)
		return ctx.Err()
	}
}

// newJob creates a job object that kills its processes when its last handle is
// closed, so nothing survives if the watcher dies.
func newJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func assignJob(job windows.Handle, pid int) error {
	p, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(p)
	return windows.AssignProcessToJobObject(job, p)
}
