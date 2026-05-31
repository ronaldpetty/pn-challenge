package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ronaldpetty/pn-challenge/level8/internal/shared"
)

func main() {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	url := fs.String("url", "", "health endpoint URL")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal(err)
	}
	if err := shared.Health(*url); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
