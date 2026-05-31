package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ronaldpetty/pn-challenge/level8/internal/registry"
)

func main() {
	fs := flag.NewFlagSet("enterprise-registry", flag.ExitOnError)
	artifactsDir := fs.String("artifacts", "/artifacts", "artifact directory")
	logsDir := fs.String("logs", "/logs", "audit log directory")
	enterprise := fs.String("enterprise", "", "enterprise key")
	joinIndexURL := fs.String("join-index", "", "index URL to join")
	addr := fs.String("addr", ":8080", "listen address")
	registryName := fs.String("registry-name", "", "runtime registry DNS name")
	registryID := fs.String("registry-id", "", "runtime registry id")
	catalogURL := fs.String("catalog-url", "", "runtime signed catalog URL")
	privateFactsURL := fs.String("private-facts-url", "", "runtime private facts URL")
	crdtUpdateURL := fs.String("crdt-update-url", "", "runtime CRDT update URL")
	factsMode := fs.String("facts-mode", "", "runtime facts mode: public or private")
	description := fs.String("description", "", "runtime registry description")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal(err)
	}
	if err := registry.RunEnterpriseWithOptions(registry.Options{
		ArtifactsDir:    *artifactsDir,
		LogsDir:         *logsDir,
		Enterprise:      *enterprise,
		JoinIndexURL:    *joinIndexURL,
		Addr:            *addr,
		RegistryName:    *registryName,
		RegistryID:      *registryID,
		CatalogURL:      *catalogURL,
		PrivateFactsURL: *privateFactsURL,
		CRDTUpdateURL:   *crdtUpdateURL,
		FactsMode:       *factsMode,
		Description:     *description,
	}); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
