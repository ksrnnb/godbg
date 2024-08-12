package main

import (
	"debug/dwarf"
	"debug/elf"
	"debug/gosym"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/ksrnnb/godbg/frame"
)

type SymbolTable struct {
	table            *gosym.Table
	dwarfData        *dwarf.Data
	runtimeETextAddr uint64
	frameEntries     frame.FrameDescriptionEntries
}

type Variable struct {
	Offset  int64
	Address uint64
	Name    string
	Type    string
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

	var runtimeETextAddr uint64
	symbols, _ := f.Symbols()
	for _, s := range symbols {
		if s.Name == "runtime.etext" {
			runtimeETextAddr = s.Value
		}
	}

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

	frameSection := f.Section(".debug_frame")
	if frameSection == nil {
		return nil, errors.New(".debug_frame section is not found")
	}
	debugFrame, err := frameSection.Data()
	if err != nil {
		return nil, err
	}
	frameEntries := frame.Parse(debugFrame)

	return &SymbolTable{
		table:            table,
		dwarfData:        dwarfData,
		runtimeETextAddr: runtimeETextAddr,
		frameEntries:     frameEntries,
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
	addr, fn, err := st.table.LineToPC(filename, line)
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

		var lineEntry dwarf.LineEntry

		for {
			if err := lineReader.Next(&lineEntry); err != nil {
				break
			}

			if lineEntry.File.Name != filename {
				continue
			}

			if lineEntry.Address == addr && lineEntry.IsStmt {
				if lineEntry.Address != fn.Entry {
					return lineEntry.Address, nil
				}

				// if address is func entry, it is not prologue end
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

func (st *SymbolTable) GetCurrentFuncLowPCAndHighPC(pc uint64) (lowPC uint64, highPC uint64, err error) {
	reader := st.dwarfData.Reader()
	for {
		entry, err := reader.Next()
		if err != nil {
			break
		}

		if entry == nil {
			break
		}

		if entry.Tag != dwarf.TagSubprogram {
			continue
		}

		lowPC, _ = entry.Val(dwarf.AttrLowpc).(uint64)
		highPC, _ = entry.Val(dwarf.AttrHighpc).(uint64)

		if pc >= lowPC && pc <= highPC {
			return lowPC, highPC, nil
		}
	}

	return 0, 0, nil
}

func (st *SymbolTable) GetCurrentFuncStartToEndLine(pc uint64) (startLine int, endLine int, err error) {
	filename, _, _ := st.PCToLine(pc)
	lowPC, highPC, err := st.GetCurrentFuncLowPCAndHighPC(pc)
	if err != nil {
		return 0, 0, err
	}

	reader := st.dwarfData.Reader()
	for {
		entry, err := reader.Next()
		if err != nil {
			break
		}

		if entry == nil {
			break
		}

		if entry.Tag != dwarf.TagCompileUnit {
			continue
		}

		lineReader, err := st.dwarfData.LineReader(entry)
		if err != nil {
			return 0, 0, err
		}

		var lineEntry dwarf.LineEntry

		startLine = math.MaxInt
		endLine = -1

		for {
			if err := lineReader.Next(&lineEntry); err != nil {
				break
			}

			if lineEntry.File.Name != filename {
				continue
			}

			if !lineEntry.IsStmt {
				continue
			}

			if lineEntry.Address >= lowPC && lineEntry.Address <= highPC {
				if lineEntry.Line < startLine {
					startLine = lineEntry.Line
				}

				if lineEntry.Line > endLine {
					endLine = lineEntry.Line
				}
			}
		}

		if startLine != math.MaxInt {
			return startLine, endLine, nil
		}
	}

	return 0, 0, fmt.Errorf("failed to find start line and end line for pc %0x", pc)
}

func (st *SymbolTable) GetFuncInfo(pc uint64) (funcName string, filename string, line int) {
	filename, line, fn := st.table.PCToLine(pc)

	return fn.Name, filename, line
}

func (st *SymbolTable) GetRuntimeETextAddress() uint64 {
	return st.runtimeETextAddr
}

func (st *SymbolTable) GetVariables(pc uint64, sp uint64) (variables []Variable, err error) {
	reader, err := st.seekToFunction(pc)
	if err != nil {
		return nil, err
	}

	for entry, err := reader.Next(); entry != nil; entry, err = reader.Next() {
		if err != nil {
			return nil, err
		}

		if entry.Tag == dwarf.TagVariable {
			name, _ := entry.Val(dwarf.AttrName).(string)

			instructions, _ := entry.Val(dwarf.AttrLocation).([]byte)

			offset, _ := entry.Val(dwarf.AttrType).(dwarf.Offset)

			t, err := st.dwarfData.Type(offset)
			if err != nil {
				return nil, err
			}

			// TODO: get arbitrary variables and specify scope
			targets := []string{"foo", "bar", "baz"}
			if slices.Contains(targets, name) {
				fde, err := st.frameEntries.FDEForPC(pc)
				var cfa int64
				if err == nil {
					fctx := fde.EstablishFrame(pc)
					cfa = fctx.CFAOffset() + int64(sp)
				}
				addr, err := frame.ExecuteStackProgram(cfa, instructions)
				if err != nil {
					return nil, err
				}

				variables = append(variables, Variable{Name: name, Address: uint64(addr), Type: t.String()})
			}
		}
	}

	return variables, nil
}

func (st *SymbolTable) seekToFunction(pc uint64) (*dwarf.Reader, error) {
	reader := st.dwarfData.Reader()
	reader.Seek(0)

	for entry, err := reader.Next(); entry != nil; entry, err = reader.Next() {
		if err != nil {
			return nil, err
		}

		lowpc, ok := entry.Val(dwarf.AttrLowpc).(uint64)
		if !ok {
			continue
		}

		highpc, ok := entry.Val(dwarf.AttrHighpc).(uint64)
		if !ok {
			continue
		}

		if lowpc <= pc && highpc > pc {
			return reader, nil
		}
	}

	return nil, fmt.Errorf("faield to seek to function for pc: %x", pc)
}
