//go:build windows

package extensions

import (
	"errors"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// killedExitCode is the code os.Process.Kill gives a process it terminates.
const killedExitCode = 1

// tree is the job object an extension runs in; what it starts joins it, unless
// another process starts it on its behalf. Closing the job's last handle,
// which lich's own exit also does, ends every process in it.
type tree struct{ job windows.Handle }

func ownGroup(*exec.Cmd) {}

func adopt(p *os.Process) (tree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return tree{}, err
	}
	if err := assign(job, p.Pid); err != nil {
		return tree{}, errors.Join(err, windows.CloseHandle(job))
	}
	return tree{job: job}, nil
}

func assign(job windows.Handle, pid int) error {
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	return errors.Join(windows.AssignProcessToJobObject(job, process), windows.CloseHandle(process))
}

func (t tree) end() error {
	if t.job == 0 {
		return nil
	}
	return errors.Join(windows.TerminateJobObject(t.job, killedExitCode), windows.CloseHandle(t.job))
}
