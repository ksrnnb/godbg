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

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Ptrace: true,
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("failed to start command: %s", err)
	}


	if err := cmd.Wait(); err != nil {
		log.Fatalf("failed to wait process: %s", err)
	}


	pid := cmd.Process.Pid
	syscall.PtraceCont(pid, 0)

	// do something
	fmt.Println("process has been completed.")
}
