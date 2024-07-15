package main

import (
	"errors"
	"strings"
)

const (
	ContinueCommand = "continue"
	QuitCommand     = "quit"
	BreakCommand    = "break"
	RegisterCommand = "register"
	UnknownCommand  = "unknown"

	DumpSubCommand = "dump"
)

type Command struct {
	Type    string
	SubType string
	Args    []string
}

func NewCommand(input string) (Command, error) {
	s := strings.Split(input, " ")

	if len(s) == 0 || s[0] == "" {
		return Command{Type: UnknownCommand}, nil
	}

	if strings.HasPrefix(ContinueCommand, s[0]) {
		return Command{Type: ContinueCommand}, nil
	}

	if strings.HasPrefix(QuitCommand, s[0]) {
		return Command{Type: QuitCommand}, nil
	}

	if strings.HasPrefix(BreakCommand, s[0]) {
		if len(s) <= 1 {
			return Command{}, errors.New("break command must have at least 1 argument")
		}

		return Command{Type: BreakCommand, Args: s[1:]}, nil
	}

	if strings.HasPrefix(RegisterCommand, s[0]) {
		//		if len(s) <= 1 {
		//			return Command{}, errors.New("register command must have at least 1 argument")
		//		}

		//		if s[1] != DumpSubCommand {
		//			return Command{}, fmt.Errorf("unexpected register sub command '%s' is given", s[1])
		//		}

		return Command{Type: RegisterCommand, SubType: DumpSubCommand}, nil
	}

	return Command{Type: UnknownCommand}, nil
}
