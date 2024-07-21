package main

import (
	"fmt"
	"log"

	"github.com/ksrnnb/godbg/logger"
)

const debuggee = "/home/kyota/src/godbg/hello.o"

func main() {
	l := logger.NewLogger()
	dbg, err := NewDebugger(debuggee, l)
	if err != nil {
		log.Fatalf("failed to set up debugger: %s", err)
	}

	handler := NewHandler(dbg)
	if err := handler.Run(); err != nil {
		log.Fatalf("failed to run handler: %s", err)
	}

	fmt.Println("process has been completed.")
}
