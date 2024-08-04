package main

func main() {
	b()
	c()
}

func a() {
}

func b() {
	a()
}

func c() {
	a()
}
