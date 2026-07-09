#!/usr/bin/env bash
# assemble-tenant.sh <tenant> — GO BUILD YOK. build-binaries.sh'in ürettiği PAYLAŞIMLI binary'yi dist-bin'den
# KOPYALAR + tenant'a özel config (frpc.toml + .masha.env + certs) yazar → tek-komut installer + zip. Derlemesiz
# olduğu için Pota/VPS'te (Go kurulu olmayan yerlerde) de çalışır — provision-tenant.sh bunu oto-stage için çağırır.
# (docs/masha-plan.md §16 devamı, 2026-07-09: per-tenant derleme → ortak-binary + tenant-config montajı.)
#
# Girdi (env) — make-trial-bundle.sh ile AYNI isimler (geriye-uyum):
#   MASHA_APP_TOKEN (ZORUNLU, VPS: grep MASHA_APP_TOKEN tenants/<tenant>/.env), MASHA_SERVER_LABEL,
#   REMOTE_PORT (default 9800), FRPS_PORT (default 7000), EDGE_ADDR (ZORUNLU), CHIMERA_DOMAIN,
#   MASHA_FRP_TOKEN, MASHA_CONTACT_EMAIL, WEB_PASSWORD, MASHA_TRIAL_LIMIT_USD, MASHA_BUNDLE_HOST_DIR.
#   YENİ: GOOS/GOARCH (hedef binary — dist-bin'de bu hedef YOKSA dürüst hata; default host),
#         MASHA_BIN_DIR (build-binaries.sh çıktısı, default ./dist-bin).
#
# Çıktı: dist-bundle/<tenant>-masha-<GOOS>.zip (ör. yalin-masha-darwin.zip) — bundle.ts §3 OS-farkında çözümü bekler bu adı.
#
# DÜRÜST SINIR: per-tenant client-cert + kod-imzalama = Faz-3 (make-trial-bundle.sh ile AYNI; operatör certs'ini reuse eder).
set -euo pipefail
cd "$(dirname "$0")"

TENANT="${1:?kullanım: MASHA_APP_TOKEN=... EDGE_ADDR=... assemble-tenant.sh <tenant> (ops: REMOTE_PORT, GOOS/GOARCH, MASHA_BIN_DIR)}"
: "${MASHA_APP_TOKEN:?MASHA_APP_TOKEN gerekli (VPS: grep MASHA_APP_TOKEN tenants/$TENANT/.env)}"
LABEL="${MASHA_SERVER_LABEL:-${TENANT}-db}"
GOOS="${GOOS:-$(go env GOOS 2>/dev/null || echo darwin)}"; GOARCH="${GOARCH:-$(go env GOARCH 2>/dev/null || echo arm64)}"
REMOTE_PORT="${REMOTE_PORT:-9800}"
FRPS_PORT="${FRPS_PORT:-7000}"
WEBPASS="${WEB_PASSWORD:-$(openssl rand -hex 6 2>/dev/null || echo degistir-beni)}"
# Edge genel adres (IP/host) + tenant domain'i PARAMETRİK — public repo'ya gerçek IP/domain GİRMEZ (sızıntı disiplini).
EDGE_ADDR="${EDGE_ADDR:-}"
CHIMERA_DOMAIN="${CHIMERA_DOMAIN:-chimera.local}"
[ -n "$EDGE_ADDR" ] || { echo "  ✗ EDGE_ADDR gerekli (edge genel IP/host) — ör. EDGE_ADDR=1.2.3.4 bash assemble-tenant.sh $TENANT"; exit 1; }

BIN_DIR="${MASHA_BIN_DIR:-dist-bin}"
EXT=""; [ "$GOOS" = windows ] && EXT=".exe"
SRC_BIN="$BIN_DIR/masha-agent-${GOOS}-${GOARCH}${EXT}"
[ -f "$SRC_BIN" ] || { echo "  ✗ $SRC_BIN yok — önce: bash build-binaries.sh (MASHA_OS_TARGETS='${GOOS}/${GOARCH} ...')"; exit 1; }

OUT="dist-bundle/${TENANT}-masha-${GOOS}"
rm -rf "$OUT"; mkdir -p "$OUT/certs" "$OUT/manifests"

AGENT_VERSION="$(cat "$BIN_DIR/VERSION" 2>/dev/null || echo '?')"
echo "▸ 1/4 Paylaşımlı binary kopyalanıyor ($GOOS/$GOARCH, sürüm $AGENT_VERSION)…"
cp "$SRC_BIN" "$OUT/masha-agent${EXT}"
chmod +x "$OUT/masha-agent${EXT}" 2>/dev/null || true

echo "▸ 2/4 Tünel enroll (frpc.toml + certs)…"
cp ../certs/ca.crt "$OUT/certs/" 2>/dev/null || echo "  ⚠ ca.crt yok (enroll'dan ekle)"
cp ../certs/agent/client.crt ../certs/agent/client.key "$OUT/certs/" 2>/dev/null || echo "  ⚠ client cert yok (per-tenant cert = enroll, Faz-3)"
FRP_TOKEN="${MASHA_FRP_TOKEN:-$(grep -E '^MASHA_FRP_TOKEN=' ../.env 2>/dev/null | cut -d= -f2- | tr -d '"'"'"'')}"
cat > "$OUT/frpc.toml" <<TOML
serverAddr = "${EDGE_ADDR}"
serverPort = ${FRPS_PORT}
loginFailExit = false
auth.method = "token"
auth.token = "${FRP_TOKEN}"
transport.tls.enable = true
transport.tls.certFile = "./certs/client.crt"
transport.tls.keyFile = "./certs/client.key"
transport.tls.trustedCaFile = "./certs/ca.crt"
transport.tls.serverName = "masha-frps"
[[proxies]]
name = "${TENANT}-${LABEL}"
type = "tcp"
localIP = "127.0.0.1"
localPort = 9800
remotePort = ${REMOTE_PORT}
TOML

echo "▸ 3/4 Deneme config (.masha.env, 0600)…"
( umask 077; cat > "$OUT/.masha.env" <<ENV
MASHA_APP_TOKEN=${MASHA_APP_TOKEN}
MASHA_SERVER_LABEL=${LABEL}
MASHA_VERSION=${AGENT_VERSION}
MASHA_MANIFEST=./manifests/${LABEL}.json
MASHA_PLAN=trial
MASHA_TRIAL_LIMIT_USD=${MASHA_TRIAL_LIMIT_USD:-10}
MASHA_CONTACT_EMAIL=${MASHA_CONTACT_EMAIL:-}
MASHA_WEB_ADDR=0.0.0.0:8787
MASHA_WEB_TLS=1
MASHA_WEB_PASSWORD=${WEBPASS}
MASHA_UPSTREAM_ADDR=0.0.0.0:9800
MASHA_FRPC_CONFIG=./frpc.toml
MASHA_FRPC_BIN=./frpc
ENV
)

# install.sh (Mac/Linux, "masha-agent" uzantısız) + install.ps1 (Windows, "masha-agent.exe") — bundle HER GOOS için
# ikisini de içerir (kullanıcı kendi OS'üne uygun olanı çalıştırır; make-trial-bundle.sh ile AYNI, literal içerik).
echo "▸ 4/4 Tek-komut installer (install.sh + install.ps1) + OKUBENI + zip…"
cat > "$OUT/install.sh" <<'INS'
#!/usr/bin/env bash
set -euo pipefail; cd "$(dirname "$0")"
echo "Chimera Masha (deneme) kuruluyor…"
if [ ! -x ./frpc ]; then
  OS=$(uname -s | tr 'A-Z' 'a-z'); AR=$(uname -m); case "$AR" in x86_64) AR=amd64;; aarch64|arm64) AR=arm64;; esac
  V=0.61.0
  echo "  tünel bileşeni (frpc) indiriliyor ($OS/$AR)…"
  curl -fsSL "https://github.com/fatedier/frp/releases/download/v${V}/frp_${V}_${OS}_${AR}.tar.gz" \
    | tar xz --strip-components=1 -C . "frp_${V}_${OS}_${AR}/frpc" 2>/dev/null \
    || echo "  ⚠ frpc otomatik inmedi — github.com/fatedier/frp'den frpc'yi bu klasöre koyun."
  chmod +x ./frpc 2>/dev/null || true
fi
chmod +x ./masha-agent 2>/dev/null || true
set -a; . ./.masha.env; set +a
nohup ./masha-agent serve > masha.log 2>&1 &
sleep 2
echo "✓ Masha çalışıyor. Yerel yüz:  https://<bu-bilgisayarın-IP>:8787"
echo "  Parola .masha.env içinde. Tarayıcıda aç → Bağlantılar → veritabanını bağla → Kur sihirbazı."
INS
chmod +x "$OUT/install.sh"
cat > "$OUT/install.ps1" <<'PS'
# Chimera Masha (deneme) — Windows. frpc.exe'yi github.com/fatedier/frp'den indirip bu klasöre koyun.
Get-Content .\.masha.env | ForEach-Object { if($_ -match '^\s*([^#=][^=]*)=(.*)$'){ [Environment]::SetEnvironmentVariable($matches[1].Trim(),$matches[2]) } }
Start-Process -NoNewWindow -FilePath .\masha-agent.exe -ArgumentList 'serve' -RedirectStandardOutput .\masha.log
Start-Sleep 2
Write-Host "Masha calisiyor. Yerel yuz: https://<IP>:8787 (parola .masha.env icinde)"
PS

cat > "$OUT/OKUBENI.txt" <<TXT
Chimera Masha — DENEME kurulumu (${TENANT}, ${GOOS})

1) Kur:     Linux/Mac -> ./install.sh        Windows -> powershell -ExecutionPolicy Bypass -File install.ps1
2) Aç:      Tarayıcıda  https://<bu-bilgisayarın-IP>:8787   (self-signed uyarısı -> devam)
            Parola: ${WEBPASS}
3) Tanımla: Bağlantılar -> veritabanını bağla -> "Kur sihirbazı" ile ne göründüğünü seç.
4) Sohbet:  ${TENANT}.${CHIMERA_DOMAIN} -> verinizle konuşun.

Veriniz KUTUNUZDA kalır; buluta yalnız maskeli sonuç çıkar. Salt-okuma.
Talep/yardım: ${MASHA_CONTACT_EMAIL:-operatörünüz}
TXT
ZIP_NAME="${TENANT}-masha-${GOOS}.zip"
# zip taşınabilirliği: VPS/container'da `zip` kurulu OLMAYABİLİR (montaj patlar) → python3 -m zipfile fallback
# (python3 zaten ensure_env/mint_key'de gerekli — daha yaygın kurulu). İkisi de yoksa DÜRÜST hata (sahte-yeşil yok).
# İçerik/isimler AYNI: ikisi de "${TENANT}-masha-${GOOS}/…" dizinini KÖKÜNDEN (relatif) saklar.
( cd dist-bundle && rm -f "$ZIP_NAME"
  if command -v zip >/dev/null 2>&1; then
    zip -qr "$ZIP_NAME" "${TENANT}-masha-${GOOS}"
  elif command -v python3 >/dev/null 2>&1; then
    python3 -m zipfile -c "$ZIP_NAME" "${TENANT}-masha-${GOOS}"
  else
    echo "  ✗ ne 'zip' ne 'python3' bulundu — bundle sıkıştırılamadı (klasör hazır: dist-bundle/${TENANT}-masha-${GOOS}/)"
    exit 1
  fi
)

# Opsiyonel: Pota /kur SELF-SERVICE indirmesi için per-tenant klasöre yayınla (bundle.ts oradan sunar; tenant.yml
# mount /run/chimera/bundles). MASHA_BUNDLE_HOST_DIR set ise (VPS'te ör. /opt/chimera-ai/masha-bundles) yerel-cp;
# operatör laptop'taysa çıktıyı VPS'e rsync/scp et (aşağıdaki ipucu). Set değilse → yalnız zip üretilir (eski davranış).
PUBLISHED=""
if [ -n "${MASHA_BUNDLE_HOST_DIR:-}" ]; then
  DEST="${MASHA_BUNDLE_HOST_DIR%/}/${TENANT}"
  mkdir -p "$DEST"
  cp "dist-bundle/${ZIP_NAME}" "$DEST/"
  PUBLISHED="$DEST/${ZIP_NAME}"
fi

echo
echo "✓ Bundle hazır: stack/masha/agent/dist-bundle/${ZIP_NAME} ($GOOS/$GOARCH)"
if [ -n "$PUBLISHED" ]; then
  echo "  → Pota /kur için yayınlandı: $PUBLISHED  (tenant '${TENANT}' kendi panelinden indirir — operatörsüz)"
else
  echo "  → SELF-SERVICE için yayınla: MASHA_BUNDLE_HOST_DIR=/opt/chimera-ai/masha-bundles ile tekrar çalıştır"
  echo "    (VPS-dışıysan: rsync -a dist-bundle/${ZIP_NAME} root@${EDGE_ADDR}:/opt/chimera-ai/masha-bundles/${TENANT}/)"
fi
echo "  → Elden gönderim de mümkün; tek komut: ./install.sh (Win: install.ps1)"
echo "  ⚠ Edge slot remotePort=${REMOTE_PORT}, dial-in serverPort=${FRPS_PORT}: paylaşılan edge frps'te aynı"
echo "    remotePort'u birden çok EŞ-ZAMANLI tenant kullanamaz; izole per-tenant frps için FRPS_PORT'u o"
echo "    instance'ın FRPS_BIND_PORT'uyla eşle (docs/masha-dcnextgen-macbook-runbook.md)."
echo "  ⚠ Per-tenant client-cert + boot-service (kardianos) = Faz-3; bu bundle operatör certs'ini reuse eder."
