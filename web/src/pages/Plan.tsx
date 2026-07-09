// Plan / Yükselt — deneme durumu + ücretsiz↔ücretli özellikler + "Talep ilet" (kullanıcı 2026-07-07).
// Talep = mailto (her yerde çalışır) + "mesajı kopyala" (mail istemcisi yoksa) → operatöre ulaşır.
// Hata-güvenli: /plan başarısızsa bile ekran çöker değil, güvenli varsayılan (normal plan) gösterir.
import { useEffect, useState } from 'react'
import { CheckIcon, XIcon } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { api, type PlanInfo } from '@/lib/api'

// Ücretsiz (deneme) ↔ ücretli özellik haritası. ÖNERİ — operatör/ürün ayarlar (docs/cloud-saas §13.3).
const FEATURES: { label: string; free: boolean; note?: string }[] = [
  { label: 'Bir veri kaynağı bağla (SQL Server / ErpNext)', free: true },
  { label: 'Temel araçlar — say, listele (salt-okuma)', free: true },
  { label: 'Yerel PII maskeleme', free: true },
  { label: 'Sohbette veriyle konuş', free: true, note: 'deneme token limitine kadar' },
  { label: 'Birden çok bağlantı', free: false },
  { label: 'Doküman/klasör ile sohbet (RAG)', free: false, note: 'yakında' },
  { label: 'Dosya sunucusu (SMB) bağlama', free: false, note: 'yakında' },
  { label: 'Yüksek token limiti + otomasyon (workflow)', free: false },
  { label: 'Öncelikli destek', free: false },
]

const REQUEST_TYPES = ['Ücretli sürüme geç', 'Token limitini artır', 'Yardım / kurulum desteği', 'Bilgi / teklif iste']

export function Plan() {
  const [plan, setPlan] = useState<PlanInfo | null>(null)
  const [kind, setKind] = useState(REQUEST_TYPES[0])
  const [msg, setMsg] = useState('')
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    // Hata-güvenli: /plan düşse bile normal plan varsay (ekran çökmesin).
    api.plan().then(setPlan).catch(() => setPlan({ plan: '', trial_limit_usd: '', contact_email: '', request_url: '' }))
  }, [])

  const isTrial = plan?.plan === 'trial'
  const contact = plan?.contact_email || ''

  const composed = `Talep türü: ${kind}\n\nMesaj: ${msg || '(boş)'}\n\n— Masha üzerinden gönderildi`
  const mailto =
    'mailto:' + encodeURIComponent(contact) +
    '?subject=' + encodeURIComponent('[Chimera] ' + kind) +
    '&body=' + encodeURIComponent(composed)

  const copy = async () => {
    try {
      await navigator.clipboard.writeText((contact ? `Kime: ${contact}\n` : '') + composed)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      /* pano yoksa sessiz — mailto zaten var */
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
      {/* Deneme durumu */}
      {isTrial && (
        <Card className="border-amber-500/40 bg-amber-500/5">
          <CardContent className="flex flex-wrap items-center gap-3 py-4">
            <span className="rounded-full bg-amber-500/20 px-3 py-1 text-sm font-semibold text-amber-700 dark:text-amber-300">
              Deneme sürümü
            </span>
            <span className="text-sm">
              {plan?.trial_limit_usd ? <>~${plan.trial_limit_usd} değerinde AI kullanımı dahil.</> : 'Sınırlı deneme.'} Beğenirsen
              aşağıdan ücretli sürüme geçebilirsin — verilerin ve ayarların korunur.
            </span>
          </CardContent>
        </Card>
      )}

      {/* Ücretsiz ↔ Ücretli özellikler */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Ücretsiz ve ücretli özellikler</CardTitle>
          <p className="text-muted-foreground text-sm">
            {isTrial ? 'Deneme sürümünde açık olanlar ✓; ücretli sürümle açılanlar kilitli.' : 'Sürümünüzde açık olan özellikler.'}
          </p>
        </CardHeader>
        <CardContent className="p-0">
          {FEATURES.map((f) => (
            <div key={f.label} className="flex items-center gap-3 border-b px-4 py-2.5 text-sm last:border-b-0">
              {f.free ? (
                <CheckIcon className="size-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
              ) : (
                <XIcon className="text-muted-foreground size-4 shrink-0" />
              )}
              <span className={f.free ? '' : 'text-muted-foreground'}>{f.label}</span>
              {f.note && <span className="text-muted-foreground ml-auto text-xs">{f.note}</span>}
              {!f.free && !f.note && (
                <span className="ml-auto rounded bg-muted px-2 py-0.5 text-xs font-medium">ücretli</span>
              )}
            </div>
          ))}
        </CardContent>
      </Card>

      {/* Talep ilet / Yükselt */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Talebini ilet</CardTitle>
          <p className="text-muted-foreground text-sm">
            Ücretli sürüme geçmek, limit artırmak ya da yardım için bize doğrudan ulaş — en kısa sürede döneriz.
          </p>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <div className="flex flex-wrap gap-2">
            {REQUEST_TYPES.map((t) => (
              <button
                key={t}
                onClick={() => setKind(t)}
                className={`rounded-full border px-3 py-1.5 text-sm ${
                  kind === t ? 'border-primary bg-primary/5 text-primary font-medium' : 'hover:bg-muted/50'
                }`}
              >
                {t}
              </button>
            ))}
          </div>
          <textarea
            value={msg}
            onChange={(e) => setMsg(e.target.value)}
            placeholder="Eklemek istediğin bir not (opsiyonel)…"
            className="border-input bg-background min-h-20 rounded-md border px-3 py-2 text-sm"
          />
          <div className="flex flex-wrap items-center gap-2">
            {contact ? (
              <Button asChild>
                <a href={mailto}>E-posta ile gönder</a>
              </Button>
            ) : (
              <span className="text-muted-foreground text-sm">İletişim adresi tanımlı değil — mesajı kopyalayıp bize iletebilirsin.</span>
            )}
            <Button variant="outline" onClick={copy}>
              {copied ? '✓ Kopyalandı' : 'Mesajı kopyala'}
            </Button>
            {contact && <span className="text-muted-foreground text-xs">{contact}</span>}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
