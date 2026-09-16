#!/usr/bin/env bash
# Sobe o servidor VAIJUNTO em Docker nesta máquina, roda o smoke TCP e
# verifica persistência após docker restart. Não cobre LAN de 3 PCs.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if docker info >/dev/null 2>&1; then
  DK=(docker)
elif sudo docker info >/dev/null 2>&1; then
  DK=(sudo docker)
else
  echo "docker não está acessível" >&2
  exit 1
fi

PORT="${SERVER_PORT:-5000}"
NAME="${CONTAINER_NAME:-vaijunto-server}"

"${DK[@]}" build -t vaijunto-server --build-arg BUILD_TARGET=server .
"${DK[@]}" rm -f "$NAME" >/dev/null 2>&1 || true
"${DK[@]}" run -d --name "$NAME" \
  -p "${PORT}:5000" \
  -v vaijunto-data:/data \
  -e LISTEN_HOST=0.0.0.0 \
  -e SERVER_PORT=5000 \
  -e DATA_PATH=/data/state.json \
  vaijunto-server

echo "aguardando listen..."
for i in $(seq 1 30); do
  if (echo >/dev/tcp/127.0.0.1/"$PORT") >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

export SERVER_HOST=127.0.0.1 SERVER_PORT="$PORT"
go build -o /tmp/vaijunto-smoke ./cmd/smoke
/tmp/vaijunto-smoke

echo "reiniciando container (volume deve sobreviver)..."
"${DK[@]}" restart "$NAME"
sleep 1
/tmp/vaijunto-smoke

"${DK[@]}" exec "$NAME" cat /data/state.json | python3 -c "
import json,sys
st=json.load(sys.stdin)
assert len(st['users'])>=5
assert len(st['rides'])>=1
print('DOCKER LOCAL OK users=%d rides=%d reservations=%d' % (
    len(st['users']), len(st['rides']), len(st['reservations'])))
"
echo "container $NAME continua no ar em 127.0.0.1:${PORT}"
