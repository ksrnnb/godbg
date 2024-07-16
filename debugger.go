package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	sys "golang.org/x/sys/unix"
)

type Debugger struct {
	pid            int
	offset         uint64
	breakpoints    map[uint64]Breakpoint
	registerClient RegisterClient
}

const MainFunctionSymbol = "main.main"

var mainReg = regexp.MustCompile(`^<([^>]+)>:$`)

func getOffset(pid int) (uint64, error) {
	filePath := fmt.Sprintf("/proc/%d/maps", pid)
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()
		s := strings.Split(line, "-")
		return strconv.ParseUint(s[0], 16, 64)
	}

	return 0, fmt.Errorf("failed to get offset for pid %d", pid)
}

func NewDebugger(pid int, debuggeePath string) (Debugger, error) {
	offset, err := getOffset(pid)
	if err != nil {
		return Debugger{}, err
	}

	fmt.Printf("offset is %x\n", offset)

	return Debugger{
		pid:            pid,
		offset:         offset,
		breakpoints:    make(map[uint64]Breakpoint),
		registerClient: NewRegisterClient(pid),
	}, nil
}

func (d *Debugger) Run() error {
	if _, err := d.waitSignal(); err != nil {
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
		if err := d.handleContinueCommand(); err != nil {
			return err
		}
	case QuitCommand:
		fmt.Println("quit process")
		os.Exit(0)
	case BreakCommand:
		if err := d.handleBreakCommand(cmd.Args); err != nil {
			fmt.Printf("failed to handle break command: %s\n", err)
		}
	case RegisterCommand:
		if err := d.handleRegisterCommand(cmd); err != nil {
			fmt.Printf("faield to handle register command: %s\n", err)
		}
		return nil
	default:
		return nil
	}

	return nil
}

func (d *Debugger) waitSignal() (syscall.Signal, error) {
	var ws sys.WaitStatus
	_, err := sys.Wait4(d.pid, &ws, 0, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to wait pid %d", d.pid)
	}

	if ws.Exited() {
		fmt.Println("process exited")
		os.Exit(0)
	}

	if ws.Signaled() {
		fmt.Printf("Process received signal %s\n", ws.Signal())
		return ws.Signal(), nil
	}

	if ws.Stopped() {
		fmt.Printf("Process stopped with signal: %s, cause: %d\n", ws.StopSignal(), ws.TrapCause())
		return ws.StopSignal(), nil
	}

	return 0, nil
}

func (d *Debugger) getPC() (uint64, error) {
	return d.registerClient.GetRegisterValue(Rip)
}

func (d *Debugger) setPC(pc uint64) error {
	return d.registerClient.SetRegisterValue(Rip, pc)
}

func (d *Debugger) stepOverBreakpointIfNeeded() error {
	pc, err := d.getPC()
	if err != nil {
		return err
	}

	// -1 is necessasry because PC is incremented when debugee stops at INT3 instruction
	breakpointPC := pc - 1
	bp, ok := d.breakpoints[breakpointPC]
	if !ok {
		return nil
	}

	if !bp.isEnabled {
		return nil
	}

	// handle breakpoint from here
	if err := d.setPC(breakpointPC); err != nil {
		return err
	}

	if err := bp.Disable(); err != nil {
		return err
	}

	if err := sys.PtraceSingleStep(d.pid); err != nil {
		return err
	}

	fmt.Println("single steppppppp")

	if _, err := d.waitSignal(); err != nil {
		return err
	}

	if err := bp.Enable(); err != nil {
		return err
	}

	return nil
}

func (d *Debugger) handleContinueCommand() error {
	if err := d.stepOverBreakpointIfNeeded(); err != nil {
		return err
	}

	if err := syscall.PtraceCont(d.pid, 0); err != nil {
		fmt.Printf("failed to cont: %s\n", err)
		return nil
	}

	if sig, err := d.waitSignal(); err != nil {
		return err
	} else if sig == syscall.SIGURG {
		// TODO: investigate why SIGURG is notified.
		return d.handleContinueCommand()
	}

	return nil
}

func (d *Debugger) handleBreakCommand(args []string) error {
	addr, err := strconv.ParseUint(args[0], 16, 64)
	if err != nil {
		return err
	}

	bp := NewBreakpoint(d.pid, addr)
	bp.Enable()

	fmt.Printf("set breakpoint at address 0x%x\n", addr)
	d.breakpoints[addr] = bp
	return nil
}

func (d *Debugger) handleRegisterCommand(cmd Command) error {
	switch cmd.SubType {
	case DumpSubCommand:
		if err := d.registerClient.DumpRegisters(); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("unexptected sub command %s is given", cmd.SubType)
}
