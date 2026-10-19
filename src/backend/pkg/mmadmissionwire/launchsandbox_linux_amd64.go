//go:build linux && amd64

package mmadmissionwire

import (
	"runtime"
	"syscall"
	"unsafe"
)

// DenyFurtherExec installs a process-wide seccomp filter before launch
// registration. TSYNC covers existing Go threads; new threads inherit it.
// It only narrows this owned process and requires no added capabilities.
func DenyFurtherExec() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const sysSeccomp = 317
	const setModeFilter = 1
	const filterTSync = 1
	const seccompRetAllow = 0x7fff0000
	const seccompRetErrno = 0x00050000
	const auditArchX8664 = 0xc000003e
	const sysExecveat = 322
	filter := []syscall.SockFilter{
		{Code: 0x20, K: 4}, // seccomp_data.arch
		{Code: 0x15, Jt: 1, K: auditArchX8664},
		{Code: 0x06, K: 0x80000000},        // kill on unexpected ABI
		{Code: 0x20, K: 0},                 // syscall number
		{Code: 0x45, Jt: 3, K: 0x40000000}, // deny the x32 syscall ABI
		{Code: 0x15, Jt: 2, K: syscall.SYS_EXECVE},
		{Code: 0x15, Jt: 1, K: sysExecveat},
		{Code: 0x06, K: seccompRetAllow},
		{Code: 0x06, K: seccompRetErrno | uint32(syscall.EPERM)},
	}
	if _, _, e := syscall.RawSyscall6(syscall.SYS_PRCTL, 38, 1, 0, 0, 0, 0); e != 0 {
		return e
	}
	program := syscall.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	result, _, e := syscall.RawSyscall(sysSeccomp, setModeFilter, filterTSync, uintptr(unsafe.Pointer(&program)))
	if e != 0 {
		return e
	}
	if result != 0 {
		return syscall.EPERM
	}
	return nil
}
