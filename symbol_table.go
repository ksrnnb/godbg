package main

import (
	"debug/elf"
	"debug/gosym"
	"errors"
)

// section is described in the elf format document.
// @see http://www.skyfree.org/linux/references/ELF_Format.pdf
//
// you can see how to use gosym in pclntab_test
// @see https://cs.opensource.google/go/go/+/refs/tags/go1.22.5:src/debug/gosym/pclntab_test.go;l=86
func NewSymbolTable(debugeePath string) (*gosym.Table, error) {
	f, err := elf.Open(debugeePath)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	s := f.Section(".gosymtab")
	if s == nil {
		return nil, errors.New(".gosymtab section is not found")
	}

	symdata, err := s.Data()
	if err != nil {
		return nil, err
	}

	pclndata, err := f.Section(".gopclntab").Data()
	if err != nil {
		return nil, err
	}

	pcln := gosym.NewLineTable(pclndata, f.Section(".text").Addr)

	return gosym.NewTable(symdata, pcln)
}
