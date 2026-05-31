package app

import (
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	issuerID  = "did:web:nanda.local:level7-issuer"
	proofType = "Ed25519Signature2020"

	statusListCredentialURL = "http://revocation-authority:8080/status-lists/level7-revocation"
	statusListSizeBits      = 65536
	statusListValidity      = time.Hour
	crdtCredentialTTL       = 20 * time.Second
	crdtPublishInterval     = 3 * time.Second
	keyRotationInterval     = 12 * time.Second
	keyPrepublishDelay      = 2 * time.Second
	keyRetireAfter          = 24 * time.Second

	minCredentialTTL = 5 * time.Second
	maxCredentialTTL = 10 * time.Second
	minExpiredGap    = 2 * time.Second
	maxExpiredGap    = 4 * time.Second
	consumerInterval = 2 * time.Second
	swimlaneInterval = 1 * time.Second
)

type TrustBundle struct {
	Issuers []Issuer `json:"issuers"`
}

type Issuer struct {
	ID                 string `json:"id"`
	VerificationMethod string `json:"verificationMethod"`
	Type               string `json:"type"`
	PublicKeyBase64URL string `json:"publicKeyBase64URL"`
	State              string `json:"state,omitempty"`
	NotBefore          string `json:"notBefore,omitempty"`
	NotAfter           string `json:"notAfter,omitempty"`
}

type KeyFile struct {
	ActiveVerificationMethod string     `json:"activeVerificationMethod"`
	NextGeneration           int        `json:"nextGeneration"`
	Keys                     []KeyEntry `json:"keys"`
}

type KeyEntry struct {
	VerificationMethod        string `json:"verificationMethod"`
	Generation                int    `json:"generation"`
	State                     string `json:"state"`
	CreatedAt                 string `json:"createdAt"`
	RetireAfter               string `json:"retireAfter,omitempty"`
	IssuerPrivateKeyBase64URL string `json:"issuerPrivateKeyBase64URL"`
	IssuerPublicKeyBase64URL  string `json:"issuerPublicKeyBase64URL"`
}

type SigningKey struct {
	VerificationMethod string
	PrivateKey         ed25519.PrivateKey
	PublicKey          ed25519.PublicKey
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
	Key             string
	Name            string
	RegistryID      string
	CatalogURL      string
	PrivateFactsURL string
	CRDTUpdateURL   string
	FactsMode       string
	Description     string
	Agents          []AgentSpec
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

type RotationSummary struct {
	Generation             int
	RegistryAddrTTL        time.Duration
	RegistryAddrExpiresAt  time.Time
	EnterpriseCatalogTTL   time.Duration
	EnterpriseCatalogExpAt time.Time
}

type RevokedCredentialError struct {
	StatusListURL string
	Index         int
}

type CRDTOperation struct {
	ID          string
	Replica     string
	Kind        string
	Target      string
	Value       string
	LogicalTime int
	Dot         string
}

type CRDTMergeSummary struct {
	ReplicaCount       int
	OperationCount     int
	RoutingProfile     string
	TelemetryEndpoints []string
	CapabilityTags     []string
	ConflictCount      int
	ConflictTarget     string
	ConflictWinner     string
	StateHash          string
}

func (e RevokedCredentialError) Error() string {
	return fmt.Sprintf("credential revoked by status list index=%d url=%s", e.Index, e.StatusListURL)
}

func GenerateArtifacts(dir string) error {
	for _, subdir := range []string{"trust", "keys", "index", "registries/enterprise-a", "registries/enterprise-b", "status-lists"} {
		if err := os.MkdirAll(filepath.Join(dir, subdir), 0o755); err != nil {
			return err
		}
	}

	if _, err := loadOrCreateKeyRing(dir); err != nil {
		return err
	}

	if _, err := writeCredentialSet(dir, 0, 10*time.Second, 7*time.Second); err != nil {
		return err
	}
	ready := filepath.Join(dir, ".ready")
	if err := os.WriteFile(ready, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Println("level7 artifacts initialized at", dir)
	return nil
}

func RunIndex(artifactsDir, logsDir, addr string) error {
	audit := NewAuditor(logsDir, "nanda-index")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"ok": true, "service": "nanda-index"}, http.StatusOK
	}))
	mux.HandleFunc("GET /registries", jsonHandler(func(_ *http.Request) (any, int) {
		index, err := loadIndex(artifactsDir)
		if err != nil {
			audit.Log("consumer", "list_registries_failed", err.Error())
			return map[string]string{"error": err.Error()}, http.StatusInternalServerError
		}
		names := registryNames(index)
		audit.Log("consumer", "list_registries", strings.Join(names, ","))
		return map[string]any{"registries": names}, http.StatusOK
	}))
	mux.HandleFunc("GET /search", jsonHandler(func(r *http.Request) (any, int) {
		index, err := loadIndex(artifactsDir)
		if err != nil {
			audit.Log("consumer", "search_registries_failed", err.Error())
			return map[string]string{"error": err.Error()}, http.StatusInternalServerError
		}
		names := registryNames(index)
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
		index, err := loadIndex(artifactsDir)
		if err != nil {
			audit.Log("consumer", "resolve_registry_failed", err.Error())
			return map[string]string{"error": err.Error()}, http.StatusInternalServerError
		}
		records := registryRecords(index)
		record, ok := records[name]
		if !ok {
			audit.Log("consumer", "resolve_registry", "not_found:"+name)
			return map[string]string{"error": "registry not found"}, http.StatusNotFound
		}
		audit.Log("consumer", "serve_registry_addr", credentialLogSummary(record))
		return record, http.StatusOK
	}))
	return listen(addr, "nanda-index", mux)
}

func RunRegistry(artifactsDir, logsDir, enterprise, addr string) error {
	if enterprise == "" {
		return errors.New("registry requires --enterprise")
	}
	audit := NewAuditor(logsDir, enterprise+"-registry")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"ok": true, "service": enterprise + "-registry"}, http.StatusOK
	}))
	mux.HandleFunc("GET /catalog", func(w http.ResponseWriter, _ *http.Request) {
		catalog, err := os.ReadFile(filepath.Join(artifactsDir, "registries", enterprise, "catalog.vc.json"))
		if err != nil {
			audit.Log("consumer", "serve_signed_catalog_failed", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		audit.Log("consumer", "serve_signed_catalog", credentialRawLogSummary(catalog))
		w.Header().Set("Content-Type", "application/vc+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(catalog)
	})
	return listen(addr, enterprise+"-registry", mux)
}

func RunPrivateFactsGateway(artifactsDir, logsDir, addr string) error {
	audit := NewAuditor(logsDir, "private-facts-gateway")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"ok": true, "service": "private-facts-gateway"}, http.StatusOK
	}))
	mux.HandleFunc("GET /private-facts/", func(w http.ResponseWriter, r *http.Request) {
		enterprise := strings.TrimPrefix(r.URL.Path, "/private-facts/")
		enterprise = strings.TrimSuffix(enterprise, "/catalog")
		if enterprise == "" || !isPrivateFactsEnterprise(enterprise) {
			audit.Log("consumer", "private_facts_not_found", enterprise)
			http.Error(w, "private facts not found", http.StatusNotFound)
			return
		}
		catalog, err := os.ReadFile(filepath.Join(artifactsDir, "registries", enterprise, "catalog.vc.json"))
		if err != nil {
			audit.Log("consumer", "serve_private_facts_failed", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		audit.Log("consumer", "serve_private_facts", enterprise+" "+credentialRawLogSummary(catalog))
		w.Header().Set("Content-Type", "application/vc+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(catalog)
	})
	return listen(addr, "private-facts-gateway", mux)
}

func RunRevocationAuthority(artifactsDir, logsDir, addr string) error {
	audit := NewAuditor(logsDir, "revocation-authority")

	go runRevocationLoop(artifactsDir, audit)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"ok": true, "service": "revocation-authority"}, http.StatusOK
	}))
	mux.HandleFunc("GET /status-lists/level7-revocation", func(w http.ResponseWriter, _ *http.Request) {
		statusList, err := os.ReadFile(statusListPath(artifactsDir))
		if err != nil {
			audit.Log("consumer", "serve_status_list_failed", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		audit.Log("consumer", "serve_status_list", credentialRawLogSummary(statusList))
		w.Header().Set("Content-Type", "application/vc+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(statusList)
	})
	return listen(addr, "revocation-authority", mux)
}

func RunCRDTUpdateBus(artifactsDir, logsDir, addr string) error {
	audit := NewAuditor(logsDir, "crdt-update-bus")
	updates := newCRDTUpdateStore(artifactsDir, audit)
	if err := updates.publishAll(); err != nil {
		return err
	}
	go updates.loop()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", jsonHandler(func(_ *http.Request) (any, int) {
		return map[string]any{"ok": true, "service": "crdt-update-bus"}, http.StatusOK
	}))
	mux.HandleFunc("GET /crdt/", func(w http.ResponseWriter, r *http.Request) {
		enterprise := strings.TrimPrefix(r.URL.Path, "/crdt/")
		enterprise = strings.TrimSuffix(enterprise, "/updates")
		if enterprise == "" || enterpriseSpec(enterprise) == nil {
			audit.Log("consumer", "crdt_updates_not_found", enterprise)
			http.Error(w, "crdt updates not found", http.StatusNotFound)
			return
		}
		raw, ok := updates.get(enterprise)
		if !ok {
			audit.Log("consumer", "serve_crdt_update_failed", enterprise+" not ready")
			http.Error(w, "crdt update not ready", http.StatusServiceUnavailable)
			return
		}
		audit.Log("consumer", "serve_crdt_update", enterprise+" "+credentialRawLogSummary(raw))
		w.Header().Set("Content-Type", "application/vc+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	})
	return listen(addr, "crdt-update-bus", mux)
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

func RunRotator(artifactsDir, logsDir string) error {
	audit := NewAuditor(logsDir, "credential-rotator")
	generation := 1
	for {
		addrTTL, catalogTTL, err := randomCredentialTTLs(generation)
		if err != nil {
			return err
		}
		summary, err := writeCredentialSet(artifactsDir, generation, addrTTL, catalogTTL)
		if err != nil {
			audit.Log("artifacts", "credential_rotation_failed", err.Error())
			time.Sleep(time.Second)
			continue
		}
		audit.Log("nanda-index", "credential_rotated_registry_addr", fmt.Sprintf("generation=%d ttl=%s expires=%s", generation, summary.RegistryAddrTTL, summary.RegistryAddrExpiresAt.Format(time.RFC3339)))
		for _, enterprise := range enterpriseSpecs() {
			audit.Log(enterprise.RegistryID, "credential_rotated_catalog", fmt.Sprintf("generation=%d ttl=%s expires=%s", generation, summary.EnterpriseCatalogTTL, summary.EnterpriseCatalogExpAt.Format(time.RFC3339)))
		}
		gap, err := randomDuration(minExpiredGap, maxExpiredGap)
		if err != nil {
			return err
		}
		audit.Log("all-services", "next_rotation_after_expiry_gap", fmt.Sprintf("generation=%d gap=%s", generation, gap))
		generation++
		time.Sleep(minDuration(summary.RegistryAddrTTL, summary.EnterpriseCatalogTTL) + gap)
	}
}

func RunKeyRotator(artifactsDir, logsDir string) error {
	audit := NewAuditor(logsDir, "issuer-key-rotator")
	time.Sleep(6 * time.Second)
	for {
		if err := rotateIssuerKey(artifactsDir, audit); err != nil {
			audit.Log("trust-bundle", "issuer_key_rotation_failed", err.Error())
			time.Sleep(time.Second)
			continue
		}
		time.Sleep(keyRotationInterval)
	}
}

func runRevocationLoop(artifactsDir string, audit Auditor) {
	revokedGenerations := map[string]bool{}
	lastErr := ""
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		err := maybePushEnterpriseBCatalogRevocation(artifactsDir, audit, revokedGenerations)
		if err == nil {
			lastErr = ""
			continue
		}
		if err.Error() != lastErr {
			audit.Log("status-list", "push_revocation_waiting", err.Error())
			lastErr = err.Error()
		}
	}
}

func maybePushEnterpriseBCatalogRevocation(artifactsDir string, audit Auditor, revokedGenerations map[string]bool) error {
	raw, err := os.ReadFile(filepath.Join(artifactsDir, "registries", "enterprise-b", "catalog.vc.json"))
	if err != nil {
		return err
	}
	vc, err := decodeCredentialMap(raw)
	if err != nil {
		return err
	}
	subject, ok := vc["credentialSubject"].(map[string]any)
	if !ok {
		return errors.New("enterprise-b catalog subject missing")
	}
	generation, ok := intValue(subject["credentialVersion"])
	if !ok {
		return errors.New("enterprise-b catalog generation missing")
	}
	revocationKey := fmt.Sprintf("enterprise-b:catalog:%d", generation)
	if revokedGenerations[revocationKey] {
		return nil
	}
	issuedAt, err := time.Parse(time.RFC3339, stringValue(vc, "issuanceDate"))
	if err != nil {
		return err
	}
	expiresAt, err := time.Parse(time.RFC3339, stringValue(vc, "expirationDate"))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if now.Sub(issuedAt) < 3*time.Second || time.Until(expiresAt) < 2*time.Second {
		return nil
	}

	index, statusListURL, err := credentialStatusIndexAndURL(vc)
	if err != nil {
		return err
	}
	bits, err := loadStatusBits(artifactsDir)
	if err != nil {
		return err
	}
	alreadyRevoked, err := statusBit(bits, index)
	if err != nil {
		return err
	}
	if alreadyRevoked {
		revokedGenerations[revocationKey] = true
		return nil
	}
	if err := setStatusBit(bits, index, true); err != nil {
		return err
	}
	if err := writeStatusListCredential(artifactsDir, now.Truncate(time.Second), bits); err != nil {
		return err
	}
	revokedGenerations[revocationKey] = true
	audit.Log("status-list", "push_revocation", fmt.Sprintf("enterprise-b catalog generation=%d statusListIndex=%d statusList=%s expires=%s", generation, index, statusListURL, expiresAt.Format(time.RFC3339)))
	return nil
}

type CRDTUpdateStore struct {
	artifactsDir string
	audit        Auditor
	mu           sync.RWMutex
	epoch        int
	rawByKey     map[string][]byte
}

func newCRDTUpdateStore(artifactsDir string, audit Auditor) *CRDTUpdateStore {
	return &CRDTUpdateStore{
		artifactsDir: artifactsDir,
		audit:        audit,
		rawByKey:     map[string][]byte{},
	}
}

func (s *CRDTUpdateStore) loop() {
	ticker := time.NewTicker(crdtPublishInterval)
	defer ticker.Stop()
	for range ticker.C {
		if err := s.publishAll(); err != nil {
			s.audit.Log("consumer", "publish_crdt_update_failed", err.Error())
		}
	}
}

func (s *CRDTUpdateStore) publishAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epoch++
	now := time.Now().UTC().Truncate(time.Second)
	signingKey, err := loadActiveSigningKey(s.artifactsDir)
	if err != nil {
		return err
	}
	for _, enterprise := range enterpriseSpecs() {
		vc, err := buildCRDTUpdateCredential(enterprise, s.epoch, now, signingKey)
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(vc, "", "  ")
		if err != nil {
			return err
		}
		raw = append(raw, '\n')
		s.rawByKey[enterprise.Key] = raw
		s.audit.Log(enterprise.RegistryID, "publish_crdt_update", fmt.Sprintf("enterprise=%s epoch=%d ops=%d key=%s url=%s", enterprise.Key, s.epoch, len(crdtOperations(enterprise.Key, s.epoch)), signingKey.VerificationMethod, enterprise.CRDTUpdateURL))
	}
	return nil
}

func (s *CRDTUpdateStore) get(enterpriseKey string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	raw, ok := s.rawByKey[enterpriseKey]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), raw...), true
}

func buildCRDTUpdateCredential(enterprise EnterpriseSpec, epoch int, now time.Time, signingKey SigningKey) (map[string]any, error) {
	ops := crdtOperations(enterprise.Key, epoch)
	opsAny := make([]any, 0, len(ops))
	for _, op := range ops {
		opsAny = append(opsAny, map[string]any{
			"id":          op.ID,
			"replica":     op.Replica,
			"kind":        op.Kind,
			"target":      op.Target,
			"value":       op.Value,
			"logicalTime": op.LogicalTime,
			"dot":         op.Dot,
		})
	}
	vc := baseCredential("urn:nanda:crdt-updates:"+enterprise.Key, "AgentFactsCRDTUpdateCredential", now, now.Add(crdtCredentialTTL), map[string]any{
		"id":                "crdt:" + enterprise.Key,
		"registryID":        enterprise.RegistryID,
		"name":              enterprise.Name,
		"crdtProtocol":      "lww-register+or-set",
		"credentialVersion": epoch,
		"ttlSeconds":        int(crdtCredentialTTL / time.Second),
		"operations":        opsAny,
	})
	addCredentialStatus(vc, statusListIndex(epoch, enterprise.Key, "crdt-updates"))
	if err := sign(vc, signingKey, now); err != nil {
		return nil, err
	}
	return vc, nil
}

func crdtOperations(enterpriseKey string, epoch int) []CRDTOperation {
	eastReplica := enterpriseKey + "-east"
	westReplica := enterpriseKey + "-west"
	baseTime := epoch * 100
	return []CRDTOperation{
		{
			ID:          fmt.Sprintf("%s:%d:routing-blue", eastReplica, epoch),
			Replica:     eastReplica,
			Kind:        "lww-register",
			Target:      "routingProfile",
			Value:       "blue",
			LogicalTime: baseTime + 1,
			Dot:         fmt.Sprintf("%s:%d", eastReplica, epoch),
		},
		{
			ID:          fmt.Sprintf("%s:%d:routing-green", westReplica, epoch),
			Replica:     westReplica,
			Kind:        "lww-register",
			Target:      "routingProfile",
			Value:       "green",
			LogicalTime: baseTime + 2,
			Dot:         fmt.Sprintf("%s:%d", westReplica, epoch),
		},
		{
			ID:          fmt.Sprintf("%s:%d:telemetry", eastReplica, epoch),
			Replica:     eastReplica,
			Kind:        "or-set-add",
			Target:      "telemetryEndpoints",
			Value:       "http://" + enterpriseKey + ".telemetry.mcp.local/metrics",
			LogicalTime: baseTime + 3,
			Dot:         fmt.Sprintf("%s:%d:telemetry", eastReplica, epoch),
		},
		{
			ID:          fmt.Sprintf("%s:%d:capability-tag", westReplica, epoch),
			Replica:     westReplica,
			Kind:        "or-set-add",
			Target:      "capabilityTags",
			Value:       "crdt-updated-facts",
			LogicalTime: baseTime + 4,
			Dot:         fmt.Sprintf("%s:%d:capability-tag", westReplica, epoch),
		},
	}
}

func RunConsumer(artifactsDir, logsDir, indexURL string) error {
	audit := NewAuditor(logsDir, "consumer")
	client := &http.Client{Timeout: 10 * time.Second}
	state := map[string]bool{}
	trustPath := filepath.Join(artifactsDir, "trust", "issuers.json")
	lastTrustSummary := ""

	for cycle := 1; ; cycle++ {
		bundle, err := loadTrustBundle(trustPath)
		if err != nil {
			audit.Log("trust-bundle", "trust_bundle_reload_failed", err.Error())
			time.Sleep(consumerInterval)
			continue
		}
		trustSummary := trustBundleSummary(bundle)
		if trustSummary != lastTrustSummary {
			audit.Log("trust-bundle", "trust_bundle_reloaded", trustSummary)
			lastTrustSummary = trustSummary
		}
		if err := runConsumerCycle(client, audit, bundle, state, indexURL, cycle); err != nil {
			audit.Log("nanda-index", "consumer_cycle_failed", err.Error())
		}
		time.Sleep(consumerInterval)
	}
}

func runConsumerCycle(client *http.Client, audit Auditor, bundle TrustBundle, state map[string]bool, indexURL string, cycle int) error {
	audit.Log("nanda-index", "search_registries", fmt.Sprintf("cycle=%d registrationType=enterprise-mcp-registry", cycle))
	var registries struct {
		Registries []string `json:"registries"`
	}
	if err := getJSON(client, indexURL+"/search?registrationType=enterprise-mcp-registry", &registries); err != nil {
		return err
	}
	audit.Log("nanda-index", "search_registries_result", fmt.Sprintf("cycle=%d %s", cycle, strings.Join(registries.Registries, ",")))

	verifiedAgents := []AgentSpec{}
	for _, registryName := range registries.Registries {
		audit.Log("nanda-index", "resolve_registry", registryName)
		rawAddr, err := getBytes(client, indexURL+"/resolve/"+url.PathEscape(registryName))
		if err != nil {
			audit.Log("nanda-index", "registry_addr_fetch_failed", registryName+" "+err.Error())
			continue
		}
		addr, err := verifyWithStatus(client, audit, rawAddr, bundle, time.Now(), "registry address "+registryName)
		if err != nil {
			logRevocationFailureIfNeeded(audit, state, "registry_addr:"+registryName, registryName, err)
			logVerificationFailure(audit, state, "registry_addr:"+registryName, "nanda-index", "registry_addr", registryName, err)
			continue
		}
		logVerificationSuccess(audit, state, "registry_addr:"+registryName, "nanda-index", "registry_addr", registryName)
		logVerificationSuccess(audit, state, "revocation:registry_addr:"+registryName, "revocation-authority", "revoked_status_list", registryName)
		catalogURL, _ := addr.Subject["catalogURL"].(string)
		privateFactsURL, _ := addr.Subject["privateFactsURL"].(string)
		factsMode, _ := addr.Subject["factsMode"].(string)
		registryID, _ := addr.Subject["registryID"].(string)
		audit.Log("nanda-index", "verified_registry_addr", registryID+" "+verificationLogSummary(addr.Credential))

		selectedURL := catalogURL
		selectedPeer := registryID
		selectedKind := "public_facts"
		if factsMode == "private" && privateFactsURL != "" {
			selectedURL = privateFactsURL
			selectedPeer = "private-facts-gateway"
			selectedKind = "private_facts"
			audit.Log(selectedPeer, "selected_private_facts_url", registryID+" "+privateFactsURL)
			audit.Log(registryID, "direct_catalog_url_not_used", catalogURL)
		} else {
			audit.Log(registryID, "selected_public_catalog_url", catalogURL)
		}

		audit.Log(selectedPeer, "fetch_"+selectedKind, selectedURL)
		rawCatalog, err := getBytes(client, selectedURL)
		if err != nil {
			audit.Log(selectedPeer, selectedKind+"_fetch_failed", err.Error())
			continue
		}
		catalog, err := verifyWithStatus(client, audit, rawCatalog, bundle, time.Now(), "enterprise catalog "+registryName)
		if err != nil {
			logRevocationFailureIfNeeded(audit, state, "catalog:"+registryID, registryName, err)
			logVerificationFailure(audit, state, "catalog:"+registryID, selectedPeer, "enterprise_catalog", registryName, err)
			continue
		}
		logVerificationSuccess(audit, state, "catalog:"+registryID, selectedPeer, "enterprise_catalog", registryName)
		logVerificationSuccess(audit, state, "revocation:catalog:"+registryID, "revocation-authority", "revoked_status_list", registryName)
		if err := fetchAndMergeCRDTUpdates(client, audit, bundle, state, catalog.Subject, registryName, registryID); err != nil {
			audit.Log("crdt-update-bus", "crdt_updates_failed", registryName+" "+err.Error())
			continue
		}
		agents, err := agentsFromSubject(catalog.Subject)
		if err != nil {
			audit.Log(selectedPeer, "catalog_parse_failed", err.Error())
			continue
		}
		audit.Log(selectedPeer, "verified_"+selectedKind, fmt.Sprintf("%s %d agents %s", registryID, len(agents), verificationLogSummary(catalog.Credential)))
		verifiedAgents = append(verifiedAgents, agents...)
	}

	for _, agent := range verifiedAgents {
		toolsListURL := strings.TrimRight(agent.Endpoint, "/") + "/mcp/tools/list"
		audit.Log(agent.ID, "confirm_skills", strings.Join(agent.Tools, ","))
		var toolsResp map[string]any
		if err := getJSON(client, toolsListURL, &toolsResp); err != nil {
			audit.Log(agent.ID, "tools_list_failed", err.Error())
			continue
		}
		listedTools, err := toolsFromListResponse(toolsResp)
		if err != nil {
			audit.Log(agent.ID, "tools_list_parse_failed", err.Error())
			continue
		}
		if !sameStringSet(agent.Tools, listedTools) {
			audit.Log(agent.ID, "tools_list_mismatch", fmt.Sprintf("catalog=%v mcp=%v", agent.Tools, listedTools))
			continue
		}
		audit.Log(agent.ID, "skills_confirmed", strings.Join(listedTools, ","))
		if len(agent.Tools) == 0 {
			continue
		}
		callURL := strings.TrimRight(agent.Endpoint, "/") + "/mcp/tools/call"
		result, err := callTool(client, callURL, agent.Tools[0], "NANDA level seven demo text")
		if err != nil {
			audit.Log(agent.ID, "tool_call_failed", err.Error())
			continue
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
	return nil
}

func fetchAndMergeCRDTUpdates(client *http.Client, audit Auditor, bundle TrustBundle, state map[string]bool, catalogSubject map[string]any, registryName, registryID string) error {
	crdtUpdateURL, _ := catalogSubject["crdtUpdateURL"].(string)
	if crdtUpdateURL == "" {
		audit.Log("crdt-update-bus", "crdt_updates_not_configured", registryName)
		return nil
	}
	audit.Log("crdt-update-bus", "fetch_crdt_updates", registryID+" "+crdtUpdateURL)
	rawUpdates, err := getBytes(client, crdtUpdateURL)
	if err != nil {
		logVerificationFailure(audit, state, "crdt:"+registryID, "crdt-update-bus", "crdt_updates", registryName, err)
		return err
	}
	updates, err := verifyWithStatus(client, audit, rawUpdates, bundle, time.Now(), "crdt updates "+registryName)
	if err != nil {
		logRevocationFailureIfNeeded(audit, state, "crdt:"+registryID, registryName, err)
		logVerificationFailure(audit, state, "crdt:"+registryID, "crdt-update-bus", "crdt_updates", registryName, err)
		return err
	}
	logVerificationSuccess(audit, state, "crdt:"+registryID, "crdt-update-bus", "crdt_updates", registryName)
	logVerificationSuccess(audit, state, "revocation:crdt:"+registryID, "revocation-authority", "revoked_status_list", registryName)
	audit.Log("crdt-update-bus", "verified_crdt_updates", registryID+" "+verificationLogSummary(updates.Credential))
	summary, err := mergeCRDTUpdates(updates.Subject)
	if err != nil {
		audit.Log("crdt-update-bus", "merge_crdt_failed", registryName+" "+err.Error())
		return err
	}
	audit.Log("crdt-update-bus", "merge_crdt_ops", fmt.Sprintf("%s replicas=%d ops=%d stateHash=%s", registryID, summary.ReplicaCount, summary.OperationCount, summary.StateHash))
	if summary.ConflictCount > 0 {
		audit.Log("crdt-update-bus", "crdt_conflict_resolved", fmt.Sprintf("%s target=%s winner=%s", registryID, summary.ConflictTarget, summary.ConflictWinner))
	}
	audit.Log("crdt-update-bus", "crdt_state_applied", fmt.Sprintf("%s routing=%s telemetry=%d tags=%s", registryID, summary.RoutingProfile, len(summary.TelemetryEndpoints), strings.Join(summary.CapabilityTags, ",")))
	return nil
}

func RunSwimlane(logsDir string) error {
	fmt.Println("Level 7 audit swimlane")
	fmt.Println("time                         | actor                  | activity")
	fmt.Println("-----------------------------+------------------------+-----------------------------------------------")
	seen := map[string]bool{}
	for {
		events, err := readEvents(logsDir)
		if err != nil {
			fmt.Printf("%-28s | %-22s | --(%s: %s)--> %s\n", time.Now().UTC().Format(time.RFC3339Nano), "swimlane", "read_logs_failed", err.Error(), "logs")
			time.Sleep(swimlaneInterval)
			continue
		}
		sort.Slice(events, func(i, j int) bool {
			return events[i].Time < events[j].Time
		})
		for _, event := range events {
			key := event.Time + "|" + event.Actor + "|" + event.Peer + "|" + event.Action + "|" + event.Result
			if seen[key] {
				continue
			}
			seen[key] = true
			marker := "   "
			if isHighlightedEvent(event) {
				marker = "!!!"
			}
			fmt.Printf("%s %-28s | %-22s | --(%s: %s)--> %s\n", marker, event.Time, event.Actor, event.Action, event.Result, event.Peer)
		}
		time.Sleep(swimlaneInterval)
	}
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

func loadOrCreateKeyRing(artifactsDir string) (KeyFile, error) {
	keyRing, err := loadKeyRing(artifactsDir)
	if err == nil {
		if err := writeTrustBundleFromKeyRing(artifactsDir, keyRing); err != nil {
			return KeyFile{}, err
		}
		return keyRing, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return KeyFile{}, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	entry, err := newKeyEntry(1, "active", now)
	if err != nil {
		return KeyFile{}, err
	}
	keyRing = KeyFile{
		ActiveVerificationMethod: entry.VerificationMethod,
		NextGeneration:           2,
		Keys:                     []KeyEntry{entry},
	}
	if err := writeKeyRing(artifactsDir, keyRing); err != nil {
		return KeyFile{}, err
	}
	if err := writeTrustBundleFromKeyRing(artifactsDir, keyRing); err != nil {
		return KeyFile{}, err
	}
	return keyRing, nil
}

func loadKeyRing(artifactsDir string) (KeyFile, error) {
	data, err := os.ReadFile(keyRingPath(artifactsDir))
	if err != nil {
		return KeyFile{}, err
	}
	var keyRing KeyFile
	if err := json.Unmarshal(data, &keyRing); err != nil {
		return KeyFile{}, err
	}
	return keyRing, nil
}

func writeKeyRing(artifactsDir string, keyRing KeyFile) error {
	return writeJSONAtomic(keyRingPath(artifactsDir), keyRing)
}

func keyRingPath(artifactsDir string) string {
	return filepath.Join(artifactsDir, "keys", "issuer.json")
}

func newKeyEntry(generation int, state string, now time.Time) (KeyEntry, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		return KeyEntry{}, err
	}
	return KeyEntry{
		VerificationMethod:        issuerID + "#key-" + strconv.Itoa(generation),
		Generation:                generation,
		State:                     state,
		CreatedAt:                 now.Format(time.RFC3339),
		IssuerPrivateKeyBase64URL: base64.RawURLEncoding.EncodeToString(privateKey),
		IssuerPublicKeyBase64URL:  base64.RawURLEncoding.EncodeToString(publicKey),
	}, nil
}

func loadActiveSigningKey(artifactsDir string) (SigningKey, error) {
	keyRing, err := loadKeyRing(artifactsDir)
	if err != nil {
		return SigningKey{}, err
	}
	for _, entry := range keyRing.Keys {
		if entry.VerificationMethod == keyRing.ActiveVerificationMethod && entry.State == "active" {
			return signingKeyFromEntry(entry)
		}
	}
	return SigningKey{}, fmt.Errorf("active signing key %q not found", keyRing.ActiveVerificationMethod)
}

func signingKeyFromEntry(entry KeyEntry) (SigningKey, error) {
	privateKeyBytes, err := base64.RawURLEncoding.DecodeString(entry.IssuerPrivateKeyBase64URL)
	if err != nil {
		return SigningKey{}, err
	}
	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return SigningKey{}, fmt.Errorf("unexpected private key size %d", len(privateKeyBytes))
	}
	publicKeyBytes, err := base64.RawURLEncoding.DecodeString(entry.IssuerPublicKeyBase64URL)
	if err != nil {
		return SigningKey{}, err
	}
	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return SigningKey{}, fmt.Errorf("unexpected public key size %d", len(publicKeyBytes))
	}
	return SigningKey{
		VerificationMethod: entry.VerificationMethod,
		PrivateKey:         ed25519.PrivateKey(privateKeyBytes),
		PublicKey:          ed25519.PublicKey(publicKeyBytes),
	}, nil
}

func writeTrustBundleFromKeyRing(artifactsDir string, keyRing KeyFile) error {
	bundle := TrustBundle{Issuers: []Issuer{}}
	for _, entry := range keyRing.Keys {
		if entry.State == "retired" {
			continue
		}
		issuer := Issuer{
			ID:                 issuerID,
			VerificationMethod: entry.VerificationMethod,
			Type:               "Ed25519VerificationKey2020",
			PublicKeyBase64URL: entry.IssuerPublicKeyBase64URL,
			State:              entry.State,
			NotBefore:          entry.CreatedAt,
			NotAfter:           entry.RetireAfter,
		}
		bundle.Issuers = append(bundle.Issuers, issuer)
	}
	sort.Slice(bundle.Issuers, func(i, j int) bool {
		return bundle.Issuers[i].VerificationMethod < bundle.Issuers[j].VerificationMethod
	})
	return writeJSONAtomic(filepath.Join(artifactsDir, "trust", "issuers.json"), bundle)
}

func rotateIssuerKey(artifactsDir string, audit Auditor) error {
	now := time.Now().UTC().Truncate(time.Second)
	keyRing, err := loadKeyRing(artifactsDir)
	if err != nil {
		return err
	}
	keyRing = retireExpiredKeys(keyRing, now, audit)
	newEntry, err := newKeyEntry(keyRing.NextGeneration, "prepublished", now)
	if err != nil {
		return err
	}
	keyRing.NextGeneration++
	keyRing.Keys = append(keyRing.Keys, newEntry)
	if err := writeKeyRing(artifactsDir, keyRing); err != nil {
		return err
	}
	if err := writeTrustBundleFromKeyRing(artifactsDir, keyRing); err != nil {
		return err
	}
	audit.Log("trust-bundle", "issuer_key_prepublished", newEntry.VerificationMethod)

	time.Sleep(keyPrepublishDelay)

	now = time.Now().UTC().Truncate(time.Second)
	keyRing, err = loadKeyRing(artifactsDir)
	if err != nil {
		return err
	}
	previousActive := keyRing.ActiveVerificationMethod
	for i := range keyRing.Keys {
		switch keyRing.Keys[i].VerificationMethod {
		case newEntry.VerificationMethod:
			keyRing.Keys[i].State = "active"
			keyRing.Keys[i].RetireAfter = ""
		case previousActive:
			keyRing.Keys[i].State = "previous"
			keyRing.Keys[i].RetireAfter = now.Add(keyRetireAfter).Format(time.RFC3339)
		}
	}
	keyRing.ActiveVerificationMethod = newEntry.VerificationMethod
	keyRing = retireExpiredKeys(keyRing, now, audit)
	if err := writeKeyRing(artifactsDir, keyRing); err != nil {
		return err
	}
	if err := writeTrustBundleFromKeyRing(artifactsDir, keyRing); err != nil {
		return err
	}
	bits, err := loadStatusBits(artifactsDir)
	if err != nil {
		return err
	}
	if err := writeStatusListCredential(artifactsDir, now, bits); err != nil {
		return err
	}
	audit.Log("trust-bundle", "issuer_key_rotated", fmt.Sprintf("from=%s to=%s", previousActive, newEntry.VerificationMethod))
	return nil
}

func retireExpiredKeys(keyRing KeyFile, now time.Time, audit Auditor) KeyFile {
	for i := range keyRing.Keys {
		entry := &keyRing.Keys[i]
		if entry.State != "previous" || entry.RetireAfter == "" {
			continue
		}
		retireAfter, err := time.Parse(time.RFC3339, entry.RetireAfter)
		if err != nil {
			continue
		}
		if now.Before(retireAfter) {
			continue
		}
		entry.State = "retired"
		audit.Log("trust-bundle", "old_issuer_key_retired", entry.VerificationMethod)
	}
	return keyRing
}

func writeCredentialSet(dir string, generation int, registryAddrTTL, enterpriseCatalogTTL time.Duration) (RotationSummary, error) {
	now := time.Now().UTC().Truncate(time.Second)
	registryAddrExpiresAt := now.Add(registryAddrTTL)
	enterpriseCatalogExpiresAt := now.Add(enterpriseCatalogTTL)
	signingKey, err := loadActiveSigningKey(dir)
	if err != nil {
		return RotationSummary{}, err
	}
	if err := writeStatusListCredential(dir, now, make([]byte, statusListSizeBits/8)); err != nil {
		return RotationSummary{}, err
	}
	index := IndexFile{}
	for _, enterprise := range enterpriseSpecs() {
		addr := baseCredential("urn:nanda:registry-addr:"+enterprise.Key, "EnterpriseRegistryAddrCredential", now, registryAddrExpiresAt, map[string]any{
			"id":                "registry:" + enterprise.Key,
			"registryID":        enterprise.RegistryID,
			"name":              enterprise.Name,
			"registrationType":  "enterprise-mcp-registry",
			"catalogURL":        enterprise.CatalogURL,
			"privateFactsURL":   enterprise.PrivateFactsURL,
			"factsMode":         enterprise.FactsMode,
			"description":       enterprise.Description,
			"credentialVersion": generation,
			"ttlSeconds":        int(registryAddrTTL / time.Second),
		})
		addCredentialStatus(addr, statusListIndex(generation, enterprise.Key, "registry-addr"))
		if err := sign(addr, signingKey, now); err != nil {
			return RotationSummary{}, err
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
		catalog := baseCredential("urn:nanda:enterprise-catalog:"+enterprise.Key, "EnterpriseMCPCatalogCredential", now, enterpriseCatalogExpiresAt, map[string]any{
			"id":                "catalog:" + enterprise.Key,
			"registryID":        enterprise.RegistryID,
			"name":              enterprise.Name,
			"description":       enterprise.Description,
			"crdtUpdateURL":     enterprise.CRDTUpdateURL,
			"agents":            agents,
			"credentialVersion": generation,
			"ttlSeconds":        int(enterpriseCatalogTTL / time.Second),
		})
		addCredentialStatus(catalog, statusListIndex(generation, enterprise.Key, "catalog"))
		if err := sign(catalog, signingKey, now); err != nil {
			return RotationSummary{}, err
		}
		if err := writeJSONAtomic(filepath.Join(dir, "registries", enterprise.Key, "catalog.vc.json"), catalog); err != nil {
			return RotationSummary{}, err
		}
	}
	if err := writeJSONAtomic(filepath.Join(dir, "index", "registries.json"), index); err != nil {
		return RotationSummary{}, err
	}
	return RotationSummary{
		Generation:             generation,
		RegistryAddrTTL:        registryAddrTTL,
		RegistryAddrExpiresAt:  registryAddrExpiresAt,
		EnterpriseCatalogTTL:   enterpriseCatalogTTL,
		EnterpriseCatalogExpAt: enterpriseCatalogExpiresAt,
	}, nil
}

func loadIndex(artifactsDir string) (IndexFile, error) {
	data, err := os.ReadFile(filepath.Join(artifactsDir, "index", "registries.json"))
	if err != nil {
		return IndexFile{}, err
	}
	var index IndexFile
	if err := json.Unmarshal(data, &index); err != nil {
		return IndexFile{}, err
	}
	return index, nil
}

func registryNames(index IndexFile) []string {
	names := make([]string, 0, len(index.Registries))
	for _, registry := range index.Registries {
		names = append(names, registry.Name)
	}
	return names
}

func registryRecords(index IndexFile) map[string]map[string]any {
	records := map[string]map[string]any{}
	for _, registry := range index.Registries {
		records[registry.Name] = registry.Credential
		for _, alias := range registry.Aliases {
			records[alias] = registry.Credential
		}
	}
	return records
}

func randomDuration(min, max time.Duration) (time.Duration, error) {
	if max < min {
		return 0, errors.New("max duration is less than min duration")
	}
	minSeconds := int64(min / time.Second)
	maxSeconds := int64(max / time.Second)
	span := maxSeconds - minSeconds + 1
	value, err := cryptorand.Int(cryptorand.Reader, big.NewInt(span))
	if err != nil {
		return 0, err
	}
	return time.Duration(minSeconds+value.Int64()) * time.Second, nil
}

func randomCredentialTTLs(generation int) (time.Duration, time.Duration, error) {
	shortTTL, err := randomDuration(5*time.Second, 7*time.Second)
	if err != nil {
		return 0, 0, err
	}
	longTTL, err := randomDuration(8*time.Second, 10*time.Second)
	if err != nil {
		return 0, 0, err
	}
	if generation%2 == 0 {
		return shortTTL, longTTL, nil
	}
	return longTTL, shortTTL, nil
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func isPrivateFactsEnterprise(key string) bool {
	for _, enterprise := range enterpriseSpecs() {
		if enterprise.Key == key {
			return enterprise.FactsMode == "private"
		}
	}
	return false
}

func enterpriseSpec(key string) *EnterpriseSpec {
	for _, enterprise := range enterpriseSpecs() {
		if enterprise.Key == key {
			spec := enterprise
			return &spec
		}
	}
	return nil
}

func enterpriseSpecs() []EnterpriseSpec {
	return []EnterpriseSpec{
		{
			Key:             "enterprise-a",
			Name:            "enterprise-a.registry.nanda.local",
			RegistryID:      "enterprise-a-registry",
			CatalogURL:      "http://enterprise-a-registry:8080/catalog",
			PrivateFactsURL: "",
			CRDTUpdateURL:   "http://crdt-update-bus:8080/crdt/enterprise-a/updates",
			FactsMode:       "public",
			Description:     "Enterprise A registry exposes public signed catalog facts directly.",
			Agents: []AgentSpec{
				{ID: "enterprise-a-reverse", Name: "enterprise-a.reverse.mcp.local", Endpoint: "http://enterprise-a-reverse:8080", Tools: []string{"reverse"}},
				{ID: "enterprise-a-uppercase", Name: "enterprise-a.uppercase.mcp.local", Endpoint: "http://enterprise-a-uppercase:8080", Tools: []string{"uppercase"}},
			},
		},
		{
			Key:             "enterprise-b",
			Name:            "enterprise-b.registry.nanda.local",
			RegistryID:      "enterprise-b-registry",
			CatalogURL:      "http://enterprise-b-registry:8080/catalog",
			PrivateFactsURL: "http://private-facts-gateway:8080/private-facts/enterprise-b/catalog",
			CRDTUpdateURL:   "http://crdt-update-bus:8080/crdt/enterprise-b/updates",
			FactsMode:       "private",
			Description:     "Enterprise B registry keeps catalog facts behind the neutral PrivateFactsURL gateway.",
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

func baseCredential(id, credentialType string, now, expiresAt time.Time, subject map[string]any) map[string]any {
	return map[string]any{
		"@context": []any{
			"https://www.w3.org/2018/credentials/v1",
			"https://nanda.local/contexts/enterprise-mcp/v1",
		},
		"id":                id,
		"type":              []any{"VerifiableCredential", credentialType},
		"issuer":            issuerID,
		"issuanceDate":      now.Format(time.RFC3339),
		"expirationDate":    expiresAt.Format(time.RFC3339),
		"credentialSubject": subject,
	}
}

func addCredentialStatus(vc map[string]any, index int) {
	vc["credentialStatus"] = map[string]any{
		"id":                   fmt.Sprintf("%s#%d", statusListCredentialURL, index),
		"type":                 "StatusList2021Entry",
		"statusPurpose":        "revocation",
		"statusListIndex":      strconv.Itoa(index),
		"statusListCredential": statusListCredentialURL,
	}
}

func statusListIndex(generation int, enterpriseKey, credentialKind string) int {
	offset := 0
	switch enterpriseKey + ":" + credentialKind {
	case "enterprise-a:registry-addr":
		offset = 1
	case "enterprise-b:registry-addr":
		offset = 2
	case "enterprise-a:catalog":
		offset = 3
	case "enterprise-b:catalog":
		offset = 4
	case "enterprise-a:crdt-updates":
		offset = 5
	case "enterprise-b:crdt-updates":
		offset = 6
	default:
		offset = 9
	}
	return generation*10 + offset
}

func writeStatusListCredential(dir string, now time.Time, bits []byte) error {
	encoded, err := encodeStatusBits(bits)
	if err != nil {
		return err
	}
	signingKey, err := loadActiveSigningKey(dir)
	if err != nil {
		return err
	}
	statusList := baseCredential(statusListCredentialURL, "StatusList2021Credential", now, now.Add(statusListValidity), map[string]any{
		"id":                statusListCredentialURL + "#list",
		"type":              "StatusList2021",
		"statusPurpose":     "revocation",
		"credentialVersion": now.Unix(),
		"encodedList":       encoded,
	})
	if err := sign(statusList, signingKey, now); err != nil {
		return err
	}
	return writeJSONAtomic(statusListPath(dir), statusList)
}

func statusListPath(dir string) string {
	return filepath.Join(dir, "status-lists", "level7-revocation.vc.json")
}

func loadStatusBits(dir string) ([]byte, error) {
	raw, err := os.ReadFile(statusListPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return make([]byte, statusListSizeBits/8), nil
	}
	if err != nil {
		return nil, err
	}
	statusList, err := decodeCredentialMap(raw)
	if err != nil {
		return nil, err
	}
	subject, ok := statusList["credentialSubject"].(map[string]any)
	if !ok {
		return nil, errors.New("status list subject missing")
	}
	encodedList, _ := subject["encodedList"].(string)
	if encodedList == "" {
		return nil, errors.New("status list encodedList missing")
	}
	return decodeStatusBits(encodedList)
}

func sign(vc map[string]any, signingKey SigningKey, created time.Time) error {
	delete(vc, "proof")
	canonical, err := canonicalCredential(vc)
	if err != nil {
		return err
	}
	vc["proof"] = map[string]any{
		"type":               proofType,
		"created":            created.Format(time.RFC3339),
		"proofPurpose":       "assertionMethod",
		"verificationMethod": signingKey.VerificationMethod,
		"jws":                base64.RawURLEncoding.EncodeToString(ed25519.Sign(signingKey.PrivateKey, canonical)),
	}
	return nil
}

func verify(raw []byte, bundle TrustBundle, now time.Time) (VerificationResult, error) {
	vc, err := decodeCredentialMap(raw)
	if err != nil {
		return VerificationResult{}, err
	}
	issuerValue, _ := vc["issuer"].(string)
	proof, ok := vc["proof"].(map[string]any)
	if !ok {
		return VerificationResult{}, errors.New("credential proof missing")
	}
	method, _ := proof["verificationMethod"].(string)
	issuer, err := bundle.findVerificationMethod(issuerValue, method)
	if err != nil {
		return VerificationResult{}, err
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
			return VerificationResult{}, fmt.Errorf("credential expired at %s", expiresAt.Format(time.RFC3339))
		}
	}
	subject, ok := vc["credentialSubject"].(map[string]any)
	if !ok {
		return VerificationResult{}, errors.New("credential subject missing")
	}
	return VerificationResult{Credential: vc, Subject: subject}, nil
}

func decodeCredentialMap(raw []byte) (map[string]any, error) {
	var vc map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&vc); err != nil {
		return nil, err
	}
	return vc, nil
}

func verifyWithStatus(client *http.Client, audit Auditor, raw []byte, bundle TrustBundle, now time.Time, subject string) (VerificationResult, error) {
	result, err := verify(raw, bundle, now)
	if err != nil {
		return VerificationResult{}, err
	}
	audit.Log("trust-bundle", "verified_with_issuer_key", subject+" "+proofVerificationMethod(result.Credential))
	if err := verifyCredentialStatus(client, audit, result.Credential, bundle, now, subject); err != nil {
		return VerificationResult{}, err
	}
	return result, nil
}

func verifyCredentialStatus(client *http.Client, audit Auditor, vc map[string]any, bundle TrustBundle, now time.Time, subject string) error {
	index, statusListURL, err := credentialStatusIndexAndURL(vc)
	if err != nil {
		return err
	}

	audit.Log("revocation-authority", "fetch_status_list", fmt.Sprintf("%s index=%d %s", subject, index, statusListURL))
	rawStatusList, err := getBytes(client, statusListURL)
	if err != nil {
		return err
	}
	statusList, err := verify(rawStatusList, bundle, now)
	if err != nil {
		return fmt.Errorf("status list verification failed: %w", err)
	}
	encodedList, _ := statusList.Subject["encodedList"].(string)
	bits, err := decodeStatusBits(encodedList)
	if err != nil {
		return fmt.Errorf("status list decode failed: %w", err)
	}
	revoked, err := statusBit(bits, index)
	if err != nil {
		return err
	}
	if revoked {
		return RevokedCredentialError{StatusListURL: statusListURL, Index: index}
	}
	audit.Log("revocation-authority", "verified_status_list", fmt.Sprintf("%s index=%d revoked=false", subject, index))
	return nil
}

func credentialStatusIndexAndURL(vc map[string]any) (int, string, error) {
	entry, ok := vc["credentialStatus"].(map[string]any)
	if !ok {
		return 0, "", errors.New("credential status missing")
	}
	if purpose, _ := entry["statusPurpose"].(string); purpose != "revocation" {
		return 0, "", fmt.Errorf("unsupported credential status purpose %q", purpose)
	}
	statusListURL, _ := entry["statusListCredential"].(string)
	if statusListURL == "" {
		return 0, "", errors.New("credential status list URL missing")
	}
	indexText, _ := entry["statusListIndex"].(string)
	index, err := strconv.Atoi(indexText)
	if err != nil {
		return 0, "", fmt.Errorf("credential status index invalid: %w", err)
	}
	return index, statusListURL, nil
}

func mergeCRDTUpdates(subject map[string]any) (CRDTMergeSummary, error) {
	rawOperations, ok := subject["operations"].([]any)
	if !ok {
		return CRDTMergeSummary{}, errors.New("crdt update subject missing operations")
	}
	lwwWinners := map[string]CRDTOperation{}
	lwwValueCounts := map[string]map[string]bool{}
	orSets := map[string]map[string]bool{}
	replicas := map[string]bool{}
	operationCount := 0

	for _, raw := range rawOperations {
		item, ok := raw.(map[string]any)
		if !ok {
			return CRDTMergeSummary{}, errors.New("crdt operation is not an object")
		}
		op, err := crdtOperationFromMap(item)
		if err != nil {
			return CRDTMergeSummary{}, err
		}
		operationCount++
		replicas[op.Replica] = true
		switch op.Kind {
		case "lww-register":
			if lwwValueCounts[op.Target] == nil {
				lwwValueCounts[op.Target] = map[string]bool{}
			}
			lwwValueCounts[op.Target][op.Value] = true
			winner, exists := lwwWinners[op.Target]
			if !exists || compareLWW(op, winner) > 0 {
				lwwWinners[op.Target] = op
			}
		case "or-set-add":
			if orSets[op.Target] == nil {
				orSets[op.Target] = map[string]bool{}
			}
			orSets[op.Target][op.Value] = true
		default:
			return CRDTMergeSummary{}, fmt.Errorf("unsupported crdt operation kind %q", op.Kind)
		}
	}

	telemetryEndpoints := sortedSet(orSets["telemetryEndpoints"])
	capabilityTags := sortedSet(orSets["capabilityTags"])
	routingProfile := ""
	conflictCount := 0
	conflictTarget := ""
	conflictWinner := ""
	if winner, ok := lwwWinners["routingProfile"]; ok {
		routingProfile = winner.Value
		if len(lwwValueCounts["routingProfile"]) > 1 {
			conflictCount = len(lwwValueCounts["routingProfile"]) - 1
			conflictTarget = "routingProfile"
			conflictWinner = winner.Replica + "/" + winner.Value
		}
	}

	state := map[string]any{
		"routingProfile":     routingProfile,
		"telemetryEndpoints": telemetryEndpoints,
		"capabilityTags":     capabilityTags,
	}
	var canonical bytes.Buffer
	if err := writeCanonicalJSON(&canonical, state); err != nil {
		return CRDTMergeSummary{}, err
	}
	hash := sha256.Sum256(canonical.Bytes())
	return CRDTMergeSummary{
		ReplicaCount:       len(replicas),
		OperationCount:     operationCount,
		RoutingProfile:     routingProfile,
		TelemetryEndpoints: telemetryEndpoints,
		CapabilityTags:     capabilityTags,
		ConflictCount:      conflictCount,
		ConflictTarget:     conflictTarget,
		ConflictWinner:     conflictWinner,
		StateHash:          fmt.Sprintf("%x", hash[:8]),
	}, nil
}

func crdtOperationFromMap(item map[string]any) (CRDTOperation, error) {
	logicalTime, ok := intValue(item["logicalTime"])
	if !ok {
		return CRDTOperation{}, fmt.Errorf("crdt operation %q missing logicalTime", stringValue(item, "id"))
	}
	op := CRDTOperation{
		ID:          stringValue(item, "id"),
		Replica:     stringValue(item, "replica"),
		Kind:        stringValue(item, "kind"),
		Target:      stringValue(item, "target"),
		Value:       stringValue(item, "value"),
		LogicalTime: logicalTime,
		Dot:         stringValue(item, "dot"),
	}
	if op.ID == "" || op.Replica == "" || op.Kind == "" || op.Target == "" || op.Value == "" || op.Dot == "" {
		return CRDTOperation{}, errors.New("crdt operation missing required field")
	}
	return op, nil
}

func compareLWW(left, right CRDTOperation) int {
	if left.LogicalTime > right.LogicalTime {
		return 1
	}
	if left.LogicalTime < right.LogicalTime {
		return -1
	}
	if left.Replica > right.Replica {
		return 1
	}
	if left.Replica < right.Replica {
		return -1
	}
	return 0
}

func sortedSet(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (bundle TrustBundle) findVerificationMethod(id, method string) (Issuer, error) {
	for _, issuer := range bundle.Issuers {
		if issuer.ID == id && issuer.VerificationMethod == method {
			return issuer, nil
		}
	}
	return Issuer{}, fmt.Errorf("verification method %q for issuer %q not trusted", method, id)
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

func encodeStatusBits(bits []byte) (string, error) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(bits); err != nil {
		_ = writer.Close()
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(compressed.Bytes()), nil
}

func decodeStatusBits(encoded string) ([]byte, error) {
	compressed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	bits, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(bits) != statusListSizeBits/8 {
		return nil, fmt.Errorf("unexpected status list size %d bytes", len(bits))
	}
	return bits, nil
}

func setStatusBit(bits []byte, index int, revoked bool) error {
	if index < 0 || index >= statusListSizeBits {
		return fmt.Errorf("status list index %d out of range", index)
	}
	byteIndex := index / 8
	mask := byte(1 << (index % 8))
	if revoked {
		bits[byteIndex] |= mask
	} else {
		bits[byteIndex] &^= mask
	}
	return nil
}

func statusBit(bits []byte, index int) (bool, error) {
	if index < 0 || index >= statusListSizeBits {
		return false, fmt.Errorf("status list index %d out of range", index)
	}
	byteIndex := index / 8
	mask := byte(1 << (index % 8))
	return bits[byteIndex]&mask != 0, nil
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

func trustBundleSummary(bundle TrustBundle) string {
	parts := make([]string, 0, len(bundle.Issuers))
	for _, issuer := range bundle.Issuers {
		state := issuer.State
		if state == "" {
			state = "trusted"
		}
		parts = append(parts, issuer.VerificationMethod+"("+state+")")
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmpFile, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func logVerificationFailure(audit Auditor, state map[string]bool, key, peer, kind, subject string, err error) {
	state[key] = true
	audit.Log(peer, "verification_failed_"+kind, subject+" "+err.Error())
}

func logVerificationSuccess(audit Auditor, state map[string]bool, key, peer, kind, subject string) {
	if state[key] {
		audit.Log(peer, "verification_recovered_"+kind, subject)
	}
	state[key] = false
}

func logRevocationFailureIfNeeded(audit Auditor, state map[string]bool, key, subject string, err error) {
	var revoked RevokedCredentialError
	if errors.As(err, &revoked) {
		logVerificationFailure(audit, state, "revocation:"+key, "revocation-authority", "revoked_status_list", subject, err)
	}
}

func credentialRawLogSummary(raw []byte) string {
	var vc map[string]any
	if err := json.Unmarshal(raw, &vc); err != nil {
		return "unparseable_credential:" + err.Error()
	}
	return credentialLogSummary(vc)
}

func verificationLogSummary(vc map[string]any) string {
	return credentialLogSummary(vc)
}

func credentialLogSummary(vc map[string]any) string {
	subject, _ := vc["credentialSubject"].(map[string]any)
	return fmt.Sprintf("type=%s generation=%v key=%s expires=%v", credentialTypeName(vc), subject["credentialVersion"], proofVerificationMethod(vc), vc["expirationDate"])
}

func proofVerificationMethod(vc map[string]any) string {
	proof, _ := vc["proof"].(map[string]any)
	method, _ := proof["verificationMethod"].(string)
	if method == "" {
		return "unknown"
	}
	return method
}

func credentialTypeName(vc map[string]any) string {
	rawTypes, ok := vc["type"].([]any)
	if !ok || len(rawTypes) == 0 {
		return "unknown"
	}
	last, _ := rawTypes[len(rawTypes)-1].(string)
	if last == "" {
		return "unknown"
	}
	return last
}

func isHighlightedEvent(event Event) bool {
	action := strings.ToLower(event.Action)
	return strings.Contains(action, "verification_failed") ||
		strings.Contains(action, "credential_rotated") ||
		strings.Contains(action, "credential_rotation_failed") ||
		strings.Contains(action, "push_revocation") ||
		strings.Contains(action, "publish_crdt_update") ||
		strings.Contains(action, "crdt_conflict_resolved") ||
		strings.Contains(action, "issuer_key_") ||
		strings.Contains(action, "old_issuer_key_retired")
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

func intValue(value any) (int, bool) {
	switch v := value.(type) {
	case json.Number:
		parsed, err := v.Int64()
		return int(parsed), err == nil
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
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
