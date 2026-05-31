package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ronaldpetty/pn-challenge/level8/internal/consumer"
)

func main() {
	fs := flag.NewFlagSet("consumer", flag.ExitOnError)
	artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
	logsDir := fs.String("logs", "/logs", "audit log directory")
	indexes := fs.String("indexes", "http://nanda-index-a:8080", "comma-separated bootstrap index URLs")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal(err)
	}
	if err := consumer.Run(*artifactsDir, *logsDir, *indexes); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
