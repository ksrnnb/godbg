package main

import (
	"bufio"
	"fmt"
	"os"
	"syscall"

	sys "golang.org/x/sys/unix"
)

type Debugger struct {
	pid int
}

func NewDebugger(pid int) Debugger {
	return Debugger{pid: pid}
}

func (d *Debugger) Run() error {
	var ws sys.WaitStatus

	_, err := sys.Wait4(d.pid, &ws, 0, nil)
	if err != nil {
		return fmt.Errorf("failed to wait pid %d", d.pid)
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
			if err := syscall.PtraceCont(d.pid, 0); err != nil {
				return fmt.Errorf("failed to cont: %s", err)
			}
		} else if s == "q" {
			// TODO: if child process is not terminated, wait4 waits forever...
			break
		}
	}

	_, err = sys.Wait4(d.pid, &ws, 0, nil)
	if err != nil {
		return fmt.Errorf("failed to wait pid %d", d.pid)
	}

	if !ws.Exited() {
		return fmt.Errorf("unexpected wait status %d", ws)
	}

	return nil
}
