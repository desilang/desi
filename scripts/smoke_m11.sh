#!/usr/bin/env bash
set -euo pipefail

# Simple smoke for M11 async path.
# It now auto-detects your Stage-1 builder:
#   go run ./compiler/cmd/desic build <src>
# You can still override with:
#   export DESI_BUILD_CMD='<your build command>'
#   scripts/smoke_m11.sh
#
# The script verifies output contains:
#   before
#   result 42

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXDIR="$ROOT/examples/m11_async_sleep"
SRC="${SRC:-$EXDIR/main.desi}"

# By default, desic names the output using the source basename (without .desi)
# e.g., main.desi -> gen/out/main
BASENAME="$(basename "$SRC")"
STEM="${BASENAME%.*}"

# Preferred OUT can be overridden, but we’ll also probe sensible fallbacks.
OUT="${OUT:-$ROOT/gen/out/$STEM}"

echo "[smoke] source: $SRC"

build_with_cli() {
  echo "[smoke] building via: go run ./compiler/cmd/desic build \"$SRC\""
  ( cd "$ROOT" && go run ./compiler/cmd/desic build "$SRC" )
}

if [[ -n "${DESI_BUILD_CMD:-}" ]]; then
  echo "[smoke] using DESI_BUILD_CMD:"
  echo "        $DESI_BUILD_CMD \"$SRC\" --out \"$OUT\""
  # shellcheck disable=SC2086
  eval "$DESI_BUILD_CMD \"$SRC\" --out \"$OUT\""
else
  # Auto-detect the standard Stage-1 builder.
  build_with_cli
fi

# Probe common output locations in priority order.
candidates=()
# 1) gen/out/<basename-without-ext>
candidates+=("$ROOT/gen/out/$STEM")
# 2) gen/out/<parent-dir-name> (some setups choose the folder name)
candidates+=("$ROOT/gen/out/$(basename "$(dirname "$SRC")")")
# 3) user-provided OUT if set
if [[ -n "${OUT:-}" ]]; then
  candidates+=("$OUT")
fi

exe=""
for c in "${candidates[@]}"; do
  if [[ -f "$c" && -x "$c" ]]; then
    exe="$c"
    break
  fi
done

if [[ -z "$exe" ]]; then
  echo "[smoke] could not find built executable. Checked:"
  for c in "${candidates[@]}"; do
    echo "  - $c"
  done
  echo "[smoke] If your builder emits a different path, set OUT or DESI_BUILD_CMD, e.g.:"
  cat <<'EONOTE'
  export OUT=gen/out/m11_async_sleep
  # or
  export DESI_BUILD_CMD='go run ./compiler/cmd/desic build'
EONOTE
  exit 1
fi

echo "[smoke] running: $exe"
"$exe" | tee "$exe.log"

echo "[smoke] verifying output..."
grep -q '^before$' "$exe.log"
grep -q '^result 42$' "$exe.log"

echo "[smoke] OK: output matches expected"
