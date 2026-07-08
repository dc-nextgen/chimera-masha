// ActivityLog — denetim kaydını (gerçek, /audit/log) müdür diline çevirir (tasarım ⑦ Etkinlik).
// §4: uydurma yok — hash-zincirli gerçek kayıtlar. conn verilirse o bağlantıya süzülür.
import { useEffect, useState } from 'react'

import { api, type AuditRecord } from '@/lib/api'

// Kayıt → okunur cümle (source/decision/tool'a göre).
function describe(r: AuditRecord): string {
  const tool = typeof r.tool === 'string' ? r.tool : ''
  switch (r.source) {
    case 'webui':
      return r.decision === 'error' ? `Araç denendi: ${tool} — hata` : `Araç denendi: ${tool}`
    case 'db-connect':
      return r.decision === 'error'
        ? `Veritabanı bağlantısı başarısız (${r.host ?? ''})`
        : `Veritabanına bağlanıldı (${r.host ?? ''}${r.database ? ' / ' + r.database : ''})`
    case 'erpnext-connect':
      return r.decision === 'error' ? 'ErpNext bağlantısı başarısız' : "ErpNext'e bağlanıldı"
    case 'onboard-apply':
      return r.decision === 'error'
        ? 'Yapılandırma yayına alınamadı'
        : `Yapılandırma yayına alındı${r.connector ? ' · ' + r.connector : ''}${typeof r.tools === 'number' ? ' · ' + r.tools + ' yetenek' : ''}`
    default:
      return tool || String(r.source ?? r.decision ?? 'kayıt')
  }
}

// ts → "bugün 09:14" / "dün 16:40" / "12 Tem" (yerel).
function when(ts?: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  const yst = new Date(now)
  yst.setDate(now.getDate() - 1)
  const hm = d.toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' })
  if (sameDay) return `bugün ${hm}`
  if (d.toDateString() === yst.toDateString()) return `dün ${hm}`
  return d.toLocaleDateString('tr-TR', { day: 'numeric', month: 'short' }) + ' ' + hm
}

export function ActivityLog({ conn, limit = 30 }: { conn?: string; limit?: number }) {
  const [records, setRecords] = useState<AuditRecord[] | null>(null)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    api
      .auditLog(conn, limit)
      .then((r) => setRecords(r.records || []))
      .catch((e) => setErr(String(e).replace(/^Error:\s*/, '')))
  }, [conn, limit])

  if (err) return <div className="text-destructive text-sm">{err}</div>
  if (!records) return <div className="text-muted-foreground text-sm">yükleniyor…</div>
  if (records.length === 0) return <div className="text-muted-foreground text-sm">Henüz etkinlik yok.</div>

  return (
    <div className="overflow-hidden rounded-lg border">
      {records.map((r, i) => (
        <div key={i} className="flex items-start gap-3 border-b px-4 py-2.5 text-sm last:border-b-0">
          <span className="text-muted-foreground w-24 shrink-0 text-xs">{when(r.ts)}</span>
          <span className={`flex-1 ${r.decision === 'error' ? 'text-destructive' : ''}`}>{describe(r)}</span>
          {r.decision === 'allow' && <span className="text-emerald-600 dark:text-emerald-400">✓</span>}
          {r.decision === 'error' && <span className="text-destructive">✕</span>}
        </div>
      ))}
    </div>
  )
}
