// Bağlantı detayı (yayın-sonrası izleme + yönetim) — tasarım ⑦. Sekmeler: Özet · Araçlar · Şema · Etkinlik.
// mssql'de "MCP Kur sihirbazı" (Wizard) buradan açılır; erpnext hazır gelir (araçlar sabit, maske otomatik).
import { useEffect, useState } from 'react'
import { SparklesIcon, WrenchIcon } from 'lucide-react'

import { ActivityLog } from '@/components/ActivityLog'
import { DbConnectForm, ErpConnectForm } from '@/components/ConnectForms'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { api, type ConnInfo, type Manifest } from '@/lib/api'
import { Araclar } from '@/pages/Araclar'
import { Sema } from '@/pages/Sema'

type Tab = 'ozet' | 'araclar' | 'sema' | 'etkinlik'

const kindLabel: Record<string, string> = { mssql: 'SQL Server', erpnext: 'ErpNext' }

export function ConnectionDetail({
  conn,
  onChanged,
  onOpenWizard,
}: {
  conn: ConnInfo
  onChanged?: () => void
  onOpenWizard?: () => void
}) {
  const isDB = conn.kind === 'mssql'
  const [tab, setTab] = useState<Tab>('ozet')
  const [man, setMan] = useState<Manifest | null>(null)

  useEffect(() => {
    if (!isDB) return
    api.exportManifest().then(setMan).catch(() => setMan(null))
  }, [isDB, conn.server_label])

  const tabs: { key: Tab; label: string }[] = [
    { key: 'ozet', label: 'Özet' },
    { key: 'araclar', label: 'Araçlar' },
    { key: 'sema', label: 'Şema' },
    { key: 'etkinlik', label: 'Etkinlik' },
  ]

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2 border-b pb-2">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`rounded-md px-3 py-1.5 text-sm font-medium ${
              tab === t.key ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            {t.label}
          </button>
        ))}
        <Badge variant={conn.connected ? 'default' : 'destructive'} className="ml-auto">
          {conn.connected ? 'sağlıklı' : 'bağlı değil'}
        </Badge>
      </div>

      {tab === 'ozet' && (
        <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
          {/* MCP Kur sihirbazı (mssql) */}
          {isDB && (
            <Card className="border-primary/30 bg-primary/5">
              <CardContent className="flex flex-wrap items-center gap-4 py-4">
                <SparklesIcon className="text-primary size-6 shrink-0" />
                <div className="flex-1">
                  <div className="text-sm font-semibold">MCP kurulum sihirbazı</div>
                  <div className="text-muted-foreground text-sm">
                    Asistanın ne görebileceğini, ne yapabileceğini adım adım belirleyin.
                    {man && (
                      <> Şu an <b>{Object.keys(man.entities).length} bilgi grubu</b>, <b>{man.tools.length} yetenek</b> açık.</>
                    )}
                  </div>
                </div>
                <Button onClick={onOpenWizard}>{man && man.tools.length ? 'Tanımı düzenle' : 'Kur sihirbazını aç'}</Button>
              </CardContent>
            </Card>
          )}

          {isDB && <ConfigTransfer onImported={onChanged} />}

          {!isDB && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-base">
                  <WrenchIcon className="size-4" /> Hazır bağlantı
                </CardTitle>
              </CardHeader>
              <CardContent className="text-muted-foreground text-sm">
                ErpNext bağlantıları hazır gelir: araçlar sabit (belge sayısı, listeleme, rapor — tümü salt-okuma),
                kişisel veriler otomatik maskelenir. Ayrı bir kurulum adımı gerekmez. Araçları "Araçlar" sekmesinden deneyebilirsiniz.
              </CardContent>
            </Card>
          )}

          {/* Durum kartı */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-base">
                {conn.label || conn.name}
                <Badge variant="outline">{kindLabel[conn.kind] ?? conn.kind}</Badge>
                <Badge variant="outline" className="border-emerald-500/40 text-emerald-600 dark:text-emerald-400">
                  Yalnızca okuma
                </Badge>
                <span className="text-muted-foreground ml-auto font-mono text-xs">server:{conn.server_label}</span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {/* Kimlik / yeniden bağlan — katlı (nadir ihtiyaç) */}
              <Collapsible className="rounded-lg border">
                <CollapsibleTrigger className="hover:bg-muted/50 flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium">
                  Kimlik / yeniden bağlan
                  <span className="text-muted-foreground text-xs font-normal">— kimlik yalnız bu cihazda saklanır (§3)</span>
                </CollapsibleTrigger>
                <CollapsibleContent className="px-3 pb-3 pt-1">
                  {isDB ? <DbConnectForm onDone={onChanged} /> : <ErpConnectForm onDone={onChanged} />}
                </CollapsibleContent>
              </Collapsible>
            </CardContent>
          </Card>
        </div>
      )}

      {tab === 'araclar' && <Araclar key={conn.server_label} conn={conn.server_label} />}
      {tab === 'sema' && <Sema key={conn.server_label} conn={conn.server_label} />}
      {tab === 'etkinlik' && (
        <div className="mx-auto w-full max-w-2xl">
          <p className="text-muted-foreground mb-3 text-sm">
            Bu bağlantıda kim neyi ne zaman yaptı — gerçek denetim kaydı (kurcalanamaz hash-zinciri).
          </p>
          <ActivityLog conn={conn.server_label} />
        </div>
      )}
    </div>
  )
}

// ConfigTransfer — yapılandırmayı taşı (§19.3): dışa aktar (JSON indir, KİMLİK YOK) / içe aktar
// (JSON → doğrula → uygula). Danışman bir kez tanımlar, başka müşteriye taşır.
function ConfigTransfer({ onImported }: { onImported?: () => void }) {
  const [msg, setMsg] = useState<string | null>(null)

  const exportCurrent = async () => {
    setMsg(null)
    try {
      const m = await api.exportManifest()
      const blob = new Blob([JSON.stringify(m, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${m.name || 'connector'}.manifest.json`
      a.click()
      URL.revokeObjectURL(url)
    } catch (e) {
      setMsg(String(e).replace(/^Error:\s*/, ''))
    }
  }

  const importFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setMsg(null)
    try {
      const m = JSON.parse(await file.text()) as Manifest
      if (!m.entities || !m.tools) throw new Error('geçersiz yapılandırma (entities/tools yok)')
      if (!window.confirm(`"${m.name || 'connector'}" yapılandırmasını uygula? Mevcut tanımın yerini alır (geri alınabilir, kayda yazılır).`)) return
      const r = await api.apply(m)
      setMsg(`✓ Uygulandı — ${r.tools.length} yetenek.`)
      onImported?.()
    } catch (err) {
      setMsg('İçe aktarma hatası: ' + String(err).replace(/^Error:\s*/, ''))
    } finally {
      e.target.value = ''
    }
  }

  return (
    <Collapsible className="rounded-lg border">
      <CollapsibleTrigger className="hover:bg-muted/50 flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium">
        Yapılandırmayı taşı
        <span className="text-muted-foreground text-xs font-normal">— başka müşteriye aktar (kimlik hariç §3)</span>
      </CollapsibleTrigger>
      <CollapsibleContent className="flex flex-col gap-2 px-3 pb-3 pt-1">
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" onClick={exportCurrent}>
            ⬇ Dışa aktar
          </Button>
          <label className="border-input bg-background hover:bg-muted inline-flex h-8 cursor-pointer items-center rounded-md border px-3 text-sm font-medium">
            ⬆ İçe aktar
            <input type="file" accept="application/json,.json" className="hidden" onChange={importFile} />
          </label>
        </div>
        {msg && (
          <span className={msg.startsWith('✓') ? 'text-sm text-emerald-600 dark:text-emerald-400' : 'text-destructive text-sm'}>
            {msg}
          </span>
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}
