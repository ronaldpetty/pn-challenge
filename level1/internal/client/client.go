package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ronaldpetty/pn-challenge/internal/credential"
)

type Options struct {
	ArtifactsDir string
	IndexURL     string
	Agents       []string
	Invoke       bool
	TamperCheck  bool
}

type Resolver struct {
	options Options
	bundle  credential.TrustBundle
	http    *http.Client
}

func NewResolver(options Options) (*Resolver, error) {
	if options.IndexURL == "" {
		return nil, errors.New("index URL is required")
	}
	bundle, err := credential.LoadTrustBundle(filepath.Join(options.ArtifactsDir, "trust", "issuers.json"))
	if err != nil {
		return nil, err
	}
	caCert, err := os.ReadFile(filepath.Join(options.ArtifactsDir, "tls", "ca", "ca.crt"))
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to load CA certificate")
	}
	return &Resolver{
		options: options,
		bundle:  bundle,
		http: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{
				RootCAs:    roots,
				MinVersion: tls.VersionTLS12,
			}},
		},
	}, nil
}

func (r *Resolver) Run() error {
	for _, agent := range r.options.Agents {
		if err := r.Resolve(agent); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) Resolve(agent string) error {
	fmt.Printf("resolve %s\n", agent)
	addrRaw, err := r.get(r.options.IndexURL + "/resolve/" + agent)
	if err != nil {
		return fmt.Errorf("fetch AgentAddr: %w", err)
	}
	addr, err := credential.Verify(addrRaw, r.bundle, time.Now())
	if err != nil {
		return fmt.Errorf("verify AgentAddr: %w", err)
	}
	factsURL, err := stringField(addr.Subject, "factsURL")
	if err != nil {
		return err
	}
	invokeURL, err := stringField(addr.Subject, "invokeURL")
	if err != nil {
		return err
	}
	network, _ := addr.Subject["network"].(string)
	fmt.Printf("  AgentAddr verified: facts=%s network=%s\n", factsURL, network)

	factsRaw, err := r.get(factsURL)
	if err != nil {
		return fmt.Errorf("fetch AgentFacts: %w", err)
	}
	facts, err := credential.Verify(factsRaw, r.bundle, time.Now())
	if err != nil {
		return fmt.Errorf("verify AgentFacts: %w", err)
	}
	description, _ := facts.Subject["description"].(string)
	fmt.Printf("  AgentFacts verified: %s\n", description)

	if r.options.TamperCheck {
		if err := r.expectTamperFailure(factsRaw); err != nil {
			return err
		}
		fmt.Println("  Tamper check passed")
	}
	if r.options.Invoke {
		if err := r.invoke(invokeURL, agent); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) expectTamperFailure(raw []byte) error {
	var tampered map[string]any
	if err := json.Unmarshal(raw, &tampered); err != nil {
		return err
	}
	subject, ok := tampered["credentialSubject"].(map[string]any)
	if !ok {
		return errors.New("tamper target missing credentialSubject")
	}
	subject["description"] = "tampered metadata"
	tamperedRaw, err := json.Marshal(tampered)
	if err != nil {
		return err
	}
	if _, err := credential.Verify(tamperedRaw, r.bundle, time.Now()); err == nil {
		return errors.New("tampered AgentFacts verified unexpectedly")
	}
	return nil
}

func (r *Resolver) invoke(invokeURL, agent string) error {
	requestBody := map[string]string{"message": "hello from resolver client to " + agent}
	raw, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	resp, err := r.http.Post(invokeURL, "application/json", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("invoke agent: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("invoke returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	fmt.Printf("  Invoke response: %s\n", strings.TrimSpace(string(body)))
	return nil
}

func (r *Resolver) get(url string) ([]byte, error) {
	resp, err := r.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func stringField(subject map[string]any, key string) (string, error) {
	value, _ := subject[key].(string)
	if value == "" {
		return "", fmt.Errorf("credential subject field %q is missing", key)
	}
	return value, nil
}

func ParseAgents(value string) []string {
	parts := strings.Split(value, ",")
	agents := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			agents = append(agents, trimmed)
		}
	}
	return agents
}

func Health(url, caPath string) error {
	caCert, err := os.ReadFile(caPath)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caCert) {
		return errors.New("failed to load CA certificate")
	}
	httpClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS12,
		}},
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", resp.Status)
	}
	return nil
}
