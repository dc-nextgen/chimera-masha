#!/usr/bin/env bash
# build-binaries.sh — Masha PAYLAŞIMLI binary'lerini BİR KEZ derler (tenant verisi YOK: token/cert/frpc/config
# yazılmaz). assemble-tenant.sh bu çıktıyı (dist-bin/) tenant'a özel config ile paketler — derlemesiz montaj,
# Pota/VPS'te Go kurulu olması GEREKMEZ. (docs/masha-plan.md §16 devamı, per-tenant derleme israfının çözümü.)
#
# Girdi (env):
#   MASHA_OS_TARGETS  boşlukla ayrılmış "GOOS/GOARCH" listesi (default: "darwin/arm64 windows/amd64 linux/amd64")
#   MASHA_BIN_DIR     çıktı klasörü (default: ./dist-bin)
#   MASHA_VERSION     sürüm ELLE override (default: `git describe --tags --dirty`; tag YOKSA damgasız build —
#                     binary main.go'daki `var version` default'unu bildirir; ikisi de TUTARLI, aşağıya bkz.)
#
# Çıktı: $MASHA_BIN_DIR/masha-agent-<os>-<arch>[.exe] (her hedef için; sürüm VARSA -ldflags -X main.version=
# ile DAMGALI) + $MASHA_BIN_DIR/VERSION (binary'nin GERÇEKTE bildireceği sürümle TUTARLI — Pota bunu OpenAPI
# info.version + /healthz üzerinden TÜNELDEN okur, VERSION dosyası yalnız yerel iz).
# İdempotent: her koşuda güncel derlenir (go build zaten değişmeyeni hızlı üretir); eski hedef dosyaları SİLİNMEZ
# (MASHA_OS_TARGETS daraltılırsa dist-bin'de eski hedefler kalır — elle temizlik operatöre bırakılır).
set -euo pipefail
cd "$(dirname "$0")"

TARGETS="${MASHA_OS_TARGETS:-darwin/arm64 windows/amd64 linux/amd64}"
OUT_DIR="${MASHA_BIN_DIR:-dist-bin}"
mkdir -p "$OUT_DIR"

echo "▸ Yerel yüz (web) derleniyor (bir kez — tüm hedefler aynı embed'i paylaşır)…"
( cd web && npm install >/tmp/masha-build-binaries-web.log 2>&1 && npm run build >>/tmp/masha-build-binaries-web.log 2>&1 ) \
  || { echo "  ✗ web build (bkz /tmp/masha-build-binaries-web.log)"; exit 1; }
git checkout HEAD -- web/dist/.gitkeep 2>/dev/null || true
[ -f web/dist/.gitkeep ] || touch web/dist/.gitkeep

# Sürüm damgası: MASHA_VERSION (elle override) > `git describe --tags --dirty` (tag'li commit → ör.
# v0.2.0-3-gabc1234-dirty) > main.go default (damgasız — tag yoksa build-time -ldflags ATLANIR).
V="${MASHA_VERSION:-$(git describe --tags --dirty 2>/dev/null || true)}"
if [ -n "$V" ]; then
  VERSION="$V"
  go_build(){ CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build -ldflags "-X main.version=$V" -o "$3" .; }
else
  # Damgasız: main.go'daki `var version = "..."` default'u — binary GERÇEKTE bunu bildirecek, VERSION dosyası
  # ile TUTARLI olmalı → aynı satırdan çek (main.go'ya dokunmadan tek-kaynak korunur; grep boşsa hardcode fallback).
  VERSION="$(grep -m1 '^var version = ' main.go 2>/dev/null | sed -E 's/^var version = "([^"]*)".*/\1/')"
  VERSION="${VERSION:-0.1.0-beta.1}"
  go_build(){ CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build -o "$3" .; }
fi
echo "$VERSION" > "$OUT_DIR/VERSION"

for T in $TARGETS; do
  GOOS="${T%/*}"; GOARCH="${T#*/}"
  EXT=""; [ "$GOOS" = windows ] && EXT=".exe"
  OUT="$OUT_DIR/masha-agent-${GOOS}-${GOARCH}${EXT}"
  echo "▸ $GOOS/$GOARCH → $OUT"
  go_build "$GOOS" "$GOARCH" "$OUT" \
    || { echo "  ✗ go build ($GOOS/$GOARCH) başarısız"; exit 1; }
done

echo
if [ -n "$V" ]; then
  echo "✓ Paylaşımlı binary'ler hazır: $OUT_DIR/ (VERSION=$VERSION, ldflags-damgalı)"
else
  echo "✓ Paylaşımlı binary'ler hazır: $OUT_DIR/ (VERSION=$VERSION, main.go default — git tag yok, damgasız)"
fi
echo "  → tenant montajı: MASHA_APP_TOKEN=... EDGE_ADDR=... bash assemble-tenant.sh <tenant> (derlemesiz, Go gerekmez)"
