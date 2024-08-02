package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func buildDebuggeeProgram(path string) (string, error) {
	debuggeename := fmt.Sprintf("__debug_%d", time.Now().Unix())

	cmd := exec.Command("go", "build", "-o", debuggeename, "-gcflags", "all=-N -l", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdin

	if err := cmd.Run(); err != nil {
		return "", err
	}

	path, err := filepath.Abs(debuggeename)
	if err != nil {
		return "", err
	}

	return path, nil
}
