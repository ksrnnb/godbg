package main

import (
	"debug/dwarf"
	"debug/elf"
	"debug/gosym"
	"errors"
	"fmt"
)

type SymbolTable struct {
	table     *gosym.Table
	dwarfData *dwarf.Data
}

// section is described in the elf format document.
// @see http://www.skyfree.org/linux/references/ELF_Format.pdf
//
// you can see how to use gosym in pclntab_test
// @see https://cs.opensource.google/go/go/+/refs/tags/go1.22.5:src/debug/gosym/pclntab_test.go;l=86
func NewSymbolTable(debugeePath string) (*SymbolTable, error) {
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

	table, err := gosym.NewTable(symdata, pcln)
	if err != nil {
		return nil, err
	}

	dwarfData, err := f.DWARF()
	if err != nil {
		return nil, err
	}

	return &SymbolTable{
		table:     table,
		dwarfData: dwarfData,
	}, nil
}

func (st *SymbolTable) PCToLine(pc uint64) (file string, line int, fn *gosym.Func) {
	return st.table.PCToLine(pc)
}

func (st *SymbolTable) PCToFunc(pc uint64) *gosym.Func {
	return st.table.PCToFunc(pc)
}

func (st *SymbolTable) LookupFunc(funcname string) (*gosym.Func, error) {
	fn := st.table.LookupFunc(funcname)
	if fn == nil {
		return nil, fmt.Errorf("failed to look up function: %s", funcname)
	}

	return fn, nil
}

func (st *SymbolTable) GetPrologueEndAddress(fn *gosym.Func) (uint64, error) {
	reader := st.dwarfData.Reader()
	for {
		entry, err := reader.Next()
		if err != nil {
			break
		}

		if entry.Tag != dwarf.TagCompileUnit {
			continue
		}

		lineReader, err := st.dwarfData.LineReader(entry)
		if err != nil {
			return 0, err
		}

		var lineEntry dwarf.LineEntry
		for {
			if err := lineReader.Next(&lineEntry); err != nil {
				break
			}

			if lineEntry.Address == fn.Entry {
				for err := lineReader.Next(&lineEntry); err == nil; {
					if lineEntry.PrologueEnd {
						return lineEntry.Address, nil
					}
				}
			}
		}

	}

	return 0, fmt.Errorf("faield to get prologue end address for function %s", fn.Name)
}

func (st *SymbolTable) GetNewStatementAddrByLine(filename string, line int) (uint64, error) {
	addr, _, err := st.table.LineToPC(filename, line)
	if err != nil {
		return 0, fmt.Errorf("failed to get addr by filename %s and line %d: %s", filename, line, err)
	}

	reader := st.dwarfData.Reader()
	for {
		entry, err := reader.Next()
		if err != nil {
			break
		}

		if entry.Tag != dwarf.TagCompileUnit {
			continue
		}

		lineReader, err := st.dwarfData.LineReader(entry)
		if err != nil {
			return 0, err
		}

		lowPC := entry.AttrField(dwarf.AttrLowpc).Val.(uint64)
		var lineEntry dwarf.LineEntry

		for {
			if err := lineReader.Next(&lineEntry); err != nil {
				break
			}

			if lineEntry.File.Name != filename {
				continue
			}

			if lineEntry.Address == addr && lineEntry.IsStmt {
				if lineEntry.Address != lowPC {
					return lineEntry.Address, nil
				}

				// if address is low pc, it is not prologue end
				for err := lineReader.Next(&lineEntry); err == nil; {
					if lineEntry.PrologueEnd {
						return lineEntry.Address, nil
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("failed to get NS addr for file %s and line %d", filename, line)
}
