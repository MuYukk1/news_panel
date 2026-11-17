#!/usr/bin/env bash
# Monthly network traffic guard for 200 GB/month micro VPS nodes (via vnstat).
# Defaults to a 170 GiB (~182.5 GB decimal) soft threshold on total (rx+tx)
# traffic so ~17.5-30 GB headroom remains regardless of whether the provider
# bills egress-only (tx) or bidirectional (rx+tx).

set -euo pipefail

IFACE="${IFACE:-ens4}"
LIMIT_GIB="${LIMIT_GIB:-170}"
WARN_GIB="${WARN_GIB:-140}"
METRIC="${METRIC:-total}" # "total" (rx+tx) or "tx" (egress only)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="${ENV_FILE:-$REPO_DIR/.env}"
COMPOSE_FILE="${COMPOSE_FILE:-$SCRIPT_DIR/docker-compose.yml}"

ENFORCE=0
for arg in "$@"; do
  case "$arg" in
    --enforce)
      ENFORCE=1
      ;;
    *)
      echo "Usage: $0 [--enforce]" >&2
      exit 2
      ;;
  esac
done

if ! command -v vnstat >/dev/null 2>&1; then
  echo "traffic-guard: vnstat is not installed" >&2
  exit 1
fi

JSON_OUT="$(vnstat -i "$IFACE" --json m 1)"

read -r RX_GIB TX_GIB USED_GIB USED_GB_DEC STATUS <<EOF
$(python3 -c '
import json, sys

data = json.loads(sys.stdin.read())
metric = sys.argv[1]
warn_gib = float(sys.argv[2])
limit_gib = float(sys.argv[3])

ifaces = data.get("interfaces") or []
if not ifaces:
    print("0.00 0.00 0.00 0.00 NO_DATA")
    sys.exit(0)

months = (ifaces[0].get("traffic") or {}).get("month") or []
if not months:
    print("0.00 0.00 0.00 0.00 NO_DATA")
    sys.exit(0)

latest = max(
    months,
    key=lambda m: ((m.get("date") or {}).get("year", 0), (m.get("date") or {}).get("month", 0)),
)
rx = int(latest.get("rx") or 0)
tx = int(latest.get("tx") or 0)
used = tx if metric == "tx" else (rx + tx)

rx_gib = rx / (1024 ** 3)
tx_gib = tx / (1024 ** 3)
gib = used / (1024 ** 3)
gb_dec = used / (1000 ** 3)

if gib >= limit_gib:
    status = "LIMIT_EXCEEDED"
elif gib >= warn_gib:
    status = "WARN"
else:
    status = "OK"

print(f"{rx_gib:.2f} {tx_gib:.2f} {gib:.2f} {gb_dec:.2f} {status}")
' "$METRIC" "$WARN_GIB" "$LIMIT_GIB" <<<"$JSON_OUT")
EOF

TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "$TS iface=$IFACE metric=$METRIC used=${USED_GIB}GiB (${USED_GB_DEC}GB) rx=${RX_GIB}GiB tx=${TX_GIB}GiB warn=${WARN_GIB}GiB limit=${LIMIT_GIB}GiB status=$STATUS"

if [[ "$STATUS" == "LIMIT_EXCEEDED" && "$ENFORCE" -eq 1 ]]; then
  echo "$TS LIMIT_EXCEEDED: stopping x-twifork crawler to preserve monthly traffic headroom"
  if [[ -f "$ENV_FILE" && -f "$COMPOSE_FILE" ]]; then
    DOCKER_BIN=(docker)
    if ! docker info >/dev/null 2>&1 && command -v sudo >/dev/null 2>&1; then
      DOCKER_BIN=(sudo -n docker)
    fi
    "${DOCKER_BIN[@]}" compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" stop x-twifork || true
  fi
fi
