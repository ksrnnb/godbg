package main

import (
	"bufio"
	"fmt"
	"io"
)

// how many lines from given line number
const lineRange = 5

func printSourceCode(reader io.Reader, line int) {
	scanner := bufio.NewScanner(reader)

	startLine := 1
	if line > lineRange {
		startLine = line - lineRange
	}
	endLine := line + lineRange

	currentLine := 1
	var lines []string
	for scanner.Scan() {
		if currentLine < startLine {
			currentLine++
			continue
		}
		if currentLine > endLine {
			break
		}

		text := scanner.Text()
		if currentLine == line {
			text = fmt.Sprintf("> %s", text)
		} else {
			text = fmt.Sprintf("  %s", text)
		}
		lines = append(lines, text)
		currentLine++
	}

	for _, text := range lines {
		fmt.Printf("%s\n", text)
	}
}
