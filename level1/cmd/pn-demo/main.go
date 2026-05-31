package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ronaldpetty/pn-challenge/internal/artifacts"
	"github.com/ronaldpetty/pn-challenge/internal/client"
	"github.com/ronaldpetty/pn-challenge/internal/server"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: %s <artifacts|index|agent|client|health>", args[0])
	}
	switch args[1] {
	case "artifacts":
		fs := flag.NewFlagSet("artifacts", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		return artifacts.Generate(*artifactsDir)
	case "index":
		fs := flag.NewFlagSet("index", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		addr := fs.String("addr", ":8443", "listen address")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		srv, err := server.NewIndexServer(*artifactsDir)
		if err != nil {
			return err
		}
		return srv.ListenAndServe(*addr)
	case "agent":
		fs := flag.NewFlagSet("agent", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		shortName := fs.String("name", "", "short agent name, alpha or beta")
		serviceName := fs.String("service-name", "", "compose service name")
		addr := fs.String("addr", ":8443", "listen address")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if *shortName == "" || *serviceName == "" {
			return fmt.Errorf("agent requires --name and --service-name")
		}
		srv, err := server.NewAgentServer(*artifactsDir, *shortName, *serviceName)
		if err != nil {
			return err
		}
		return srv.ListenAndServe(*addr)
	case "client":
		fs := flag.NewFlagSet("client", flag.ExitOnError)
		artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
		indexURL := fs.String("index", "https://index:8443", "index base URL")
		agents := fs.String("agents", "alpha.nanda.local,beta.nanda.local", "comma-separated agent names")
		invoke := fs.Bool("invoke", true, "call verified runtime endpoint")
		tamperCheck := fs.Bool("tamper-check", false, "prove tampered facts fail verification")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		resolver, err := client.NewResolver(client.Options{
			ArtifactsDir: *artifactsDir,
			IndexURL:     *indexURL,
			Agents:       client.ParseAgents(*agents),
			Invoke:       *invoke,
			TamperCheck:  *tamperCheck,
		})
		if err != nil {
			return err
		}
		return resolver.Run()
	case "health":
		fs := flag.NewFlagSet("health", flag.ExitOnError)
		url := fs.String("url", "", "health URL")
		ca := fs.String("ca", "/artifacts/tls/ca/ca.crt", "CA certificate")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if *url == "" {
			return fmt.Errorf("health requires --url")
		}
		return client.Health(*url, *ca)
	default:
		slog.Error("unknown command", "command", args[1])
		return fmt.Errorf("unknown command %q", args[1])
	}
}
