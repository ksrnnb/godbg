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
	if err := d.waitSignal(); err != nil {
		return err
	}

	sc := bufio.NewScanner(os.Stdin)
	fmt.Print("godbg> ")

	for sc.Scan() {
		s := sc.Text()

		if err := d.handleInput(s); err != nil {
			return err
		}

		fmt.Printf("godbg> ")
	}

	return nil
}

func (d *Debugger) handleInput(input string) error {
	cmd, err := NewCommand(input)
	if err != nil {
		fmt.Printf("failed to parse command: %s\n", err)
		return nil
	}

	switch cmd.Type {
	case ContinueCommand:
		if err := syscall.PtraceCont(d.pid, 0); err != nil {
			return fmt.Errorf("failed to cont: %s", err)
		}

		if err := d.waitSignal(); err != nil {
			return err
		}
	case QuitCommand:
		fmt.Println("quit process")
		os.Exit(0)
	case BreakCommand:
		fmt.Printf("break command with %s\n", cmd.Args[0])
	default:
		return nil
	}

	return nil
}

func (d *Debugger) waitSignal() error {
	var ws sys.WaitStatus

	_, err := sys.Wait4(d.pid, &ws, 0, nil)
	if err != nil {
		return fmt.Errorf("failed to wait pid %d", d.pid)
	}

	if ws.Exited() {
		fmt.Println("process exited")
		os.Exit(0)
	}

	return nil
}
