//go:build windows

package providers

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// killedExitCode is the code os.Process.Kill gives a process it terminates.
const killedExitCode = 1

// tree is the job object an installer runs in, as an extension's does
// (internal/extensions): ending it ends whatever the installer started.
// once keeps the cancel and the cleanup after Wait from closing the handle
// twice.
type tree struct {
	job  windows.Handle
	once *sync.Once
}

func ownSession(*exec.Cmd) {}

// adopt puts p in a job of its own; without one, end kills p alone.
func adopt(p *os.Process) tree {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return tree{}
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return tree{}
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		_ = windows.CloseHandle(job)
		return tree{}
	}
	err = errors.Join(windows.AssignProcessToJobObject(job, process), windows.CloseHandle(process))
	if err != nil {
		_ = windows.CloseHandle(job)
		return tree{}
	}
	return tree{job: job, once: new(sync.Once)}
}

func (t tree) end(p *os.Process) error {
	if t.job == 0 {
		if p == nil {
			return nil
		}
		if err := p.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return nil
	}
	var err error
	t.once.Do(func() {
		err = errors.Join(windows.TerminateJobObject(t.job, killedExitCode), windows.CloseHandle(t.job))
	})
	return err
}

// asRoot is false: an elevated lich is the user's own choice on Windows, and
// the installers lich runs install per user either way.
func asRoot() bool { return false }
