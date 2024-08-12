package main

import "fmt"

func main() {
	foo := -3
	bar := 2
	baz := foo + bar

	foo = 4

	fmt.Printf("foo: %d, bar: %d, baz: %d\n", foo, bar, baz)
}
