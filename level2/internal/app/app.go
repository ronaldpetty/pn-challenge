package app

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	issuerID    = "did:web:nanda.local:level2-issuer"
	issuerKeyID = issuerID + "#key-1"
	proofType   = "Ed25519Signature2020"
)

type TrustBundle struct {
	Issuers []Issuer `json:"issuers"`
}

type Issuer struct {
	ID                 string `json:"id"`
	VerificationMethod string `json:"verificationMethod"`
	Type               string `json:"type"`
	PublicKeyBase64URL string `json:"publicKeyBase64URL"`
}

type IndexFile struct {
	Registries []RegistryRecord `json:"registries"`
}

type RegistryRecord struct {
	Name       string         `json:"name"`
	Aliases    []string       `json:"aliases"`
	Credential map[string]any `json:"credential"`
}

type EnterpriseSpec struct {
	Key         string
	Name        string
	RegistryID  string
	CatalogURL  string
	Description string
	Agents      []AgentSpec
}

type AgentSpec struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Endpoint string   `json:"endpoint"`
	Tools    []string `json:"tools"`
}

type Event struct {
	Time   string `json:"time"`
	Actor  string `json:"actor"`
	Peer   string `json:"peer"`
	Action string `json:"action"`
	Result string `json:"result"`
}

func GenerateArtifacts(dir string) error {
	ready := filepath.Join(dir, ".ready")
	if _, err := os.Stat(ready); err == nil {
		fmt.Println("level2 artifacts already initialized at", dir)
		return nil
	}

	for _, subdir := range []string{"trust", "keys", "index", "registries/enterprise-a", "registries/enterprise-b"} {
		if err := os.MkdirAll(filepath.Join(dir, subdir), 0o755); err != nil {
			return err
		}
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	keyFile := map[string]string{
		"issuerPrivateKeyBase64URL": base64.RawURLEncoding.EncodeToString(privateKey),
		"issuerPublicKeyBase64URL":  base64.RawURLEncoding.EncodeToString(publicKey),
	}
	if err := writeJSON(filepath.Join(dir, "keys", "issuer.json"), keyFile); err != nil {
		return err
	}

	bundle := TrustBundle{Issuers: []Issuer{{
		ID:                 issuerID,
		VerificationMethod: issuerKeyID,
		Type:               "Ed25519VerificationKey2020",
		PublicKeyBase64URL: base64.RawURLEncoding.EncodeToString(publicKey),
	}}}
	if err := writeJSON(filepath.Join(dir, "trust", "issuers.json"), bundle); err != nil {
		return err
	}

	now := time.Now().UTC().Truncate(time.Second)
	index := IndexFile{}
	for _, enterprise := range enterpriseSpecs() {
		addr := baseCredential("urn:nanda:registry-addr:"+enterprise.Key, "EnterpriseRegistryAddrCredential", now, map[string]any{
			"id":               "registry:" + enterprise.Key,
			"registryID":       enterprise.RegistryID,
			"name":             enterprise.Name,
			"registrationType": "enterprise-mcp-registry",
			"catalogURL":       enterprise.CatalogURL,
			"description":      enterprise.Description,
		})
		if err := sign(addr, privateKey, now); err != nil {
			return err
		}
		index.Registries = append(index.Registries, RegistryRecord{
			Name:       enterprise.Name,
			Aliases:    []string{enterprise.Key, enterprise.RegistryID},
			Credential: addr,
		})

		agents := make([]any, 0, len(enterprise.Agents))
		for _, agent := range enterprise.Agents {
			agents = append(agents, map[string]any{
				"id":       agent.ID,
				"name":     agent.Name,
				"endpoint": agent.Endpoint,
				"tools":    agent.Tools,
			})
		}
		catalog := baseCredential("urn:nanda:enterprise-catalog:"+enterprise.Key, "EnterpriseMCPCatalogCredential", now, map[string]any{
			"id":          "catalog:" + enterprise.Key,
			"registryID":  enterprise.RegistryID,
			"name":        enterprise.Name,
			"description": enterprise.Description,
			"agents":      agents,
		})
		if err := sign(catalog, privateKey, now); err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(dir, "registries", enterprise.Key, "catalog.vc.json"), catalog); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(dir, "index", "registries.json"), index); err != nil {
		return err
	}
	if err := os.WriteFile(ready, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Println("level2 artifacts initialized at", dir)
	return nil
}

func RunIndex(artifactsDir, logsDir, addr string) error {
	data, err := os.ReadFile(filepath.Join(artifactsDir, "index", "registries.json"))
	if err != nil {
		return err
	}
	var index IndexFile
	if err := json.Unmarshal(data, &index); err != nil {
		return err
	}
	records := map[string]map[string]any{}
	names := make([]string, 0, len(index.Registries))
	for _, registry := range index.Registries {
		names = append(names, registry.Name)
		records[registry.Name] = registry.Credential
		for _, alias := range registry.Aliases {
			records[alias] = registry.Credential
		}
	}
	audit := NewAuditor(logsDir, "nanda-index")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"ok": true, "service": "nanda-index"}, http.StatusOK
	}))
	mux.HandleFunc("GET /registries", jsonHandler(func(_ *http.Request) (any, int) {
		audit.Log("consumer", "list_registries", strings.Join(names, ","))
		return map[string]any{"registries": names}, http.StatusOK
	}))
	mux.HandleFunc("GET /search", jsonHandler(func(r *http.Request) (any, int) {
		registrationType := r.URL.Query().Get("registrationType")
		if registrationType != "" && registrationType != "enterprise-mcp-registry" {
			audit.Log("consumer", "search_registries", "no_match:"+registrationType)
			return map[string]any{
				"registrationType": registrationType,
				"registries":       []string{},
			}, http.StatusOK
		}
		audit.Log("consumer", "search_registries", strings.Join(names, ","))
		return map[string]any{
			"registrationType": "enterprise-mcp-registry",
			"registries":       names,
		}, http.StatusOK
	}))
	mux.HandleFunc("GET /resolve/", jsonHandler(func(r *http.Request) (any, int) {
		rawName := strings.TrimPrefix(r.URL.Path, "/resolve/")
		name, err := url.PathUnescape(rawName)
		if err != nil {
			return map[string]string{"error": "bad registry name"}, http.StatusBadRequest
		}
		record, ok := records[name]
		if !ok {
			audit.Log("consumer", "resolve_registry", "not_found:"+name)
			return map[string]string{"error": "registry not found"}, http.StatusNotFound
		}
		audit.Log("consumer", "resolve_registry", name)
		return record, http.StatusOK
	}))
	return listen(addr, "nanda-index", mux)
}

func RunRegistry(artifactsDir, logsDir, enterprise, addr string) error {
	if enterprise == "" {
		return errors.New("registry requires --enterprise")
	}
	catalog, err := os.ReadFile(filepath.Join(artifactsDir, "registries", enterprise, "catalog.vc.json"))
	if err != nil {
		return err
	}
	audit := NewAuditor(logsDir, enterprise+"-registry")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"ok": true, "service": enterprise + "-registry"}, http.StatusOK
	}))
	mux.HandleFunc("GET /catalog", func(w http.ResponseWriter, _ *http.Request) {
		audit.Log("consumer", "serve_signed_catalog", enterprise)
		w.Header().Set("Content-Type", "application/vc+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(catalog)
	})
	return listen(addr, enterprise+"-registry", mux)
}

func RunMCPServer(logsDir, agentID, tool, addr string) error {
	if agentID == "" || tool == "" {
		return errors.New("mcp requires --agent and --tool")
	}
	audit := NewAuditor(logsDir, agentID)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"ok": true, "service": agentID}, http.StatusOK
	}))
	mux.HandleFunc("GET /mcp/tools/list", jsonHandler(func(_ *http.Request) (any, int) {
		audit.Log("consumer", "list_tools", tool)
		return map[string]any{
			"agent": agentID,
			"tools": []any{map[string]any{
				"name":        tool,
				"description": "simple text tool: " + tool,
			}},
		}, http.StatusOK
	}))
	mux.HandleFunc("POST /mcp/tools/call", jsonHandler(func(r *http.Request) (any, int) {
		var req struct {
			Tool  string `json:"tool"`
			Input string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return map[string]string{"error": "invalid JSON"}, http.StatusBadRequest
		}
		if req.Tool != tool {
			audit.Log("consumer", "reject_skill_mismatch", req.Tool+" not in "+tool)
			return map[string]any{
				"agent":          agentID,
				"requested_tool": req.Tool,
				"allowed_tools":  []string{tool},
				"error":          "skill mismatch",
			}, http.StatusBadRequest
		}
		result := runTool(tool, req.Input)
		audit.Log("consumer", "execute_tool", tool+"="+result)
		return map[string]any{
			"agent":  agentID,
			"tool":   tool,
			"input":  req.Input,
			"result": result,
		}, http.StatusOK
	}))
	return listen(addr, agentID, mux)
}

func RunConsumer(artifactsDir, logsDir, indexURL string) error {
	audit := NewAuditor(logsDir, "consumer")
	bundle, err := loadTrustBundle(filepath.Join(artifactsDir, "trust", "issuers.json"))
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}

	audit.Log("nanda-index", "search_registries", "registrationType=enterprise-mcp-registry")
	var registries struct {
		Registries []string `json:"registries"`
	}
	if err := getJSON(client, indexURL+"/search?registrationType=enterprise-mcp-registry", &registries); err != nil {
		return err
	}
	audit.Log("nanda-index", "search_registries_result", strings.Join(registries.Registries, ","))

	verifiedAgents := []AgentSpec{}
	for _, registryName := range registries.Registries {
		audit.Log("nanda-index", "resolve_registry", registryName)
		rawAddr, err := getBytes(client, indexURL+"/resolve/"+url.PathEscape(registryName))
		if err != nil {
			return err
		}
		addr, err := verify(rawAddr, bundle, time.Now())
		if err != nil {
			return fmt.Errorf("verify registry addr for %s: %w", registryName, err)
		}
		catalogURL, _ := addr.Subject["catalogURL"].(string)
		registryID, _ := addr.Subject["registryID"].(string)
		audit.Log("nanda-index", "verified_registry_addr", registryID)

		audit.Log(registryID, "fetch_catalog", catalogURL)
		rawCatalog, err := getBytes(client, catalogURL)
		if err != nil {
			return err
		}
		catalog, err := verify(rawCatalog, bundle, time.Now())
		if err != nil {
			return fmt.Errorf("verify catalog for %s: %w", registryName, err)
		}
		agents, err := agentsFromSubject(catalog.Subject)
		if err != nil {
			return err
		}
		audit.Log(registryID, "verified_catalog", fmt.Sprintf("%d agents", len(agents)))
		verifiedAgents = append(verifiedAgents, agents...)
	}

	for _, agent := range verifiedAgents {
		toolsListURL := strings.TrimRight(agent.Endpoint, "/") + "/mcp/tools/list"
		audit.Log(agent.ID, "confirm_skills", strings.Join(agent.Tools, ","))
		var toolsResp map[string]any
		if err := getJSON(client, toolsListURL, &toolsResp); err != nil {
			return err
		}
		listedTools, err := toolsFromListResponse(toolsResp)
		if err != nil {
			return err
		}
		if !sameStringSet(agent.Tools, listedTools) {
			return fmt.Errorf("tool mismatch for %s: catalog=%v mcp=%v", agent.ID, agent.Tools, listedTools)
		}
		audit.Log(agent.ID, "skills_confirmed", strings.Join(listedTools, ","))
		if len(agent.Tools) == 0 {
			continue
		}
		callURL := strings.TrimRight(agent.Endpoint, "/") + "/mcp/tools/call"
		result, err := callTool(client, callURL, agent.Tools[0], "NANDA level two demo text")
		if err != nil {
			return err
		}
		audit.Log(agent.ID, "tool_result", result)
	}

	if len(verifiedAgents) > 0 {
		agent := verifiedAgents[0]
		wrongTool := "truncate"
		if contains(agent.Tools, wrongTool) {
			wrongTool = "reverse"
		}
		audit.Log(agent.ID, "prove_invalid_skill_mismatch", wrongTool+" not in verified catalog")
		_, err := callTool(client, strings.TrimRight(agent.Endpoint, "/")+"/mcp/tools/call", wrongTool, "this should fail")
		if err == nil {
			return errors.New("invalid skill mismatch unexpectedly succeeded")
		}
		audit.Log(agent.ID, "invalid_skill_rejected", err.Error())
	}
	fmt.Println("level2 consumer completed discovery, verification, tool calls, and skill mismatch rejection")
	return nil
}

func RunSwimlane(logsDir string) error {
	events, err := readEvents(logsDir)
	if err != nil {
		return err
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Time < events[j].Time
	})
	fmt.Println("Level 2 audit swimlane")
	fmt.Println("time                         | actor                  | activity")
	fmt.Println("-----------------------------+------------------------+-----------------------------------------------")
	for _, event := range events {
		fmt.Printf("%-28s | %-22s | --(%s: %s)--> %s\n", event.Time, event.Actor, event.Action, event.Result, event.Peer)
	}
	return nil
}

func Health(endpoint string) error {
	if endpoint == "" {
		return errors.New("health requires --url")
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health returned %s", resp.Status)
	}
	return nil
}

type VerificationResult struct {
	Credential map[string]any
	Subject    map[string]any
}

func enterpriseSpecs() []EnterpriseSpec {
	return []EnterpriseSpec{
		{
			Key:         "enterprise-a",
			Name:        "enterprise-a.registry.nanda.local",
			RegistryID:  "enterprise-a-registry",
			CatalogURL:  "http://enterprise-a-registry:8080/catalog",
			Description: "Enterprise A registry catalogs text transform MCP servers.",
			Agents: []AgentSpec{
				{ID: "enterprise-a-reverse", Name: "enterprise-a.reverse.mcp.local", Endpoint: "http://enterprise-a-reverse:8080", Tools: []string{"reverse"}},
				{ID: "enterprise-a-uppercase", Name: "enterprise-a.uppercase.mcp.local", Endpoint: "http://enterprise-a-uppercase:8080", Tools: []string{"uppercase"}},
			},
		},
		{
			Key:         "enterprise-b",
			Name:        "enterprise-b.registry.nanda.local",
			RegistryID:  "enterprise-b-registry",
			CatalogURL:  "http://enterprise-b-registry:8080/catalog",
			Description: "Enterprise B registry catalogs text utility MCP servers.",
			Agents: []AgentSpec{
				{ID: "enterprise-b-truncate", Name: "enterprise-b.truncate.mcp.local", Endpoint: "http://enterprise-b-truncate:8080", Tools: []string{"truncate"}},
				{ID: "enterprise-b-count", Name: "enterprise-b.count.mcp.local", Endpoint: "http://enterprise-b-count:8080", Tools: []string{"count"}},
			},
		},
	}
}

func runTool(tool, input string) string {
	switch tool {
	case "reverse":
		runes := []rune(input)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes)
	case "uppercase":
		return strings.ToUpper(input)
	case "truncate":
		if len(input) <= 12 {
			return input
		}
		return input[:12]
	case "count":
		return strconv.Itoa(len([]rune(input)))
	default:
		return input
	}
}

func baseCredential(id, credentialType string, now time.Time, subject map[string]any) map[string]any {
	return map[string]any{
		"@context": []any{
			"https://www.w3.org/2018/credentials/v1",
			"https://nanda.local/contexts/enterprise-mcp/v1",
		},
		"id":                id,
		"type":              []any{"VerifiableCredential", credentialType},
		"issuer":            issuerID,
		"issuanceDate":      now.Format(time.RFC3339),
		"expirationDate":    now.Add(365 * 24 * time.Hour).Format(time.RFC3339),
		"credentialSubject": subject,
	}
}

func sign(vc map[string]any, privateKey ed25519.PrivateKey, created time.Time) error {
	delete(vc, "proof")
	canonical, err := canonicalCredential(vc)
	if err != nil {
		return err
	}
	vc["proof"] = map[string]any{
		"type":               proofType,
		"created":            created.Format(time.RFC3339),
		"proofPurpose":       "assertionMethod",
		"verificationMethod": issuerKeyID,
		"jws":                base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, canonical)),
	}
	return nil
}

func verify(raw []byte, bundle TrustBundle, now time.Time) (VerificationResult, error) {
	var vc map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&vc); err != nil {
		return VerificationResult{}, err
	}
	issuerValue, _ := vc["issuer"].(string)
	issuer, err := bundle.findIssuer(issuerValue)
	if err != nil {
		return VerificationResult{}, err
	}
	proof, ok := vc["proof"].(map[string]any)
	if !ok {
		return VerificationResult{}, errors.New("credential proof missing")
	}
	if method, _ := proof["verificationMethod"].(string); method != issuer.VerificationMethod {
		return VerificationResult{}, fmt.Errorf("unexpected verification method %q", method)
	}
	if typ, _ := proof["type"].(string); typ != proofType {
		return VerificationResult{}, fmt.Errorf("unexpected proof type %q", typ)
	}
	signatureValue, _ := proof["jws"].(string)
	signature, err := base64.RawURLEncoding.DecodeString(signatureValue)
	if err != nil {
		return VerificationResult{}, err
	}
	publicKeyBytes, err := base64.RawURLEncoding.DecodeString(issuer.PublicKeyBase64URL)
	if err != nil {
		return VerificationResult{}, err
	}
	canonical, err := canonicalCredential(vc)
	if err != nil {
		return VerificationResult{}, err
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), canonical, signature) {
		return VerificationResult{}, errors.New("signature verification failed")
	}
	if expiration, _ := vc["expirationDate"].(string); expiration != "" {
		expiresAt, err := time.Parse(time.RFC3339, expiration)
		if err != nil {
			return VerificationResult{}, err
		}
		if !now.Before(expiresAt) {
			return VerificationResult{}, errors.New("credential expired")
		}
	}
	subject, ok := vc["credentialSubject"].(map[string]any)
	if !ok {
		return VerificationResult{}, errors.New("credential subject missing")
	}
	return VerificationResult{Credential: vc, Subject: subject}, nil
}

func (bundle TrustBundle) findIssuer(id string) (Issuer, error) {
	for _, issuer := range bundle.Issuers {
		if issuer.ID == id {
			return issuer, nil
		}
	}
	return Issuer{}, fmt.Errorf("issuer %q not trusted", id)
}

func canonicalCredential(vc map[string]any) ([]byte, error) {
	copy := make(map[string]any, len(vc))
	for key, value := range vc {
		if key != "proof" {
			copy[key] = value
		}
	}
	var buf bytes.Buffer
	if err := writeCanonicalJSON(&buf, copy); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonicalJSON(buf *bytes.Buffer, value any) error {
	switch v := value.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		encoded, _ := json.Marshal(v)
		buf.Write(encoded)
	case json.Number:
		buf.WriteString(v.String())
	case float64:
		buf.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	case []any:
		buf.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonicalJSON(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			encodedKey, _ := json.Marshal(key)
			buf.Write(encodedKey)
			buf.WriteByte(':')
			if err := writeCanonicalJSON(buf, v[key]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var decoded any
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err != nil {
			return err
		}
		return writeCanonicalJSON(buf, decoded)
	}
	return nil
}

type Auditor struct {
	dir   string
	actor string
}

func NewAuditor(dir, actor string) Auditor {
	_ = os.MkdirAll(dir, 0o755)
	return Auditor{dir: dir, actor: actor}
}

func (a Auditor) Log(peer, action, result string) {
	event := Event{
		Time:   time.Now().UTC().Format(time.RFC3339Nano),
		Actor:  a.actor,
		Peer:   peer,
		Action: action,
		Result: result,
	}
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("marshal audit event", "err", err)
		return
	}
	path := filepath.Join(a.dir, sanitizeFileName(a.actor)+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Error("open audit log", "path", path, "err", err)
		return
	}
	defer file.Close()
	_, _ = file.Write(append(data, '\n'))
}

func sanitizeFileName(value string) string {
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, ":", "-")
	return value
}

func listen(addr, service string, handler http.Handler) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("listening", "service", service, "addr", addr)
	return server.ListenAndServe()
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

func loadTrustBundle(path string) (TrustBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TrustBundle{}, err
	}
	var bundle TrustBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return TrustBundle{}, err
	}
	return bundle, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func getBytes(client *http.Client, endpoint string) ([]byte, error) {
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func getJSON(client *http.Client, endpoint string, target any) error {
	data, err := getBytes(client, endpoint)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func callTool(client *http.Client, endpoint, tool, input string) (string, error) {
	body, err := json.Marshal(map[string]string{"tool": tool, "input": input})
	if err != nil {
		return "", err
	}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var decoded struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	return decoded.Result, nil
}

func agentsFromSubject(subject map[string]any) ([]AgentSpec, error) {
	rawAgents, ok := subject["agents"].([]any)
	if !ok {
		return nil, errors.New("catalog subject missing agents")
	}
	agents := make([]AgentSpec, 0, len(rawAgents))
	for _, raw := range rawAgents {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New("agent entry is not an object")
		}
		tools, err := stringSliceValue(item["tools"])
		if err != nil {
			return nil, fmt.Errorf("parse tools for %s: %w", stringValue(item, "id"), err)
		}
		agents = append(agents, AgentSpec{
			ID:       stringValue(item, "id"),
			Name:     stringValue(item, "name"),
			Endpoint: stringValue(item, "endpoint"),
			Tools:    tools,
		})
	}
	return agents, nil
}

func toolsFromListResponse(response map[string]any) ([]string, error) {
	rawTools, ok := response["tools"].([]any)
	if !ok {
		return nil, errors.New("tools/list response missing tools")
	}
	tools := make([]string, 0, len(rawTools))
	for _, rawTool := range rawTools {
		item, ok := rawTool.(map[string]any)
		if !ok {
			return nil, errors.New("tools/list entry is not an object")
		}
		name, _ := item["name"].(string)
		if name == "" {
			return nil, errors.New("tools/list entry missing name")
		}
		tools = append(tools, name)
	}
	sort.Strings(tools)
	return tools, nil
}

func stringSliceValue(value any) ([]string, error) {
	switch v := value.(type) {
	case []any:
		values := make([]string, 0, len(v))
		for _, item := range v {
			text, ok := item.(string)
			if !ok {
				return nil, errors.New("array item is not a string")
			}
			values = append(values, text)
		}
		sort.Strings(values)
		return values, nil
	case string:
		values := []string{}
		for _, item := range strings.Split(v, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				values = append(values, item)
			}
		}
		sort.Strings(values)
		return values, nil
	default:
		return nil, errors.New("value is not a string array")
	}
}

func stringValue(item map[string]any, key string) string {
	value, _ := item[key].(string)
	return value
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	for i := range leftCopy {
		if leftCopy[i] != rightCopy[i] {
			return false
		}
	}
	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func readEvents(logsDir string) ([]Event, error) {
	matches, err := filepath.Glob(filepath.Join(logsDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	events := []Event{}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var event Event
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				return nil, fmt.Errorf("parse %s: %w", path, err)
			}
			events = append(events, event)
		}
	}
	return events, nil
}
