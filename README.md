# masha-agent — on-prem DB connector ajanı (Go tek-binary)

Müşteri kutusunda (SQL Server'ın yanında), arka-plan servis olarak koşar. Bulut sohbet
(`müşteri.chimera-ai.com.tr/sohbet`) veritabanına **yalnız buradan**, mTLS reverse-tunnel
üzerinden erişir. **MCP yürütmesi burada (kutuda); tanım (manifest) bulutta üretilir, iner.**
Ham satır kutuda kalır; tünelden yalnız PII-temiz sonuç çıkar.

Tam tasarım + fazlar: [`docs/masha-plan.md` §17–§18](../../../docs/masha-plan.md).

## Neden Go tek-binary
Docker / ayrı Node·Python runtime yok → müşteri kutusunda kurulum friction'ı yok. Native
arka-plan servis (Windows Service / launchd, Faz 3). İmzalama self-service ölçeğine ertelendi
(§17.9) — şimdilik imzasız + admin kurulumu.

## Yapı
```
main.go                       komutlar: serve | introspect | version  (web/dist EMBED)
internal/
  config/      env → Config (sır DB kimliği YEREL)
  manifest/    MCP TANIMI (connectors/<name>/manifest.json deseni) + Validate (fail-closed)
  connector/   Connector arayüzü (parametrik) + mssql/ (go-mssqldb: introspeksiyon + sqlgen)
  expression/  per-alan maske/format (ON-PREM, tünelden önce)
  toolserver/  bulut-yüzey: OpenAPI + bearer-auth + allowlist + audit (app.py'nin Go portu)
  audit/       hash-zincirli denetim (tamper-evident)
  webui/       yerel yüz: gömülü SPA sunar + JSON API (/healthz /tools /try /schema)
  tunnel/      frpc sidecar (mevcut frpc.toml)
web/                          yerel yüz — React + Vite + shadcn/ui (radix-nova) + Tailwind v4
  src/pages/                  Durum · Araçlar (dene) · Şema (introspeksiyon)
  → npm run build → web/dist → Go binary'ye //go:embed (tek-binary korunur)
example/manifest.json         örnek MSSQL manifest (count + query + filtre + expression)
```

## Derle & çalıştır
```bash
make build          # web (npm) + go → masha-agent (UI gömülü)
# veya ayrı: cd web && npm install && npm run build ; go build -o masha-agent .

MASHA_APP_TOKEN=<bearer>            # OWUI tool-server key ile AYNI (ZORUNLU)
MASHA_MANIFEST=./example/manifest.json   # connector manifest (ZORUNLU)
MASHA_DB_DSN='sqlserver://user:pass@host:1433?database=Db'  # yerel; buluta gitmez
MASHA_FRPC_CONFIG=./frpc.toml      # boş = tünel KAPALI (yalnız-yerel)
./masha-agent serve
```
- **Yerel yüz (React+shadcn):** `http://127.0.0.1:8787/` — Durum · Araçlar (tarayıcıdan dene) · Şema
- Tool-server (tünel upstream): `127.0.0.1:9800/<server>/openapi.json` · `POST .../<tool>` (bearer)

Yüz geliştirme (hot reload): `make dev` (Vite :5173, API'yi :8787'ye proxy'ler) + ayrı terminalde `make run`.

`web/dist` build artefaktıdır (gitignored; `.gitkeep` embed'in kırılmaması için tutulur) — `make build` üretir.

Şema tara (onboarding; **satır okumaz**):
```bash
MASHA_MANIFEST=... MASHA_DB_DSN=... ./masha-agent introspect
```

## Güvenlik sınırı (korunur)
Bearer-auth (sabit-zamanlı) · yol TAM `[server, leaf]` (traversal kapalı) · tool **allowlist**
(manifest) + yazma-fiili reddi (fail-closed) · **serbest SQL yok** (yalnız manifest'ten parametreli
SELECT; kullanıcı değeri daima parametre) · read-only iki katman (DB-login GRANT SELECT-only +
statement) · hash-zincirli audit.

## Durum — Faz 1 (çekirdek) + yerel yüz + Faz 2 (onboarding çekirdeği)
✅ manifest+validate · mssql introspeksiyon+sqlgen · expression · toolserver+openapi+auth+allowlist ·
audit · **yerel yüz React+shadcn (gömülü)** · tunnel(sidecar) · **canlı MSSQL e2e** (count/query + maske + şema) ·
**bulut /sohbet e2e** (dc-nextgen "Auroville MCP", frp tünel → OWUI → gerçek DB).
**Faz 2 çekirdek ✅:** `internal/onboard` üreteç (şema→aday manifest, heuristik PII/format) + `internal/live`
(atomik manifest → **hot-reload**, restart yok) + webui `/onboard/suggest`·`/onboard/apply` + **Kur** ekranı
(`web/src/pages/Onboarding.tsx`). 35 Go test.
**Bekleyen:** LLM eşleme önerisi (Faz 2.4); frp embed; keychain; `kardianos/service` (Faz 3). Bkz `docs/masha-plan.md §18`.
