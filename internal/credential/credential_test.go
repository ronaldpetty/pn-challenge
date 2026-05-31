package credential

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"
)

func TestVerifyRejectsTamperedCredential(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	vc := map[string]any{
		"@context":       []any{"https://www.w3.org/2018/credentials/v1"},
		"id":             "urn:test:credential",
		"type":           []any{"VerifiableCredential", "AgentFactsCredential"},
		"issuer":         DefaultIssuerID,
		"issuanceDate":   now.Format(time.RFC3339),
		"expirationDate": now.Add(time.Hour).Format(time.RFC3339),
		"credentialSubject": map[string]any{
			"id":   "agent:test",
			"name": "test.nanda.local",
		},
	}
	if err := Sign(vc, privateKey, DefaultIssuerKeyID, now); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(vc)
	if err != nil {
		t.Fatal(err)
	}
	bundle := TrustBundle{Issuers: []Issuer{{
		ID:                 DefaultIssuerID,
		VerificationMethod: DefaultIssuerKeyID,
		Type:               "Ed25519VerificationKey2020",
		PublicKeyBase64URL: PublicKeyBase64URL(publicKey),
	}}}
	if _, err := Verify(raw, bundle, now); err != nil {
		t.Fatalf("valid credential did not verify: %v", err)
	}

	var tampered map[string]any
	if err := json.Unmarshal(raw, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered["credentialSubject"].(map[string]any)["name"] = "evil.nanda.local"
	tamperedRaw, err := json.Marshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(tamperedRaw, bundle, now); err == nil {
		t.Fatal("tampered credential verified successfully")
	}
}
