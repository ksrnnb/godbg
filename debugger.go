package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
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
	symTable       *SymbolTable
	logger         *slog.Logger
}

const MainFunctionSymbol = "main.main"

const (
	// you can see signal code by "cat /usr/include/asm-generic/siginfo.h"
	SignalCodeTrapBreakpoint = 1
	SignalCodeTrapTrace      = 2

	SignalCodeKernel = 0x80
)

func NewDebugger(debuggeePath string, logger *slog.Logger) (*Debugger, error) {
	pid, err := executeDebuggeeProcess()
	if err != nil {
		return nil, err
	}

	symTable, err := NewSymbolTable(debuggeePath)
	if err != nil {
		return nil, err
	}

	return &Debugger{
		pid:            pid,
		breakpoints:    make(map[uint64]Breakpoint),
		registerClient: NewRegisterClient(pid),
		debuggeePath:   debuggeePath,
		symTable:       symTable,
		logger:         logger,
	}, nil
}

func (d *Debugger) HandleCommand(cmd Command) error {
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
	case SingleStepInstructionCommand:
		if err := d.handleSingleStepInstructionCommand(); err != nil {
			fmt.Printf("failed to handle single step instruction: %s\n", err)
		}
	case StepOutCommand:
		if err := d.handleStepOutCommand(); err != nil {
			fmt.Printf("failed to handle step out command: %s\n", err)
		}
	case StepInCommand:
		if err := d.handleStepInCommand(); err != nil {
			fmt.Printf("failed to handle step in comand: %s\n", err)
		}
	case NextCommand:
		if err := d.handleNextCommand(); err != nil {
			fmt.Printf("failed to handle next command: %s\n", err)
		}
	default:
		return nil
	}

	return nil
}

func (d *Debugger) WaitSignal() (syscall.Signal, error) {
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
		d.logger.Debug("Process received signal", "signal", ws.Signal())
		return ws.Signal(), nil
	}

	if ws.Stopped() {
		d.logger.Debug("Process stopped with sigal", "signal", ws.StopSignal(), "cause", ws.TrapCause())
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

	d.logger.Debug("stop by signal", "number", sigInfo.Signo, "code", sigInfo.Code)

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

	fmt.Printf("hit breakpoint at address: 0x%x\n", newPC)

	return d.printSourceCode()
}

func (d *Debugger) getPC() (uint64, error) {
	return d.registerClient.GetRegisterValue(Rip)
}

func (d *Debugger) setPC(pc uint64) error {
	return d.registerClient.SetRegisterValue(Rip, pc)
}

func (d *Debugger) setBreakpoint(addr uint64) {
	bp := NewBreakpoint(d.pid, addr)
	bp.Enable()

	fmt.Printf("set breakpoint at address: 0x%x\n", addr)
	d.breakpoints[addr] = bp
}

func (d *Debugger) setBreakpointAtFunction(funcname string) error {
	fn, err := d.symTable.LookupFunc(funcname)
	if err != nil {
		return err
	}

	peAddr, err := d.symTable.GetPrologueEndAddress(fn)
	if err != nil {
		return err
	}

	d.setBreakpoint(peAddr)
	return nil
}

func (d *Debugger) setBreakpointAtLine(filename string, line int) error {
	addr, err := d.symTable.GetNewStatementAddrByLine(filename, line)
	if err != nil {
		return err
	}

	d.setBreakpoint(addr)

	return nil
}

func (d *Debugger) removeBreakpoint(addr uint64) {
	bp, ok := d.breakpoints[addr]
	if ok {
		// breakpoint must be disabled before delete it from map
		bp.Disable()
	}

	delete(d.breakpoints, addr)
}

func (d *Debugger) singleStepInstruction() error {
	pc, err := d.getPC()
	if err != nil {
		return err
	}

	_, ok := d.breakpoints[pc]
	if ok {
		return d.stepOverBreakpointIfNeeded()
	}

	// if breakpoint is not exist,
	if err := sys.PtraceSingleStep(d.pid); err != nil {
		return err
	}

	_, err = d.WaitSignal()
	return err
}

func (d *Debugger) stepOverBreakpointIfNeeded() error {
	pc, err := d.getPC()
	if err != nil {
		return err
	}

	bp, ok := d.breakpoints[pc]
	if !ok {
		d.logger.Debug("break point is not found", "address", fmt.Sprintf("0x%x", pc))
		return nil
	}

	if !bp.isEnabled {
		d.logger.Debug("break point is not enabled", "address", fmt.Sprintf("0x%x", pc))
		return nil
	}

	// handle breakpoint from here
	if err := bp.Disable(); err != nil {
		return err
	}

	if err := sys.PtraceSingleStep(d.pid); err != nil {
		return err
	}

	if _, err := d.WaitSignal(); err != nil {
		return err
	}

	if err := bp.Enable(); err != nil {
		return err
	}

	return nil
}

func (d *Debugger) handleContinueCommand() error {
	return d.continueInstruction()
}

func (d *Debugger) continueInstruction() error {
	if err := d.stepOverBreakpointIfNeeded(); err != nil {
		return err
	}

	if err := syscall.PtraceCont(d.pid, 0); err != nil {
		d.logger.Error("failed to cont", "error", err)
		return err
	}

	if sig, err := d.WaitSignal(); err != nil {
		return err
	} else if sig == syscall.SIGURG {
		// TODO: investigate why SIGURG is notified.
		return d.continueInstruction()
	}

	return nil
}

func (d *Debugger) handleBreakCommand(args []string) error {
	addr, err := strconv.ParseUint(args[0], 16, 64)
	if err != nil {
		// break with filename and line number when the length of arguments is 2
		if len(args) == 2 {
			line, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("line number must be number: %s", err)
			}

			return d.setBreakpointAtLine(args[0], line)
		}

		// break by function
		return d.setBreakpointAtFunction(args[0])
	}

	d.setBreakpoint(addr)
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

func (d *Debugger) handleSingleStepInstructionCommand() error {
	if err := d.singleStepInstruction(); err != nil {
		return err
	}

	return d.printSourceCode()
}

func (d *Debugger) handleStepOutCommand() error {
	rbp, err := d.registerClient.GetRegisterValue(Rbp)
	if err != nil {
		return fmt.Errorf("faield to read register in step out command: %s", err)
	}

	returnAddress, err := d.readMemory(rbp + 8)
	if err != nil {
		return err
	}

	_, ok := d.breakpoints[returnAddress]
	if !ok {
		d.setBreakpoint(returnAddress)
	}

	if err := d.continueInstruction(); err != nil {
		return err
	}

	if !ok {
		d.removeBreakpoint(returnAddress)
	}

	return nil
}

func (d *Debugger) handleStepInCommand() error {
	pc, err := d.getPC()
	if err != nil {
		return err
	}

	filename, line, _ := d.symTable.PCToLine(pc)

	if err := d.stepIn(filename, line); err != nil {
		return err
	}

	return d.printSourceCode()
}

func (d *Debugger) handleNextCommand() error {
	pc, err := d.getPC()
	if err != nil {
		return err
	}

	fn := d.symTable.PCToFunc(pc)

	_, startLine, _ := d.symTable.PCToLine(fn.Entry)
	_, endLine, _ := d.symTable.PCToLine(fn.End)
	_, line, _ := d.symTable.PCToLine(pc)

	fmt.Println(startLine, endLine, line)
	// TODO: implement next command after implement source level breakpoint
	return nil
}

func (d *Debugger) stepIn(filename string, line int) error {
	pc, err := d.getPC()
	if err != nil {
		return err
	}

	f, l, _ := d.symTable.PCToLine(pc)
	if f == filename && l == line {
		return d.stepIn(filename, line)
	}

	return nil
}

func (d *Debugger) readMemory(addr uint64) (uint64, error) {
	// data is 8 byte to store uint64 value
	data := make([]byte, 8)
	_, err := sys.PtracePeekData(d.pid, uintptr(addr), data)
	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint64(data), nil
}

func (d *Debugger) printSourceCode() error {
	pc, err := d.getPC()
	if err != nil {
		return err
	}

	filename, line, _ := d.symTable.PCToLine(pc)

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	printSourceCode(f, line)

	return nil
}

func executeDebuggeeProcess() (pid int, err error) {
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

	//	syscall.PtraceSetOptions(pid, syscall.PTRACE_O_TRACECLONE)
	if err := cmd.Start(); err != nil {
		log.Fatalf("failed to start command: %s", err)
	}

	return cmd.Process.Pid, nil
}
