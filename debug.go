//go:build debug

package rtu

var alloc int

func noteAlloc(x int) {
	alloc = x
}
