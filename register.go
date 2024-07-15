package main

import (
	"fmt"
	"reflect"

	sys "golang.org/x/sys/unix"
)

type Register string

const (
	R15      Register = "R15"
	R14      Register = "R14"
	R13      Register = "R13"
	R12      Register = "R12"
	Rbp      Register = "Rbp"
	Rbx      Register = "Rbx"
	R11      Register = "R11"
	R10      Register = "R10"
	R9       Register = "R9"
	R8       Register = "R8"
	Rax      Register = "Rax"
	Rcx      Register = "Rcx"
	Rdx      Register = "Rdx"
	Rsi      Register = "Rsi"
	Rdi      Register = "Rdi"
	Orig_rax Register = "Orig_rax"
	Rip      Register = "Rip"
	Cs       Register = "Cs"
	Eflags   Register = "Eflags"
	Rsp      Register = "Rsp"
	Ss       Register = "Ss"
	Fs_base  Register = "Fs_base"
	Gs_base  Register = "Gs_base"
	Ds       Register = "Ds"
	Es       Register = "Es"
	Fs       Register = "Fs"
	Gs       Register = "Gs"
)

type RegisterClient struct {
	pid int
}

func NewRegisterClient(pid int) RegisterClient {
	return RegisterClient{pid: pid}
}

func (c RegisterClient) GetRegisterValue(register Register) (uint64, error) {
	var regs sys.PtraceRegs
	if err := sys.PtraceGetRegs(c.pid, &regs); err != nil {
		return 0, err
	}

	v := reflect.ValueOf(regs).Elem()
	field := v.FieldByName(string(register))
	if !field.IsValid() {
		return 0, fmt.Errorf("no '%s' field in sys.PtraceRegs", register)
	}
	if field.Kind() != reflect.Uint64 {
		return 0, fmt.Errorf("field %s is not of type uint64", register)
	}

	return field.Uint(), nil
}

func (c RegisterClient) SetRegisterValue(register Register, value uint64) error {
	var regs sys.PtraceRegs
	if err := sys.PtraceGetRegs(c.pid, &regs); err != nil {
		return err
	}

	v := reflect.ValueOf(regs).Elem()
	field := v.FieldByName(string(register))
	if !field.IsValid() {
		return fmt.Errorf("no '%s' field in sys.PtraceRegs", register)
	}
	if field.Kind() != reflect.Uint64 {
		return fmt.Errorf("field %s is not of type uint64", register)
	}
	if !field.CanSet() {
		return fmt.Errorf("field %s cannot set", register)
	}
	field.SetUint(value)

	return sys.PtraceSetRegs(c.pid, &regs)
}

func (c RegisterClient) DumpRegisters() error {
	regs := &sys.PtraceRegs{}
	if err := sys.PtraceGetRegs(c.pid, regs); err != nil {
		return fmt.Errorf("failed to get regs for pid %d: %s", c.pid, err)
	}

	v := reflect.ValueOf(regs).Elem()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fmt.Printf("%s: 0x%x\n", v.Type().Field(i).Name, field.Uint())
	}

	return nil
}
