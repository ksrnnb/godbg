package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	sys "golang.org/x/sys/unix"
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

	var ws sys.WaitStatus
	_, err := sys.Wait4(cmd.Process.Pid, &ws, 0, nil)
	if err != nil {
		log.Fatalf("failed to wait pid %d", cmd.Process.Pid)
	}

	if ws.Exited() {
		fmt.Println("process exited")
		os.Exit(0)
	}

	pid := cmd.Process.Pid
	if err := syscall.PtraceCont(pid, 0); err != nil {
		log.Fatalf("failed to cont: %s", err)
	}

	// do something
	fmt.Println("process has been completed.")
}
