package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ronaldpetty/pn-challenge/level8/internal/swimlane"
)

func main() {
	fs := flag.NewFlagSet("swimlane", flag.ExitOnError)
	logsDir := fs.String("logs", "/logs", "audit log directory")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal(err)
	}
	if err := swimlane.Run(*logsDir); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
