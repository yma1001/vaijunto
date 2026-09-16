#!/usr/bin/env bash
# Smoke local: sobe o servidor, disputa a última vaga, derruba, sobe de novo e
# verifica persistência (LOGIN de novo + carona ainda visível).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
TMP="$ROOT/data/smoke-state.json"
rm -f "$TMP" "${TMP}.tmp"
mkdir -p "$ROOT/data" "$ROOT/bin"
go build -o bin/server ./cmd/server
go build -o bin/smoke ./cmd/smoke
go build -o bin/passenger ./cmd/passenger

export DATA_PATH="$TMP"
export SERVER_HOST=127.0.0.1
export SERVER_PORT=5000
export LISTEN_HOST=127.0.0.1

./bin/server &
SID=$!
cleanup() { kill "$SID" 2>/dev/null || true; }
trap cleanup EXIT
sleep 0.4
./bin/smoke

kill "$SID"
wait "$SID" 2>/dev/null || true
trap - EXIT

# Reinício: a sessão TCP morreu; contas e caronas devem continuar.
./bin/server &
SID=$!
trap 'kill "$SID" 2>/dev/null || true' EXIT
sleep 0.4
./bin/smoke
python3 - <<'PY'
import json,os,sys
p=os.environ.get("DATA_PATH","data/smoke-state.json")
st=json.load(open(p))
users={u["username"] for u in st["users"]}
assert "motorista1" in users and "passageiro1" in users, users
assert len(st["rides"])>=1, st["rides"]
print("PERSISTENCE RESTART OK: users=%d rides=%d reservations=%d" % (
    len(st["users"]), len(st["rides"]), len(st["reservations"])))
PY
kill "$SID"
wait "$SID" 2>/dev/null || true
trap - EXIT
echo "SMOKE LOCAL OK"
