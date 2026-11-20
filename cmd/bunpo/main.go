// Command bunpo splits a test suite across parallel CI machines using
// recorded timings, and verifies that every test file was assigned once.
package main

import (
	"fmt"
	"os"
)

func main() {
	// Placeholder until split, plan, record, merge and verify exist.
	fmt.Fprintln(os.Stderr, "bunpo: no commands are implemented yet")
	os.Exit(2)
}
