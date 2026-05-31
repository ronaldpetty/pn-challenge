package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ronaldpetty/pn-challenge/internal/artifacts"
)

type IndexServer struct {
	artifactsDir string
	records      map[string]map[string]any
	names        []string
}

func NewIndexServer(artifactsDir string) (*IndexServer, error) {
	data, err := os.ReadFile(filepath.Join(artifactsDir, "index", "agents.json"))
	if err != nil {
		return nil, err
	}
	var indexFile artifacts.IndexFile
	if err := json.Unmarshal(data, &indexFile); err != nil {
		return nil, err
	}
	records := make(map[string]map[string]any)
	names := make([]string, 0, len(indexFile.Agents))
	for _, agent := range indexFile.Agents {
		names = append(names, agent.Name)
		records[agent.Name] = agent.Credential
		for _, alias := range agent.Aliases {
			records[alias] = agent.Credential
		}
	}
	return &IndexServer{artifactsDir: artifactsDir, records: records, names: names}, nil
}

func (s *IndexServer) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"ok": true, "service": "index"}, http.StatusOK
	}))
	mux.HandleFunc("GET /agents", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"agents": s.names}, http.StatusOK
	}))
	mux.HandleFunc("GET /resolve/", jsonHandler(func(r *http.Request) (any, int) {
		name := strings.TrimPrefix(r.URL.Path, "/resolve/")
		decoded, err := url.PathUnescape(name)
		if err != nil {
			return map[string]string{"error": "bad agent name"}, http.StatusBadRequest
		}
		record, ok := s.records[decoded]
		if !ok {
			return map[string]string{"error": "agent not found"}, http.StatusNotFound
		}
		return record, http.StatusOK
	}))
	return listenAndServeTLS(addr, s.artifactsDir, "index", mux)
}

type AgentServer struct {
	artifactsDir string
	shortName    string
	serviceName  string
	facts        json.RawMessage
}

func NewAgentServer(artifactsDir, shortName, serviceName string) (*AgentServer, error) {
	data, err := os.ReadFile(filepath.Join(artifactsDir, "agents", shortName, "facts.vc.json"))
	if err != nil {
		return nil, err
	}
	return &AgentServer{
		artifactsDir: artifactsDir,
		shortName:    shortName,
		serviceName:  serviceName,
		facts:        data,
	}, nil
}

func (s *AgentServer) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"ok": true, "service": s.serviceName}, http.StatusOK
	}))
	mux.HandleFunc("GET /facts", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vc+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(s.facts)
	})
	mux.HandleFunc("POST /invoke", jsonHandler(func(r *http.Request) (any, int) {
		var body struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return map[string]string{"error": "invalid JSON request"}, http.StatusBadRequest
		}
		if body.Message == "" {
			body.Message = "ping"
		}
		return map[string]any{
			"agent":   s.shortName,
			"service": s.serviceName,
			"reply":   fmt.Sprintf("%s handled %q", s.shortName, body.Message),
			"time":    time.Now().UTC().Format(time.RFC3339),
		}, http.StatusOK
	}))
	return listenAndServeTLS(addr, s.artifactsDir, s.serviceName, mux)
}

func listenAndServeTLS(addr, artifactsDir, serviceName string, handler http.Handler) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	certPath := filepath.Join(artifactsDir, "tls", serviceName, "tls.crt")
	keyPath := filepath.Join(artifactsDir, "tls", serviceName, "tls.key")
	slog.Info("listening", "service", serviceName, "addr", addr)
	return server.ListenAndServeTLS(certPath, keyPath)
}

func jsonHandler(fn func(*http.Request) (any, int)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, status := fn(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			slog.Error("encode response", "err", err)
		}
	}
}
