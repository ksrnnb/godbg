package main

type BreakPoint struct {
	pid  int
	addr int
}

func NewBreakPoint(pid int, addr int) BreakPoint {
	return BreakPoint{pid: pid, addr: addr}
}
