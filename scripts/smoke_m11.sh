#!/usr/bin/env bash
set -euo pipefail

# Simple smoke for M11 async path.
# It expects your repo/tooling to provide a way to compile .desi to C and then to a binary.
# You can point the builder via DESI_BUILD_CMD, e.g.:
#   export DESI_BUILD_CMD='desi build --out gen/out/m11_async_sleep'
#
# The script only verifies the runtime wiring + generated program output.

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXDIR="$ROOT/examples/m11_async_sleep"
SRC="$EXDIR/main.desi"
OUT="${OUT:-$ROOT/gen/out/m11_async_sleep}"

echo "[smoke] source: $SRC"
echo "[smoke] out:    $OUT"

# Prefer an explicit builder if provided.
if [[ -n "${DESI_BUILD_CMD:-}" ]]; then
  echo "[smoke] using DESI_BUILD_CMD:"
  echo "        $DESI_BUILD_CMD \"$SRC\" --out \"$OUT\""
  eval "$DESI_BUILD_CMD \"$SRC\" --out \"$OUT\""
else
  cat <<'EONOTE'
[smoke] No DESI_BUILD_CMD set. This script expects your repo to have a CLI or
        a command that lowers .desi → C → executable.

        Example (pseudo):
          DESI_BUILD_CMD='desic -emit-c gen/out/m11_async_sleep.c -- && go run ./cmd/cc -in gen/out/m11_async_sleep.c -o gen/out/m11_async_sleep'

        You can export that and re-run:
          export DESI_BUILD_CMD='<your build command>'
          scripts/smoke_m11.sh
EONOTE
  exit 0
fi

echo "[smoke] running: $OUT"
"$OUT" | tee "$OUT.log"

echo "[smoke] verifying output..."
grep -q '^before$' "$OUT.log"
grep -q '^result 42$' "$OUT.log"

echo "[smoke] OK: output matches expected"
