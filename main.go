package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	sys "golang.org/x/sys/unix"
)

const debuggee = "/home/kyota/src/godbg/hello"

func main() {
	pid, err := execChildProcess()
	if err != nil {
		log.Fatalf("failed to execute child process")
	}

	dbg, err := NewDebugger(pid, debuggee)
	if err != nil {
		log.Fatalf("failed to set up debugger: %s", err)
	}

	if err := dbg.Run(); err != nil {
		log.Fatalf("failed to run debugger: %s", err)
	}

	fmt.Println("process has been completed.")
}

func execChildProcess() (pid int, err error) {
	cmd := exec.Command(debuggee)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Ptrace: true,
	}

	// set personality not to randomize address
	// this code is based on delve(https://github.com/go-delve/delve/tree/v1.22.1).
	// Copyright (c) 2014 Derek Parker
	// MIT LICENSE: https://github.com/go-delve/delve/blob/v1.22.1/LICENSE
	var personalityGetPersonality uintptr = 0xffffffff // argument to pass to personality syscall to get the current personality
	var _ADDR_NO_RANDOMIZE uintptr = 0x0040000         // ADDR_NO_RANDOMIZE linux constant

	oldPersonality, _, err := syscall.Syscall(sys.SYS_PERSONALITY, personalityGetPersonality, 0, 0)
	if err == syscall.Errno(0) {
		newPersonality := oldPersonality | _ADDR_NO_RANDOMIZE
		syscall.Syscall(sys.SYS_PERSONALITY, newPersonality, 0, 0)
		defer syscall.Syscall(sys.SYS_PERSONALITY, oldPersonality, 0, 0)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("failed to start command: %s", err)
	}

	return cmd.Process.Pid, nil
}
