package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	sys "golang.org/x/sys/unix"
)

type Debugger struct {
	pid    int
	offset uint64
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

func getMainAddress(path string) (uint64, error) {
	cmd := exec.Command("objdump", "-d", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		s := strings.Split(line, " ")
		if len(s) <= 1 {
			continue
		}

		match := mainReg.FindStringSubmatch(s[1])

		if len(match) <= 1 {
			continue
		}

		if match[1] != MainFunctionSymbol {
			continue
		}

		return strconv.ParseUint(s[0], 16, 64)
	}

	return 0, fmt.Errorf("failed to get main function address for %s", path)
}

func NewDebugger(pid int, debuggeePath string) (Debugger, error) {
	offset, err := getOffset(pid)
	if err != nil {
		return Debugger{}, err
	}

	fmt.Printf("offset is %x\n", offset)
	mainAddr, err := getMainAddress(debuggeePath)
	if err != nil {
		return Debugger{}, err
	}

	fmt.Printf("main address is %x\n", mainAddr)

	return Debugger{pid: pid, offset: offset}, nil
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
