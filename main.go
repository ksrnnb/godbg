package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	cmd := exec.Command("/home/kyota/src/godbg/hello")

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Ptrace: true,
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("failed to start command: %s", err)
	}

	dbg, err := NewDebugger(cmd.Process.Pid)
	if err != nil {
		log.Fatalf("failed to set up debugger: %s", err)
	}

	if err := dbg.Run(); err != nil {
		log.Fatalf("failed to run debugger: %s", err)
	}

	fmt.Println("process has been completed.")
}
