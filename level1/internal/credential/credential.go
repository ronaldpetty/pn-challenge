package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"
)

const (
	ProofType          = "Ed25519Signature2020"
	DefaultIssuerID    = "did:web:nanda.local:issuer"
	DefaultIssuerKeyID = DefaultIssuerID + "#key-1"
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

type VerificationResult struct {
	Credential map[string]any
	Subject    map[string]any
	Issuer     Issuer
}

func LoadTrustBundle(path string) (TrustBundle, error) {
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

func LoadEd25519PrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", path)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	privateKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s is %T, not an Ed25519 private key", path, key)
	}
	return privateKey, nil
}

func LoadEd25519PublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", path)
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%s is %T, not an Ed25519 public key", path, key)
	}
	return publicKey, nil
}

func PublicKeyBase64URL(key ed25519.PublicKey) string {
	return base64.RawURLEncoding.EncodeToString(key)
}

func Sign(vc map[string]any, privateKey ed25519.PrivateKey, verificationMethod string, created time.Time) error {
	delete(vc, "proof")

	canonical, err := canonicalCredential(vc)
	if err != nil {
		return err
	}
	signature := ed25519.Sign(privateKey, canonical)
	vc["proof"] = map[string]any{
		"type":               ProofType,
		"created":            created.UTC().Format(time.RFC3339),
		"proofPurpose":       "assertionMethod",
		"verificationMethod": verificationMethod,
		"jws":                base64.RawURLEncoding.EncodeToString(signature),
	}
	return nil
}

func Verify(raw []byte, bundle TrustBundle, now time.Time) (VerificationResult, error) {
	var vc map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&vc); err != nil {
		return VerificationResult{}, err
	}

	issuerID, _ := vc["issuer"].(string)
	if issuerID == "" {
		return VerificationResult{}, errors.New("credential issuer is missing")
	}
	issuer, err := bundle.findIssuer(issuerID)
	if err != nil {
		return VerificationResult{}, err
	}

	proof, ok := vc["proof"].(map[string]any)
	if !ok {
		return VerificationResult{}, errors.New("credential proof is missing")
	}
	if proofType, _ := proof["type"].(string); proofType != ProofType {
		return VerificationResult{}, fmt.Errorf("unsupported proof type %q", proofType)
	}
	if method, _ := proof["verificationMethod"].(string); method != issuer.VerificationMethod {
		return VerificationResult{}, fmt.Errorf("unexpected verification method %q", method)
	}
	signatureValue, _ := proof["jws"].(string)
	if signatureValue == "" {
		return VerificationResult{}, errors.New("proof jws is missing")
	}
	signature, err := base64.RawURLEncoding.DecodeString(signatureValue)
	if err != nil {
		return VerificationResult{}, fmt.Errorf("decode proof jws: %w", err)
	}
	publicKeyBytes, err := base64.RawURLEncoding.DecodeString(issuer.PublicKeyBase64URL)
	if err != nil {
		return VerificationResult{}, fmt.Errorf("decode issuer public key: %w", err)
	}
	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return VerificationResult{}, fmt.Errorf("issuer public key has %d bytes", len(publicKeyBytes))
	}

	canonical, err := canonicalCredential(vc)
	if err != nil {
		return VerificationResult{}, err
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), canonical, signature) {
		return VerificationResult{}, errors.New("credential signature verification failed")
	}

	if err := verifyTimeWindow(vc, now); err != nil {
		return VerificationResult{}, err
	}
	subject, ok := vc["credentialSubject"].(map[string]any)
	if !ok {
		return VerificationResult{}, errors.New("credential subject is missing")
	}
	return VerificationResult{Credential: vc, Subject: subject, Issuer: issuer}, nil
}

func (bundle TrustBundle) findIssuer(id string) (Issuer, error) {
	for _, issuer := range bundle.Issuers {
		if issuer.ID == id {
			return issuer, nil
		}
	}
	return Issuer{}, fmt.Errorf("issuer %q is not trusted", id)
}

func verifyTimeWindow(vc map[string]any, now time.Time) error {
	if issuance, _ := vc["issuanceDate"].(string); issuance != "" {
		issuedAt, err := time.Parse(time.RFC3339, issuance)
		if err != nil {
			return fmt.Errorf("parse issuanceDate: %w", err)
		}
		if now.Before(issuedAt.Add(-1 * time.Minute)) {
			return errors.New("credential is not valid yet")
		}
	}
	if expiration, _ := vc["expirationDate"].(string); expiration != "" {
		expiresAt, err := time.Parse(time.RFC3339, expiration)
		if err != nil {
			return fmt.Errorf("parse expirationDate: %w", err)
		}
		if !now.Before(expiresAt) {
			return errors.New("credential has expired")
		}
	}
	return nil
}

func canonicalCredential(vc map[string]any) ([]byte, error) {
	copy := make(map[string]any, len(vc))
	for key, value := range vc {
		if key == "proof" {
			continue
		}
		copy[key] = value
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
		if v.String() == "" {
			return errors.New("empty json number")
		}
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
