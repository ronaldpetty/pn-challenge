package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ronaldpetty/pn-challenge/level8/internal/revocation"
)

func main() {
	fs := flag.NewFlagSet("revocation-authority", flag.ExitOnError)
	artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
	logsDir := fs.String("logs", "/logs", "audit log directory")
	addr := fs.String("addr", ":8080", "listen address")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal(err)
	}
	if err := revocation.RunAuthority(*artifactsDir, *logsDir, *addr); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
