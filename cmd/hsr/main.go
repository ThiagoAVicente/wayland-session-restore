package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	fmt.Fprintln(os.Stderr, "hsr", version, "- not wired yet")
	os.Exit(2)
}
