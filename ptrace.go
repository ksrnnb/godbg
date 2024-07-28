package main

import (
	sys "golang.org/x/sys/unix"
)

func ptraceSingleStep(pid int, sig int) error {
	_, _, e1 := sys.Syscall6(sys.SYS_PTRACE, uintptr(sys.PTRACE_SINGLESTEP), uintptr(pid), uintptr(0), uintptr(sig), 0, 0)
	if e1 != 0 {
		return e1
	}
	return nil
}
