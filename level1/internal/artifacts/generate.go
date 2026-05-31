package artifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ronaldpetty/pn-challenge/internal/credential"
)

type AgentSpec struct {
	ShortName   string
	Name        string
	ID          string
	ServiceName string
	Network     string
	Description string
}

type IndexFile struct {
	Agents []IndexAgent `json:"agents"`
}

type IndexAgent struct {
	Name       string         `json:"name"`
	Aliases    []string       `json:"aliases"`
	Credential map[string]any `json:"credential"`
}

func Generate(dir string) error {
	now := time.Now().UTC().Truncate(time.Second)
	privateKey, err := credential.LoadEd25519PrivateKey(filepath.Join(dir, "keys", "nanda-issuer.pem"))
	if err != nil {
		return err
	}
	publicKey, err := credential.LoadEd25519PublicKey(filepath.Join(dir, "keys", "nanda-issuer.pub.pem"))
	if err != nil {
		return err
	}

	for _, subdir := range []string{"trust", "index", "agents/alpha", "agents/beta"} {
		if err := os.MkdirAll(filepath.Join(dir, subdir), 0o755); err != nil {
			return err
		}
	}

	bundle := credential.TrustBundle{Issuers: []credential.Issuer{{
		ID:                 credential.DefaultIssuerID,
		VerificationMethod: credential.DefaultIssuerKeyID,
		Type:               "Ed25519VerificationKey2020",
		PublicKeyBase64URL: credential.PublicKeyBase64URL(publicKey),
	}}}
	if err := writeJSON(filepath.Join(dir, "trust", "issuers.json"), bundle); err != nil {
		return err
	}

	specs := []AgentSpec{
		{
			ShortName:   "alpha",
			Name:        "alpha.nanda.local",
			ID:          "agent-alpha",
			ServiceName: "agent-alpha",
			Network:     "agent_alpha_net",
			Description: "Alpha is a Level 1 local echo agent isolated on the alpha Docker network.",
		},
		{
			ShortName:   "beta",
			Name:        "beta.nanda.local",
			ID:          "agent-beta",
			ServiceName: "agent-beta",
			Network:     "agent_beta_net",
			Description: "Beta is a Level 1 local echo agent isolated on the beta Docker network.",
		},
	}

	indexFile := IndexFile{Agents: make([]IndexAgent, 0, len(specs))}
	for _, spec := range specs {
		addrCredential, err := agentAddrCredential(spec, now)
		if err != nil {
			return err
		}
		if err := credential.Sign(addrCredential, privateKey, credential.DefaultIssuerKeyID, now); err != nil {
			return fmt.Errorf("sign AgentAddr for %s: %w", spec.Name, err)
		}
		factsCredential, err := agentFactsCredential(spec, now)
		if err != nil {
			return err
		}
		if err := credential.Sign(factsCredential, privateKey, credential.DefaultIssuerKeyID, now); err != nil {
			return fmt.Errorf("sign AgentFacts for %s: %w", spec.Name, err)
		}
		if err := writeJSON(filepath.Join(dir, "agents", spec.ShortName, "facts.vc.json"), factsCredential); err != nil {
			return err
		}
		indexFile.Agents = append(indexFile.Agents, IndexAgent{
			Name:       spec.Name,
			Aliases:    []string{spec.ShortName, spec.ID},
			Credential: addrCredential,
		})
	}
	return writeJSON(filepath.Join(dir, "index", "agents.json"), indexFile)
}

func agentAddrCredential(spec AgentSpec, now time.Time) (map[string]any, error) {
	return baseCredential(
		"urn:nanda:agent-addr:"+spec.ShortName,
		"AgentAddrCredential",
		now,
		map[string]any{
			"id":          "agent:" + spec.ShortName,
			"agentID":     spec.ID,
			"name":        spec.Name,
			"factsURL":    fmt.Sprintf("https://%s:8443/facts", spec.ServiceName),
			"invokeURL":   fmt.Sprintf("https://%s:8443/invoke", spec.ServiceName),
			"network":     spec.Network,
			"tlsCA":       "NANDA Demo Local CA",
			"expiresHint": now.Add(15 * time.Minute).UTC().Format(time.RFC3339),
		},
	), nil
}

func agentFactsCredential(spec AgentSpec, now time.Time) (map[string]any, error) {
	return baseCredential(
		"urn:nanda:agent-facts:"+spec.ShortName,
		"AgentFactsCredential",
		now,
		map[string]any{
			"id":          "agent:" + spec.ShortName,
			"agentID":     spec.ID,
			"name":        spec.Name,
			"description": spec.Description,
			"network":     spec.Network,
			"capabilities": []any{
				"echo",
				"status",
			},
			"service": []any{
				map[string]any{
					"id":              "facts",
					"type":            "AgentFacts",
					"serviceEndpoint": fmt.Sprintf("https://%s:8443/facts", spec.ServiceName),
				},
				map[string]any{
					"id":              "invoke",
					"type":            "AgentRuntime",
					"serviceEndpoint": fmt.Sprintf("https://%s:8443/invoke", spec.ServiceName),
				},
			},
		},
	), nil
}

func baseCredential(id, credentialType string, now time.Time, subject map[string]any) map[string]any {
	return map[string]any{
		"@context": []any{
			"https://www.w3.org/2018/credentials/v1",
			"https://nanda.local/contexts/agent/v1",
		},
		"id":                id,
		"type":              []any{"VerifiableCredential", credentialType},
		"issuer":            credential.DefaultIssuerID,
		"issuanceDate":      now.UTC().Format(time.RFC3339),
		"expirationDate":    now.Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339),
		"credentialSubject": subject,
	}
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
