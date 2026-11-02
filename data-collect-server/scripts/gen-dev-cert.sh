#!/usr/bin/env bash
# Generate a self-signed CA and a server certificate for local IoTDA testing.
#
# Usage:
#   ./scripts/gen-dev-cert.sh [IP_OR_DOMAIN]
#
# Outputs:
#   certs/ca.crt       upload this to IoTDA: Devices > Device Certificates > CA Certificates
#   certs/server.crt   TLS_CERT_FILE
#   certs/server.key   TLS_KEY_FILE
set -euo pipefail

HOST="${1:-116.62.55.142}"
OUT_DIR="${OUT_DIR:-certs}"

if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required" >&2
    exit 1
fi

mkdir -p "$OUT_DIR"
cd "$OUT_DIR"

openssl genrsa -out ca.key 4096
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 \
    -subj "/C=CN/O=DataCollect/CN=DataCollect Dev CA" \
    -out ca.crt

openssl genrsa -out server.key 2048
openssl req -new -key server.key \
    -subj "/C=CN/O=DataCollect/CN=${HOST}" \
    -out server.csr

if [[ "$HOST" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    HOST_ALT="IP.2 = ${HOST}"
else
    HOST_ALT="DNS.2 = ${HOST}"
fi

cat > server.ext <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
${HOST_ALT}
EOF

openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out server.crt -days 825 -sha256 -extfile server.ext

rm -f server.csr server.ext ca.srl
chmod 600 ca.key server.key

echo
echo "Generated:"
echo "  ${OUT_DIR}/ca.crt      -> upload to IoTDA CA certificate page"
echo "  ${OUT_DIR}/server.crt  -> TLS_CERT_FILE"
echo "  ${OUT_DIR}/server.key  -> TLS_KEY_FILE"
echo
echo "Push URL example: https://${HOST}:11294/data/push"
