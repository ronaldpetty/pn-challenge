package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ronaldpetty/pn-challenge/level8/internal/agents"
)

func main() {
	fs := flag.NewFlagSet("mcp-agent", flag.ExitOnError)
	logsDir := fs.String("logs", "/logs", "audit log directory")
	agentID := fs.String("agent", "", "agent id")
	tool := fs.String("tool", "", "single supported tool")
	addr := fs.String("addr", ":8080", "listen address")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal(err)
	}
	if err := agents.RunMCPServer(*logsDir, *agentID, *tool, *addr); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
