package main

import (
	"encoding/binary"

	sys "golang.org/x/sys/unix"
)

const Int3Instruction = 0xcc

type Breakpoint struct {
	pid                 int
	addr                uintptr
	originalInstruction []byte
	isEnabled           bool
}

func NewBreakpoint(pid int, addr uint64) *Breakpoint {
	return &Breakpoint{pid: pid, addr: uintptr(addr), originalInstruction: make([]byte, 8)}
}

func (bp *Breakpoint) Enable() error {
	_, err := sys.PtracePeekData(bp.pid, bp.addr, bp.originalInstruction)
	if err != nil {
		return err
	}

	data := binary.LittleEndian.Uint64(bp.originalInstruction)
	// data & ^0xff => data & 11111111 11111111 11111111 00000000
	newData := (data & ^uint64(0xff)) | Int3Instruction
	newInstruction := make([]byte, 8)
	binary.LittleEndian.PutUint64(newInstruction, newData)

	_, err = sys.PtracePokeData(bp.pid, bp.addr, newInstruction)
	if err != nil {
		return err
	}

	bp.isEnabled = true
	return nil
}

func (bp *Breakpoint) Disable() error {
	_, err := sys.PtracePokeData(bp.pid, bp.addr, bp.originalInstruction)
	if err != nil {
		return err
	}

	bp.isEnabled = false
	return nil
}

func (bp *Breakpoint) IsEnabled() bool {
	return bp.isEnabled
}
