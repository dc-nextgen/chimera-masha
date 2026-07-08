# Chimera Masha — on-prem veri bağlayıcı ajanı

**Masha, kendi sunucunuzda çalışan küçük bir bağlayıcıdır.** Chimera bulut sohbeti verinize
**yalnız bu ajan üzerinden**, şifreli ve **yalnızca-dışa-arayan** bir tünelle erişir. Ham veri
kutunuzdan çıkmaz; tünelden yalnız **PII-temiz, maskeli sonuç** geçer.

Bu depo **açık kaynaktır** (Apache-2.0) çünkü şirket ağınıza kurduğunuz şeyin **tam olarak ne
yaptığını görebilmeniz** gerekir. Aşağıdaki güvenceleri koda bakarak doğrulayabilirsiniz.

## Ne yapar / ne YAPMAZ (güvenceler — kodda doğrulanabilir)
- ✅ **Salt-okunur.** Yalnızca parametreli `SELECT` üretir; `INSERT/UPDATE/DELETE/DDL` yoktur.
  → `internal/connector/mssql/sqlgen.go` · `internal/connector/mssql/tools.go`.
- ✅ **Veri kutunuzda kalır.** DB kimliği yereldedir, buluta gönderilmez. Alan değerleri
  **tünelden önce** maskelenir/biçimlenir. → `internal/expression/` · `internal/config/`.
- ✅ **Yalnızca-dışa tünel.** Ajan dışarı arar (reverse-tunnel); ağınıza gelen bir port açılmaz.
  → `internal/tunnel/`.
- ✅ **Tanımlı araçlar dışına çıkamaz.** Yalnızca manifest'te tanımlı sorgular çalışır
  (parametreli, allowlist'li; kullanıcı değeri daima parametre). → `internal/manifest/` · `internal/toolserver/`.
- ✅ **Kurcalama-belirgin denetim.** Her erişim hash-zincirli audit'e yazılır. → `internal/audit/`.
- ✅ **Bearer-auth (sabit-zamanlı) + kimlik.** Bulut yüzeyi token ister; yol TAM `[server, leaf]`
  (traversal kapalı); yerel yüz parola/loopback korumalı. → `internal/toolserver/` · `internal/webui/auth.go`.

## Yeniden-üretilebilir kurulum (kurduğunuzun bu kaynak olduğunu doğrulayın)
Size ulaşan kurulum paketindeki ikili dosyayı bu kaynaktan kendiniz derleyip karşılaştırabilirsiniz:
```bash
make build          # web (npm) + go → masha-agent (UI gömülü tek-binary)
# veya ayrı: cd web && npm install && npm run build ; cd .. && CGO_ENABLED=0 go build -o masha-agent .
```
CGO kapalı, saf-Go → Windows/macOS/Linux için çapraz-derlenebilir.

## Çalıştır
```bash
MASHA_APP_TOKEN=<bearer>                     # bulut tarafıyla AYNI (zorunlu)
MASHA_MANIFEST=./example/manifest.json       # connector tanımı (zorunlu)
MASHA_DB_DSN='sqlserver://user:pass@host:1433?database=Db'   # YEREL; buluta gitmez
MASHA_FRPC_CONFIG=./frpc.toml                # boş = tünel KAPALI (yalnız-yerel)
./masha-agent serve                          # yerel yüz :8787 · tünel upstream :9800
```
- **Yerel yüz** `http://127.0.0.1:8787/` — durum · araçları tarayıcıdan deneme · şema keşfi. Manifest
  boş başlayabilir → yerel yüzdeki sihirbazla veritabanınızı tanımlarsınız.
- Şema tara (onboarding; **satır okumaz**): `./masha-agent introspect`
- Yüz geliştirme (hot reload): `make dev` (Vite, API'yi :8787'ye proxy'ler) + ayrı terminalde `make run`.

## Yapı
```
main.go                 komutlar: serve | introspect | version   (web/dist gömülü)
internal/
  config/       env → yapılandırma (DB kimliği YEREL kalır)
  manifest/     MCP tanımı + Validate (fail-closed: geçersizse başlamaz)
  connector/    parametrik Connector arayüzü + mssql/ (introspeksiyon + salt-okunur SQL üretimi)
  expression/   per-alan maske/biçim — buluta gitmeden ÖNCE (on-prem)
  toolserver/   bulut yüzeyi: OpenAPI + bearer-auth + allowlist + audit
  audit/        hash-zincirli, kurcalama-belirgin denetim
  webui/        yerel yüz: gömülü SPA + JSON API (/healthz /tools /try /schema)
  tunnel/       yalnızca-dışa reverse-tunnel (frpc)
  onboard/      "veritabanını anla → MCP tanımı öner" (yardımcı; satır okumaz)
web/            yerel yüz — React + Vite + Tailwind (→ web/dist → Go binary'ye gömülür)
example/manifest.json    örnek MSSQL manifest (generic; gerçek şema DEĞİL)
```

## Mimari (kısaca)
**MCP yürütmesi kutuda; connector tanımı (manifest) bulutta üretilir, kutuya iner.** Yani zeka/tanım
Chimera'da, yürütme sizde. Bu depo **yürütme motorudur** + generic bir örnek — gerçek bağlayıcı
tanımlarınız değildir.

## Test
```bash
go test ./...        # birim testler (connector/manifest/expression/toolserver/audit/…)
```

## Yol haritası (gelecek eklemeler)
Aşağıdakiler planlanıyor — öncelik/zamanlama değişebilir. **Önerinizi, isteğinizi veya sorununuzu
[Issues](https://github.com/dc-nextgen/chimera-masha/issues)'dan iletebilirsiniz;** geri bildiriminiz
önceliklendirmemize yardımcı olur.
- **Otomatik güncelleme** — yeni sürüm çıktığında ajanı güvenli şekilde güncelleme (önce "güncelle"
  bildirimi/onayı, sonra tam otomatik).
- **Arka-plan servisi** — Windows Service / macOS launchd / Linux systemd olarak kurulum (yeniden
  başlatmaya dayanıklı).
- **İmzalı ikili dosyalar** — kod imzalama (indirdiğinizin doğruluğunu ek olarak garanti eder).
- **Daha fazla veritabanı** — MSSQL yanında PostgreSQL, MySQL ve diğer kaynaklar.
- **Dosya/doküman bağlayıcı** — bir klasörü/paylaşımı izleyip belgelerle sohbet (salt-okunur).
- **Şema→araç önerisi** — LLM destekli onboarding (veritabanınızı anlayıp araç taslağı önerir).

## Lisans
Apache-2.0 — bkz [`LICENSE`](./LICENSE). Değerimiz kurulum + bakım hizmetindedir; ajanı okumakta,
derlemekte, denetlemekte özgürsünüz.
