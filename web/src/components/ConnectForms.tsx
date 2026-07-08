// Bağlantı formları — hem "yeni ekle" (Bağlantılar) hem "yeniden bağlan" (Sihirbaz Adım 1 / Detay).
// Kimlik YERELDE kalır (§3); buluta/log'a gitmez. DB=/db/connect, ErpNext=/erpnext/connect.
// UX yeniden-tasarım (2026-07-07): güvenli varsayılan (trust-cert KAPALI), teknik alanlar "Gelişmiş"
// altında katlı, en-az-yetki hazır-komut, bağlanınca "N kaynak bulundu" geri bildirimi.
import { useState } from 'react'
import { ChevronRightIcon } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { api, type DBFields } from '@/lib/api'

function msgClass(m: string): string {
  return m.startsWith('✓')
    ? 'text-sm text-emerald-600 dark:text-emerald-400'
    : 'text-destructive text-sm'
}

// Bağlanınca kaç kaynak (tablo/doctype) görünür → "sına" geri bildirimi + sihirbazda "Devam" için.
async function countSources(conn?: string): Promise<number | null> {
  try {
    const s = await api.schema(conn)
    return s.tables?.length ?? 0
  } catch {
    return null
  }
}

// En-az-yetki hazır SQL — BT ekibine gönderilir. Asistan YALNIZCA okur (db_datareader).
function readonlyGrantSQL(database: string, user: string): string {
  const db = database.trim() || '<veritabanı>'
  const u = (user.trim() || 'chimera_ro').replace(/[^\w]/g, '') || 'chimera_ro'
  return [
    '-- Asistan için SALT-OKUMA kullanıcısı (BT ekibinize gönderin)',
    `CREATE LOGIN [${u}] WITH PASSWORD = '<güçlü-bir-parola>';`,
    `USE [${db}];`,
    `CREATE USER [${u}] FOR LOGIN [${u}];`,
    `ALTER ROLE db_datareader ADD MEMBER [${u}];  -- yalnızca OKUMA; hiçbir şeyi değiştiremez`,
  ].join('\n')
}

export function DbConnectForm({
  onDone,
  onConnected,
}: {
  onDone?: () => void
  onConnected?: (sources: number | null) => void
}) {
  const [f, setF] = useState<DBFields>({
    host: '',
    port: '',
    database: '',
    user: '',
    password: '',
    encrypt: 'true',
    trust_server_cert: false, // GÜVENLİ VARSAYILAN — self-signed sertifikaya kör güven KAPALI (§ tasarım)
  })
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const set = (k: keyof DBFields, v: string | boolean) => setF((p) => ({ ...p, [k]: v }))

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setMsg(null)
    try {
      await api.dbConnect(f)
      const n = await countSources()
      setMsg(n === null ? '✓ Bağlandı — kimlik yerelde saklandı.' : `✓ Bağlantı başarılı — ${n} kaynak bulundu.`)
      onConnected?.(n)
      onDone?.()
    } catch (e) {
      setMsg(String(e).replace(/^Error:\s*/, ''))
    } finally {
      setBusy(false)
    }
  }

  const copyGrant = async () => {
    try {
      await navigator.clipboard.writeText(readonlyGrantSQL(f.database, f.user))
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      /* pano yoksa sessiz */
    }
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="Sunucu adresi" req hint="Örn. sunucu.sirketiniz.local — bilmiyorsanız BT ekibinize sorun.">
          <Input value={f.host} onChange={(e) => set('host', e.target.value)} placeholder="mssql.sirket.local" required />
        </Field>
        <Field label="Veritabanı adı" req hint="Bağlanılacak veritabanının adı.">
          <Input value={f.database} onChange={(e) => set('database', e.target.value)} placeholder="SalesDB" required />
        </Field>
        <Field label="Kullanıcı adı" req>
          <Input value={f.user} onChange={(e) => set('user', e.target.value)} placeholder="salt_okuma_user" required />
        </Field>
        <Field label="Parola" hint="Yalnızca bu cihazda saklanır, buluta gitmez.">
          <Input type="password" value={f.password} onChange={(e) => set('password', e.target.value)} />
        </Field>
      </div>

      {/* En-az-yetki yardımcısı */}
      <div className="flex items-start gap-3 rounded-lg border border-sky-500/30 bg-sky-500/5 px-3 py-2.5">
        <span aria-hidden className="text-base">🛡️</span>
        <div className="text-sm">
          <b>Öneri: sadece-okuma yetkili bir kullanıcı kullanın.</b> Böylece asistan hiçbir şeyi
          değiştiremez, yalnızca okur.{' '}
          <button type="button" onClick={copyGrant} className="font-semibold text-sky-600 underline dark:text-sky-400">
            {copied ? '✓ Kopyalandı' : 'Hazır komutu kopyala → BT ekibine gönder'}
          </button>
        </div>
      </div>

      {/* Gelişmiş — port, şifreleme, sertifika (çoğu kullanıcının dokunmasına gerek yok) */}
      <Collapsible className="rounded-lg border">
        <CollapsibleTrigger className="hover:bg-muted/50 flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium [&[data-state=open]>svg]:rotate-90">
          <ChevronRightIcon className="text-muted-foreground size-4 transition-transform" />
          Gelişmiş ayarlar
          <span className="text-muted-foreground text-xs font-normal">— port, şifreleme, sertifika</span>
        </CollapsibleTrigger>
        <CollapsibleContent className="flex flex-col gap-3 px-3 pb-3 pt-1">
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Port">
              <Input value={f.port} onChange={(e) => set('port', e.target.value)} placeholder="1433 (varsayılan)" />
            </Field>
            <Field label="Şifreleme (encrypt)">
              <select
                value={f.encrypt}
                onChange={(e) => set('encrypt', e.target.value)}
                className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
              >
                <option value="true">true (önerilen)</option>
                <option value="false">false</option>
                <option value="disable">disable</option>
              </select>
            </Field>
          </div>
          <label className="flex items-start gap-2 text-sm">
            <input
              type="checkbox"
              className="mt-0.5"
              checked={!!f.trust_server_cert}
              onChange={(e) => set('trust_server_cert', e.target.checked)}
            />
            <span>
              Sunucu sertifikasına güven <span className="text-muted-foreground">(self-signed DB sertifikası)</span>
              <span className="mt-0.5 block text-xs text-amber-600 dark:text-amber-400">
                ⚠ Güvenliği gevşetir — yalnızca sunucunuzun sertifikasına güveniyorsanız açın.
              </span>
            </span>
          </label>
        </CollapsibleContent>
      </Collapsible>

      {msg && <span className={msgClass(msg)}>{msg}</span>}
      <div>
        <Button type="submit" disabled={busy}>
          {busy ? 'Bağlanıyor…' : 'Bağlan & sına'}
        </Button>
      </div>
    </form>
  )
}

export function ErpConnectForm({ onDone }: { onDone?: () => void }) {
  const [erp, setErp] = useState({ url: '', api_key: '', api_secret: '' })
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState<string | null>(null)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setMsg(null)
    try {
      await api.erpnextConnect(erp)
      setMsg('✓ ErpNext bağlandı — kimlik yerelde saklandı.')
      onDone?.()
    } catch (e) {
      setMsg(String(e).replace(/^Error:\s*/, ''))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <Field label="ErpNext adresi" req hint="Yerel ağınızdaki ya da buluttaki ERP adresi.">
        <Input value={erp.url} onChange={(e) => setErp({ ...erp, url: e.target.value })} placeholder="https://erp.sirket.local" required />
      </Field>
      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="API Key">
          <Input value={erp.api_key} onChange={(e) => setErp({ ...erp, api_key: e.target.value })} />
        </Field>
        <Field label="API Secret" hint="Yalnızca bu cihazda saklanır (§3).">
          <Input type="password" value={erp.api_secret} onChange={(e) => setErp({ ...erp, api_secret: e.target.value })} />
        </Field>
      </div>
      {msg && <span className={msgClass(msg)}>{msg}</span>}
      <div>
        <Button type="submit" disabled={busy}>
          {busy ? 'Bağlanıyor…' : 'Bağlan & sına'}
        </Button>
      </div>
    </form>
  )
}

// Bağlanılacak sistem tipleri — logo + açıklama (tasarım ② tip seçici).
export const CONN_KINDS = [
  { key: 'mssql' as const, title: 'SQL Server veritabanı', desc: 'Satış, üretim, stok gibi veritabanları', logo: '/assets/sql-server-logo.png' },
  { key: 'erpnext' as const, title: 'ErpNext', desc: 'ERP / muhasebe sistemi', logo: '/assets/erpnext-logo.png' },
]

// Yeni bağlantı ekle — kind seç + ilgili form. onDone → liste tazele.
export function AddConnection({ onDone }: { onDone?: () => void }) {
  const [kind, setKind] = useState<'mssql' | 'erpnext'>('mssql')
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-base">Yeni bağlantı ekle</CardTitle>
        <p className="text-muted-foreground text-sm">
          Girdiğiniz kimlik yalnızca bu cihazda saklanır — buluta gitmez. Sadece-okuma bir kullanıcı önerilir.
        </p>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="grid gap-3 sm:grid-cols-2">
          {CONN_KINDS.map((k) => (
            <button
              key={k.key}
              type="button"
              onClick={() => setKind(k.key)}
              className={`flex items-center gap-3 rounded-lg border p-3 text-left transition ${
                kind === k.key ? 'border-primary bg-primary/5 ring-primary/20 ring-1' : 'hover:bg-muted/50'
              }`}
            >
              <img src={k.logo} alt="" className="size-8 shrink-0 object-contain" />
              <div>
                <div className="text-sm font-semibold">{k.title}</div>
                <div className="text-muted-foreground text-xs">{k.desc}</div>
              </div>
            </button>
          ))}
        </div>
        {kind === 'mssql' ? <DbConnectForm onDone={onDone} /> : <ErpConnectForm onDone={onDone} />}
      </CardContent>
    </Card>
  )
}

export function Field({
  label,
  req,
  hint,
  children,
}: {
  label: string
  req?: boolean
  hint?: string
  children: React.ReactNode
}) {
  return (
    <div>
      <label className="text-muted-foreground text-xs font-medium">
        {label}
        {req ? <span className="text-destructive"> *</span> : ''}
      </label>
      {children}
      {hint && <p className="text-muted-foreground mt-1 text-xs">{hint}</p>}
    </div>
  )
}
