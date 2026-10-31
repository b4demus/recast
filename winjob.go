package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// killJob is a Windows job object that takes every ffmpeg down with recast,
// even when recast itself is killed.
type killJob struct{ h windows.Handle }

func newKillJob() *killJob {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(h)
		return nil
	}
	return &killJob{h}
}

func (j *killJob) add(p *os.Process) {
	if j == nil || p == nil {
		return
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		return
	}
	windows.AssignProcessToJobObject(j.h, h)
	windows.CloseHandle(h)
}
