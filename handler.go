package main

import (
	"bufio"
	"fmt"
	"os"
)

type Handler struct {
	d *Debugger
}

func NewHandler(d *Debugger) *Handler {
	return &Handler{d: d}
}

func (h *Handler) Run() error {
	if _, err := h.d.WaitSignal(); err != nil {
		return err
	}

	sc := bufio.NewScanner(os.Stdin)
	fmt.Print("godbg> ")

	for sc.Scan() {
		s := sc.Text()

		cmd, err := NewCommand(s)
		if err != nil {
			fmt.Printf("failed to parse command: %s\n", err)
			continue
		}

		if err := h.d.HandleCommand(cmd); err != nil {
			return err
		}

		fmt.Printf("\ngodbg> ")
	}

	return nil
}
