package main

import (
	"bufio"
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
	pid := cmd.Process.Pid

	_, err := sys.Wait4(pid, &ws, 0, nil)
	if err != nil {
		log.Fatalf("failed to wait pid %d", cmd.Process.Pid)
	}

	if ws.Exited() {
		fmt.Println("process exited")
		os.Exit(0)
	}

	sc := bufio.NewScanner(os.Stdin)
	fmt.Print("godbg> ")

	for sc.Scan() {
		fmt.Printf("godbg> ")
		s := sc.Text()

		if s == "c" {
			if err := syscall.PtraceCont(pid, 0); err != nil {
				log.Fatalf("failed to cont: %s", err)
			}
		} else if s == "q" {
			// TODO: if child process is not terminated, wait4 waits forever...
			break
		}
	}

	_, err = sys.Wait4(pid, &ws, 0, nil)
	if err != nil {
		log.Fatalf("failed to wait pid %d", cmd.Process.Pid)
	}

	if !ws.Exited() {
		log.Fatalf("unexpected wait status %d", ws)
	}

	fmt.Println("process has been completed.")
}
