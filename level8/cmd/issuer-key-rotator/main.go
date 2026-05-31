package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ronaldpetty/pn-challenge/level8/internal/rotator"
)

func main() {
	fs := flag.NewFlagSet("issuer-key-rotator", flag.ExitOnError)
	artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
	logsDir := fs.String("logs", "/logs", "audit log directory")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal(err)
	}
	if err := rotator.RunIssuerKeys(*artifactsDir, *logsDir); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
