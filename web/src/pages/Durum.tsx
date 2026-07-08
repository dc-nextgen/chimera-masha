// Durum — bağlantılar panosu (tasarım ①): bir bakışta durum + tıklanabilir kartlar + PII dikkat notu.
// §4: rakamlar gerçek (health + manifest export); uydurma yok.
import { useEffect, useState } from 'react'
import { ChevronRightIcon } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { api, type ConnInfo, type Health, type Manifest } from '@/lib/api'
import { humanize, kindLabel } from '@/lib/format'

function maskedFields(m: Manifest): number {
  let n = 0
  for (const ev of Object.values(m.entities))
    for (const f of Object.values(ev.fields)) if ((f.expression || '').startsWith('mask:')) n++
  return n
}

export function Durum({
  health,
  conns,
  onSelect,
}: {
  health: Health | null
  conns: ConnInfo[]
  onSelect?: (c: ConnInfo) => void
}) {
  const up = !!health?.ok
  const connected = conns.filter((c) => c.connected).length
  // Birincil mssql manifest'i (görebilir özeti + PII dikkat için). erpnext generic → manifest yok.
  const [man, setMan] = useState<Manifest | null>(null)
  useEffect(() => {
    api.exportManifest().then(setMan).catch(() => setMan(null))
  }, [conns.length])

  const pii = man ? maskedFields(man) : 0

  const cards: { label: string; value: string; tone?: 'ok' | 'bad' }[] = [
    { label: 'Agent', value: up ? 'Çalışıyor' : 'Durdu', tone: up ? 'ok' : 'bad' },
    { label: 'Aktif bağlantı', value: String(conns.length) },
    { label: 'Bağlı', value: `${connected}/${conns.length}`, tone: connected === conns.length && conns.length > 0 ? 'ok' : undefined },
    { label: 'Açık yetenek', value: String(health?.tools?.length ?? 0) },
  ]

  // mssql bağlantısı için "görebilir" özeti = manifest entity'leri (müdür dili).
  const seeSummary = (c: ConnInfo): string => {
    if (c.kind === 'erpnext') return 'Belge sayımı · listeleme · rapor'
    if (!man) return '—'
    const names = Object.keys(man.entities).map(humanize)
    if (names.length === 0) return 'henüz bir şey açılmadı'
    return names.slice(0, 3).join(', ') + (names.length > 3 ? ` +${names.length - 3}` : '')
  }

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-5">
      <div>
        <h2 className="text-xl font-semibold">Veri bağlantıları</h2>
        <p className="text-muted-foreground text-sm">
          Asistanın <b>yerel ağınızdaki</b> verilere nasıl eriştiğini buradan izlersiniz.
        </p>
      </div>

      {/* Özet şeridi */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {cards.map((c) => (
          <Card key={c.label}>
            <CardContent className="py-3">
              <div
                className={`truncate text-2xl font-semibold ${
                  c.tone === 'ok' ? 'text-emerald-600 dark:text-emerald-400' : c.tone === 'bad' ? 'text-destructive' : ''
                }`}
              >
                {c.value}
              </div>
              <div className="text-muted-foreground text-xs">{c.label}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* PII dikkat notu (gerçek: manifest'te maskeli alan varsa) */}
      {pii > 0 && (
        <div className="flex items-start gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-800 dark:text-amber-300">
          <span aria-hidden>⚠</span>
          <span>
            Açık bağlantılarda <b>kişisel veri</b> var ({pii} alan maskeli). Kimlerin sorabileceğini{' '}
            <b>Chimera Pota'daki kullanıcı/grup</b> ayarlarından sınırlayın.
          </span>
        </div>
      )}

      {/* Bağlantı kartları */}
      <div className="flex flex-col gap-3">
        {conns.length === 0 ? (
          <div className="text-muted-foreground rounded-lg border border-dashed p-6 text-center text-sm">
            Henüz bağlantı yok — "Bağlantılar" sekmesinden ekleyin.
          </div>
        ) : (
          conns.map((c) => (
            <button
              key={c.server_label}
              onClick={() => onSelect?.(c)}
              className="hover:bg-muted/40 flex flex-col gap-2 rounded-xl border p-4 text-left transition"
            >
              <div className="flex flex-wrap items-center gap-2">
                <span className={`size-2.5 rounded-full ${c.connected ? 'bg-emerald-500' : 'bg-rose-500'}`} />
                <span className="font-semibold">{c.label || c.name}</span>
                <Badge variant="outline" className="text-xs">{kindLabel(c.kind)}</Badge>
                <Badge variant="outline" className="border-emerald-500/40 text-xs text-emerald-600 dark:text-emerald-400">
                  Yalnızca okuma
                </Badge>
                <ChevronRightIcon className="text-muted-foreground ml-auto size-4" />
              </div>
              <div className="text-muted-foreground flex flex-wrap gap-x-6 gap-y-1 text-xs">
                <span>
                  <span className="opacity-70">Görebilir:</span> {seeSummary(c)}
                </span>
                <span>
                  <span className="opacity-70">Durum:</span>{' '}
                  {c.connected ? <span className="text-emerald-600 dark:text-emerald-400">sağlıklı</span> : 'bağlı değil'}
                </span>
              </div>
            </button>
          ))
        )}
      </div>

      <p className="text-muted-foreground text-xs">
        Salt-okunur. Ham satır bu kutuda kalır; buluta yalnız maskeli sonuç çıkar. Bulut sohbet bu
        kaynaklara yalnız bu ajan üzerinden (tünel) erer.
      </p>
    </div>
  )
}
