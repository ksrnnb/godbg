package main

import (
	"bufio"
	"debug/gosym"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	sys "golang.org/x/sys/unix"
)

type Debugger struct {
	pid            int
	offset         uint64
	breakpoints    map[uint64]Breakpoint
	registerClient RegisterClient
	debuggeePath   string
	symTable       *gosym.Table
}

const MainFunctionSymbol = "main.main"

const (
	// you can see signal code by "cat /usr/include/asm-generic/siginfo.h"
	SignalCodeTrapBreakpoint = 1
	SignalCodeTrapTrace      = 2

	SignalCodeKernel = 0x80
)

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

	symTable, err := NewSymbolTable(debuggeePath)
	if err != nil {
		return Debugger{}, nil
	}

	return Debugger{
		pid:            pid,
		offset:         offset,
		breakpoints:    make(map[uint64]Breakpoint),
		registerClient: NewRegisterClient(pid),
		debuggeePath:   debuggeePath,
		symTable:       symTable,
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
		return ws.Signal(), nil
	}

	if ws.Stopped() {
		if err := d.handleStopSignal(); err != nil {
			return ws.StopSignal(), err
		}
		return ws.StopSignal(), nil
	}

	return 0, nil
}

func (d *Debugger) handleStopSignal() error {
	var sigInfo sys.Siginfo
	_, _, errno := syscall.Syscall6(uintptr(syscall.SYS_PTRACE), uintptr(sys.PTRACE_GETSIGINFO), uintptr(d.pid), 0, uintptr(unsafe.Pointer(&sigInfo)), 0, 0)

	if errno != 0 {
		err := sys.Errno(errno)
		return fmt.Errorf("failed to get siginfo: %s", err)
	}

	switch sigInfo.Code {
	case SignalCodeTrapTrace:
		// When sigle step is called, sig_code will be TRAP_TRACE
	case SignalCodeTrapBreakpoint, SignalCodeKernel:
		// When breakpoint is hit, SI_KERNEL or TRAP_BRKPT signal code is sent
		if err := d.handleHitBreakpoint(); err != nil {
			return err
		}
	}

	return nil
}

func (d *Debugger) handleHitBreakpoint() error {
	pc, err := d.getPC()
	if err != nil {
		return err
	}

	newPC := pc - 1
	if err := d.setPC(newPC); err != nil {
		return err
	}

	fmt.Printf("hit breakpoint at address 0x%x\n", newPC)

	// TODO: print source file and line
	filename, line, _ := d.symTable.PCToLine(newPC)

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	printSourceCode(f, line)
	return nil
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

	bp, ok := d.breakpoints[pc]
	if !ok {
		return nil
	}

	if !bp.isEnabled {
		return nil
	}

	// handle breakpoint from here
	if err := bp.Disable(); err != nil {
		return err
	}

	if err := sys.PtraceSingleStep(d.pid); err != nil {
		return err
	}

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
