package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ksrnnb/godbg/logger"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		log.Fatalf("debuggee path must be given")
	}

	l := logger.NewLogger()
	dbg, err := NewDebugger(args[1], l)
	if err != nil {
		log.Fatalf("failed to set up debugger: %s", err)
	}

	handler := NewHandler(dbg)
	if err := handler.Run(); err != nil {
		log.Fatalf("failed to run handler: %s", err)
	}

	fmt.Println("process has been completed.")
}
