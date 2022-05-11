package Time

import (
	"fmt"
	"runtime"
	"time"
)

// Converts bytes to Mega-bytes
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// Prints elapsed time to console, see README.md for usage.
func Elapsed(what string) func() {
	start := time.Now()

	return func() {
		fmt.Printf("%s took %v\n", what, time.Since(start))
	}
}

// Prints elapsed time with total memory allocation to console, see README.md for usage.
func ElapsedWithMemory(what string) func() {
	start := time.Now()
	var m runtime.MemStats

	return func() {
		runtime.ReadMemStats(&m)
		fmt.Printf("%s took %v\n", what, time.Since(start))
		fmt.Printf("MemoryAllocated = %v MB\n", bToMb(m.Alloc))
	}
}
