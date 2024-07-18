package main

import "fmt"

func main() {
	a := 1
	b := 2
	c := a + b

	a = 4
	fmt.Printf("a: %d, b: %d, c: %d\n", a, b, c)
}
