package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ronaldpetty/pn-challenge/level6/internal/app"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: %s <artifacts|index|registry|private-facts|revocation-authority|crdt-update-bus|mcp|rotator|consumer|swimlane|health>", args[0])
	}

	switch args[1] {
	case "artifacts":
		fs := flag.NewFlagSet("artifacts", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.GenerateArtifacts(*artifactsDir)
	case "index":
		fs := flag.NewFlagSet("index", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		logsDir := fs.String("logs", "/logs", "audit log directory")
		addr := fs.String("addr", ":8080", "listen address")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.RunIndex(*artifactsDir, *logsDir, *addr)
	case "registry":
		fs := flag.NewFlagSet("registry", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		logsDir := fs.String("logs", "/logs", "audit log directory")
		enterprise := fs.String("enterprise", "", "enterprise key: enterprise-a or enterprise-b")
		addr := fs.String("addr", ":8080", "listen address")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.RunRegistry(*artifactsDir, *logsDir, *enterprise, *addr)
	case "private-facts":
		fs := flag.NewFlagSet("private-facts", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		logsDir := fs.String("logs", "/logs", "audit log directory")
		addr := fs.String("addr", ":8080", "listen address")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.RunPrivateFactsGateway(*artifactsDir, *logsDir, *addr)
	case "revocation-authority":
		fs := flag.NewFlagSet("revocation-authority", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		logsDir := fs.String("logs", "/logs", "audit log directory")
		addr := fs.String("addr", ":8080", "listen address")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.RunRevocationAuthority(*artifactsDir, *logsDir, *addr)
	case "crdt-update-bus":
		fs := flag.NewFlagSet("crdt-update-bus", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		logsDir := fs.String("logs", "/logs", "audit log directory")
		addr := fs.String("addr", ":8080", "listen address")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.RunCRDTUpdateBus(*artifactsDir, *logsDir, *addr)
	case "mcp":
		fs := flag.NewFlagSet("mcp", flag.ExitOnError)
		logsDir := fs.String("logs", "/logs", "audit log directory")
		agentID := fs.String("agent", "", "agent id")
		tool := fs.String("tool", "", "single supported tool")
		addr := fs.String("addr", ":8080", "listen address")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.RunMCPServer(*logsDir, *agentID, *tool, *addr)
	case "rotator":
		fs := flag.NewFlagSet("rotator", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		logsDir := fs.String("logs", "/logs", "audit log directory")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.RunRotator(*artifactsDir, *logsDir)
	case "consumer":
		fs := flag.NewFlagSet("consumer", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		logsDir := fs.String("logs", "/logs", "audit log directory")
		indexURL := fs.String("index", "http://nanda-index:8080", "NANDA index URL")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.RunConsumer(*artifactsDir, *logsDir, *indexURL)
	case "swimlane":
		fs := flag.NewFlagSet("swimlane", flag.ExitOnError)
		logsDir := fs.String("logs", "/logs", "audit log directory")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.RunSwimlane(*logsDir)
	case "health":
		fs := flag.NewFlagSet("health", flag.ExitOnError)
		url := fs.String("url", "", "health endpoint URL")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return app.Health(*url)
	default:
		return fmt.Errorf("unknown command %q", args[1])
	}
}
