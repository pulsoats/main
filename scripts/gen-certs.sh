#!/usr/bin/env bash
# pulsoats cert generator (ECDSA P-256, all PEM).
#
# Modes:
#   ./gen-certs.sh [OUTPUT_DIR]            # base set (run once)
#   ./gen-certs.sh node <ip|host> [DIR]   # daemon server cert for a new node
#
# Base set produces:
#   gRPC mTLS:   ca.pem ca-key.pem   cert.pem key.pem   analysis-cert.pem analysis-key.pem
#     - ca-key.pem is SEC1 ("EC PRIVATE KEY"): certgen signs live-worker certs with it
#     - cert.pem/key.pem  = main gRPC client (to analysis + workers)
#     - analysis-*.pem    = analysis gRPC server (SAN: analysis)
#   Docker mTLS: docker-ca.pem docker-ca-key.pem   docker-cert.pem docker-key.pem
#     - main is the TLS *client* to each node's dockerd
#     - docker-ca-key.pem is kept so you can mint a server cert per node (see `node` mode)
#
# Per node (run when adding a machine), then install node-<id>-* on that node's dockerd
# (--tlscert/--tlskey) and trust docker-ca.pem via --tlscacert + --tlsverify:
#   ./gen-certs.sh node 203.0.113.10
set -euo pipefail

CURVE=prime256v1
DAYS_CA=3650
DAYS_LEAF=825

# <cert> <key> <CN> <eku> <san> <ca-cert> <ca-key>
sign_leaf() {
  local cert="$1" key="$2" cn="$3" eku="$4" san="$5" cacert="$6" cakey="$7"
  openssl ecparam -name "$CURVE" -genkey -noout -out "$key"
  openssl req -new -key "$key" -out _csr.pem -subj "/CN=$cn"
  {
    echo "extendedKeyUsage=$eku"
    [ -n "$san" ] && echo "subjectAltName=$san"
  } > _ext.cnf
  openssl x509 -req -in _csr.pem -CA "$cacert" -CAkey "$cakey" -CAcreateserial \
    -days "$DAYS_LEAF" -sha256 -extfile _ext.cnf -out "$cert"
  rm -f _csr.pem _ext.cnf
}

# ---- node mode: server cert for a node's docker daemon ----
if [ "${1:-}" = "node" ]; then
  ADDR="${2:?usage: gen-certs.sh node <ip|host> [DIR]}"
  OUT="${3:-./certs}"
  cd "$OUT"
  [ -f docker-ca.pem ] && [ -f docker-ca-key.pem ] || {
    echo "error: docker-ca.pem / docker-ca-key.pem not found in $OUT (run base set first)" >&2
    exit 1
  }
  if [[ "$ADDR" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    SAN="IP:$ADDR"
  else
    SAN="DNS:$ADDR"
  fi
  id="${ADDR//[^A-Za-z0-9_.-]/_}"
  sign_leaf "node-$id-cert.pem" "node-$id-key.pem" "$ADDR" serverAuth "$SAN" \
    docker-ca.pem docker-ca-key.pem
  rm -f ./*.srl
  chmod 600 "node-$id-key.pem" 2>/dev/null || true
  echo ">> node cert -> $OUT/node-$id-cert.pem (install on the node's dockerd; CA = docker-ca.pem)"
  exit 0
fi

# ---- base set ----
OUT="${1:-./certs}"
mkdir -p "$OUT"
cd "$OUT"

echo ">> gRPC CA"
openssl ecparam -name "$CURVE" -genkey -noout -out ca-key.pem
openssl req -x509 -new -key ca-key.pem -sha256 -days "$DAYS_CA" \
  -out ca.pem -subj "/CN=pulsoats-ca"

echo ">> main gRPC client"
sign_leaf cert.pem key.pem main clientAuth "" ca.pem ca-key.pem

echo ">> analysis gRPC server"
sign_leaf analysis-cert.pem analysis-key.pem analysis serverAuth \
  "DNS:analysis,DNS:localhost" ca.pem ca-key.pem

echo ">> Docker CA + main docker client"
openssl ecparam -name "$CURVE" -genkey -noout -out docker-ca-key.pem
openssl req -x509 -new -key docker-ca-key.pem -sha256 -days "$DAYS_CA" \
  -out docker-ca.pem -subj "/CN=pulsoats-docker-ca"
sign_leaf docker-cert.pem docker-key.pem pulsoats-docker-client clientAuth "" \
  docker-ca.pem docker-ca-key.pem

rm -f ./*.srl
# 644: certs/keys are read by the non-root container users (main uid 65532,
# analysis uid 10001) over the shared /run/certs mount, so they must be
# world-readable. The dir itself should be root-owned on a single-tenant host.
chmod 644 ./*.pem 2>/dev/null || true
echo ">> done -> $(pwd)"
ls -1
