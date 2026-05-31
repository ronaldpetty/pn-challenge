#!/usr/bin/env sh
set -eu

ARTIFACTS="${ARTIFACTS_DIR:-/artifacts}"
READY_FILE="$ARTIFACTS/.ready"
BINARY="${BINARY_PATH:-/shared-bin/pn-demo}"

if [ -f "$READY_FILE" ]; then
  echo "artifacts already initialized at $ARTIFACTS"
  exit 0
fi

command -v openssl >/dev/null 2>&1 || {
  echo "openssl is required in the artifact-init container" >&2
  exit 1
}

if [ ! -x "$BINARY" ]; then
  echo "$BINARY is required before artifact generation" >&2
  exit 1
fi

mkdir -p "$ARTIFACTS/tls/ca" "$ARTIFACTS/keys" "$ARTIFACTS/tmp"

echo "generating local CA"
openssl req \
  -x509 \
  -newkey rsa:2048 \
  -nodes \
  -days 365 \
  -subj "/CN=NANDA Demo Local CA" \
  -keyout "$ARTIFACTS/tls/ca/ca.key" \
  -out "$ARTIFACTS/tls/ca/ca.crt"

make_cert() {
  service="$1"
  san="$2"
  service_dir="$ARTIFACTS/tls/$service"
  mkdir -p "$service_dir"
  conf="$ARTIFACTS/tmp/$service.openssl.cnf"

  cat > "$conf" <<EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
CN = $service

[v3_req]
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = $san
EOF

  echo "generating TLS certificate for $service"
  openssl req \
    -newkey rsa:2048 \
    -nodes \
    -keyout "$service_dir/tls.key" \
    -out "$service_dir/tls.csr" \
    -config "$conf"

  openssl x509 \
    -req \
    -in "$service_dir/tls.csr" \
    -CA "$ARTIFACTS/tls/ca/ca.crt" \
    -CAkey "$ARTIFACTS/tls/ca/ca.key" \
    -CAcreateserial \
    -out "$service_dir/tls.crt" \
    -days 365 \
    -sha256 \
    -extfile "$conf" \
    -extensions v3_req
}

make_cert "index" "DNS:index,DNS:index.nanda.local,DNS:localhost,IP:127.0.0.1"
make_cert "agent-alpha" "DNS:agent-alpha,DNS:alpha.nanda.local,DNS:localhost,IP:127.0.0.1"
make_cert "agent-beta" "DNS:agent-beta,DNS:beta.nanda.local,DNS:localhost,IP:127.0.0.1"

echo "generating Ed25519 issuer key"
openssl genpkey -algorithm ED25519 -out "$ARTIFACTS/keys/nanda-issuer.pem"
openssl pkey -in "$ARTIFACTS/keys/nanda-issuer.pem" -pubout -out "$ARTIFACTS/keys/nanda-issuer.pub.pem"

echo "generating signed AgentAddr and AgentFacts credentials"
"$BINARY" artifacts --artifacts "$ARTIFACTS"

rm -rf "$ARTIFACTS/tmp"
touch "$READY_FILE"
echo "artifacts initialized at $ARTIFACTS"
