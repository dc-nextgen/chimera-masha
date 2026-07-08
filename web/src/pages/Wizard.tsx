// Wizard — MCP tanımlama, kılavuzlu 5 adım (Claude Design wireframe, 2026-07-07 onaylı).
// Bağlan · Ne görülsün · Neler yapsın · Kim sorabilir · Dene & Yayına al. Müdür dili; jargon gizli;
// güvenli varsayılan; görebilir/göremez panosu; "eksikler DÜŞER" yerine "neler değişecek" diff.
// SQL Server (manifest) akışı; onboard/* PRIMARY mssql'e bağlı (tek mssql = primary). §4: uydurma yok.
import { useEffect, useMemo, useState } from 'react'
import { ArrowLeftIcon, ArrowRightIcon, CheckCircle2Icon, SparklesIcon } from 'lucide-react'

import { DbConnectForm } from '@/components/ConnectForms'
import { Stepper, type StepDef } from '@/components/Stepper'
import { capability, humanize } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  api,
  EXPRESSIONS,
  type ConnInfo,
  type Manifest,
  type RunResult,
  type Schema,
  type TableAdvice,
  type Tool,
} from '@/lib/api'

const STEPS: StepDef[] = [
  { key: 'baglan', title: 'Bağlan' },
  { key: 'gorulsun', title: 'Ne görülsün' },
  { key: 'yapsin', title: 'Neler yapsın' },
  { key: 'kim', title: 'Kim sorabilir' },
  { key: 'dene', title: 'Dene & Yayına al' },
]

function maskedCount(fields: Manifest['entities'][string]['fields']): number {
  return Object.values(fields).filter((f) => (f.expression || '').startsWith('mask:')).length
}

export function Wizard({ conn, onExit, onChanged }: { conn: ConnInfo; onExit: () => void; onChanged?: () => void }) {
  const [step, setStep] = useState(0)
  const [furthest, setFurthest] = useState(0)
  const [err, setErr] = useState<string | null>(null)

  // Şema + mevcut/aday manifest (Ne görülsün / Neler yapsın / Dene ortak durumu).
  const [schema, setSchema] = useState<Schema | null>(null)
  const [current, setCurrent] = useState<Manifest | null>(null) // yürürlükteki (diff için)
  const [cand, setCand] = useState<Manifest | null>(null) // düzenlenen aday
  const [excluded, setExcluded] = useState<Set<string>>(new Set()) // "entity.field" kapalı
  const [disabledTools, setDisabledTools] = useState<Set<string>>(new Set()) // kapalı yetenekler (tool.name)
  const [advice, setAdvice] = useState<Record<string, TableAdvice>>({}) // key = table.toLowerCase()
  const [adviceMsg, setAdviceMsg] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [tableFilter, setTableFilter] = useState('')
  const [openEditor, setOpenEditor] = useState<string | null>(null) // hangi entity'nin alanları açık

  // Kim sorabilir (Adım 4) — YEREL/pasif (backend yok; erişim bulut Authentik/OWUI grup katmanında).
  const [access, setAccess] = useState<'herkes' | 'roller' | 'kisiler'>('roller')

  // Adım 5 önizleme.
  const [liveTools, setLiveTools] = useState<Tool[]>([])
  const [previewTool, setPreviewTool] = useState('')
  const [previewArgs, setPreviewArgs] = useState<Record<string, string>>({})
  const [preview, setPreview] = useState<RunResult | null>(null)
  const [previewErr, setPreviewErr] = useState<string | null>(null)
  const [published, setPublished] = useState<string | null>(null)

  const label = conn.server_label

  // Aday manifest'i ilk yükle: yürürlükteki manifest'i aday yap (alan/expression KORUNUR).
  useEffect(() => {
    if (conn.kind !== 'mssql') return
    api.schema(label).then(setSchema).catch(() => {})
    api
      .exportManifest()
      .then((m) => {
        setCurrent(m)
        setCand(structuredClone(m))
      })
      .catch(() => {
        setCurrent(null)
        setCand({ name: conn.name || 'connector', label: conn.label || '', erp_kind: '', db: { driver: 'sqlserver', read_only: true }, entities: {}, tools: [] })
      })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [label])

  const goTo = (i: number) => {
    setStep(i)
    setFurthest((f) => Math.max(f, i))
    setErr(null)
  }
  const next = () => goTo(Math.min(STEPS.length - 1, step + 1))
  const back = () => goTo(Math.max(0, step - 1))

  // Tablo ekle/çıkar: eklenen tablo için tek-tablo suggest → aday'a MERGE (diğer düzenlemeler korunur).
  const entityByTable = useMemo(() => {
    const m: Record<string, string> = {}
    if (cand) for (const [ek, ev] of Object.entries(cand.entities)) m[ev.table.toLowerCase()] = ek
    return m
  }, [cand])

  const addTable = async (tableId: string) => {
    if (!cand) return
    setBusy(true)
    setErr(null)
    try {
      const m = await api.suggest({ name: cand.name, label: cand.label, tables: [tableId] })
      setCand((c) => {
        if (!c) return c
        const nc = structuredClone(c)
        for (const [ek, ev] of Object.entries(m.entities)) nc.entities[ek] = ev
        const have = new Set(nc.tools.map((t) => t.name))
        for (const t of m.tools) if (!have.has(t.name)) nc.tools.push(t)
        return nc
      })
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ''))
    } finally {
      setBusy(false)
    }
  }

  const removeEntity = (ek: string) => {
    setCand((c) => {
      if (!c) return c
      const nc = structuredClone(c)
      delete nc.entities[ek]
      nc.tools = nc.tools.filter((t) => t.entity !== ek)
      return nc
    })
  }

  const setExpr = (entity: string, field: string, expr: string) =>
    setCand((c) => {
      if (!c) return c
      const nc = structuredClone(c)
      if (expr) nc.entities[entity].fields[field].expression = expr
      else delete nc.entities[entity].fields[field].expression
      return nc
    })

  const toggleField = (entity: string, field: string) =>
    setExcluded((s) => {
      const key = `${entity}.${field}`
      const n = new Set(s)
      n.has(key) ? n.delete(key) : n.add(key)
      return n
    })

  const toggleTool = (name: string) =>
    setDisabledTools((s) => {
      const n = new Set(s)
      n.has(name) ? n.delete(name) : n.add(name)
      return n
    })

  // AI danışman: aday tabloları sınıflandır → PII/isim öner (adaya işlenir). LLM yoksa 501 bilgi.
  const runAdvise = async () => {
    if (!cand) return
    setBusy(true)
    setAdviceMsg(null)
    try {
      const tables = Object.values(cand.entities).map((e) => e.table)
      const res = await api.advise({ tables })
      const byTable: Record<string, TableAdvice> = {}
      for (const a of res.tables) byTable[a.table.toLowerCase()] = a
      setAdvice(byTable)
      setCand((c) => {
        if (!c) return c
        const nc = structuredClone(c)
        for (const ev of Object.values(nc.entities)) {
          const a = byTable[ev.table.toLowerCase()]
          if (!a?.fields) continue
          const byCol: Record<string, string> = {}
          for (const f of a.fields) if (f.expression) byCol[f.column.toLowerCase()] = f.expression
          for (const fv of Object.values(ev.fields)) {
            const e = byCol[fv.column.toLowerCase()]
            if (e) fv.expression = e
          }
        }
        return nc
      })
      const sens = res.tables.filter((a) => a.sensitive).length
      setAdviceMsg(`Asistan ${res.tables.length} bilgi grubunu inceledi · ${sens} tanesinde kişisel/hassas veri işaretlendi.`)
    } catch (e) {
      setAdviceMsg(
        String(e).includes('501')
          ? 'Yapılandırma asistanı bu kurulumda kapalı — maskeler yine de alan adından otomatik önerildi.'
          : String(e).replace(/^Error:\s*/, ''),
      )
    } finally {
      setBusy(false)
    }
  }

  // assemble — düzenlenen adaydan geçerli manifest (kapalı alan + kapalı yetenek çıkar; boş-select tool düşer).
  const assemble = (c: Manifest): Manifest => {
    const entities: Manifest['entities'] = {}
    for (const [ek, ev] of Object.entries(c.entities)) {
      const fields: (typeof ev)['fields'] = {}
      for (const [fk, fv] of Object.entries(ev.fields)) if (!excluded.has(`${ek}.${fk}`)) fields[fk] = fv
      entities[ek] = { table: ev.table, fields }
    }
    const tools: Manifest['tools'] = []
    for (const t of c.tools) {
      if (disabledTools.has(t.name)) continue
      if (t.kind === 'query') {
        const sel = (t.select || []).filter((f) => !excluded.has(`${t.entity}.${f}`))
        if (sel.length === 0) continue
        tools.push({ ...t, select: sel })
      } else tools.push(t)
    }
    return { ...c, entities, tools }
  }

  const finalManifest = useMemo(() => (cand ? assemble(cand) : null), [cand, excluded, disabledTools])

  // Diff: yayınlayınca ne eklenir/kaldırılır (entity + tool bazında; current vs final).
  const diff = useMemo(() => {
    const cur = current
    const fin = finalManifest
    if (!fin) return { addedEnt: [], removedEnt: [], addedTool: 0, removedTool: 0 }
    const curEnt = new Set(cur ? Object.keys(cur.entities) : [])
    const finEnt = new Set(Object.keys(fin.entities))
    const curTool = new Set(cur ? cur.tools.map((t) => t.name) : [])
    const finTool = new Set(fin.tools.map((t) => t.name))
    return {
      addedEnt: [...finEnt].filter((e) => !curEnt.has(e)),
      removedEnt: [...curEnt].filter((e) => !finEnt.has(e)),
      addedTool: [...finTool].filter((t) => !curTool.has(t)).length,
      removedTool: [...curTool].filter((t) => !finTool.has(t)).length,
    }
  }, [current, finalManifest])

  const publish = async () => {
    if (!finalManifest) return
    setBusy(true)
    setErr(null)
    try {
      const r = await api.apply(finalManifest)
      setPublished(`${r.tools.length} yetenek yayına alındı — asistan artık hazır.`)
      setCurrent(structuredClone(finalManifest))
      onChanged?.()
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ''))
    } finally {
      setBusy(false)
    }
  }

  const loadLiveTools = () => {
    api.tools(label).then((r) => {
      setLiveTools(r.tools)
      if (r.tools.length && !previewTool) setPreviewTool(r.tools[0].name)
    }).catch(() => {})
  }
  const runPreview = async () => {
    if (!previewTool) return
    setPreviewErr(null)
    setPreview(null)
    const a: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(previewArgs)) if (v.trim()) a[k] = v.trim()
    try {
      setPreview(await api.run(previewTool, a, label))
    } catch (e) {
      setPreviewErr(String(e).replace(/^Error:\s*/, ''))
    }
  }
  useEffect(() => {
    if (step === 4) loadLiveTools()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step])

  if (conn.kind !== 'mssql') {
    return (
      <div className="text-muted-foreground text-sm">
        Bu sihirbaz SQL Server bağlantıları içindir. ErpNext gibi bağlantılar hazır gelir (araçlar sabit,
        maskeleme otomatik) — detay ekranından yönetin.
      </div>
    )
  }

  const entities = cand ? Object.entries(cand.entities) : []
  const totalMasked = entities.reduce((n, [, ev]) => n + maskedCount(ev.fields), 0)
  const piiEntities = entities.filter(([, ev]) => maskedCount(ev.fields) > 0 || advice[ev.table.toLowerCase()]?.sensitive)

  const schemaTables = (schema?.tables ?? []).filter((t) =>
    tableFilter ? `${t.schema}.${t.name}`.toLowerCase().includes(tableFilter.toLowerCase()) : true,
  )

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-5">
      <div className="rounded-xl border bg-card p-4">
        <Stepper steps={STEPS} active={step} furthest={furthest} onGo={goTo} />
      </div>

      {err && (
        <div className="border-destructive/40 bg-destructive/10 text-destructive rounded-md border px-3 py-2 text-sm">
          {err}
        </div>
      )}

      {/* ───────── Adım 1 · Bağlan ───────── */}
      {step === 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-lg">Şirket sisteminize bağlanın</CardTitle>
            <p className="text-muted-foreground text-sm">
              Bilgileri BT ekibinizden alabilirsiniz. Girdiğiniz parola yalnızca bu cihazda saklanır, buluta gitmez.
            </p>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <div className="flex items-start gap-3 rounded-lg border bg-muted/40 px-3 py-2.5 text-sm">
              <span aria-hidden>🏢</span>
              <span className="text-muted-foreground">
                Bu uygulamadaki tüm bağlantılar <b>yerel ağınızda</b> kalır — veriniz dışarı çıkmaz. Bulut
                sistemlerinizi bağlamak için Chimera Pota'yı kullanın.
              </span>
            </div>
            {conn.connected && (
              <div className="flex items-center gap-2 rounded-md border border-emerald-500/40 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700 dark:text-emerald-400">
                <CheckCircle2Icon className="size-4" /> Bu bağlantı şu an bağlı. Kimliği güncellemek için formu kullanın, ya da devam edin.
              </div>
            )}
            <DbConnectForm onDone={onChanged} onConnected={() => setFurthest((f) => Math.max(f, 1))} />
          </CardContent>
          <CardContent className="flex items-center pt-0">
            <div className="ml-auto">
              <Button onClick={next} disabled={!conn.connected}>
                Devam <ArrowRightIcon className="size-4" />
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* ───────── Adım 2 · Ne görülsün ───────── */}
      {step === 1 && cand && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-lg">Asistan hangi bilgileri görebilsin?</CardTitle>
            <p className="text-muted-foreground text-sm">Yalnızca burada açtıklarınızı görür. Kapalı olanlara asla erişemez.</p>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            {/* Erişim uyarısı — MUST (§17.7): sohbet, açılanları DB-login yetkisi kadar herkese açar */}
            <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2.5 text-sm text-amber-800 dark:text-amber-300">
              <b>⚠ Açtığınız bilgiler, asistanla konuşan herkese açılır</b> — bağlanan kullanıcının okuma
              yetkisi kadar. ERP'deki kişiye-özel yetkiler burada geçmez. Yalnızca herkesin görmesinde
              sakınca olmayan bilgileri açın.
            </div>

            {totalMasked > 0 && (
              <div className="flex items-start gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2.5 text-sm text-amber-800 dark:text-amber-300">
                <span aria-hidden>🔒</span>
                <span>
                  <b>{piiEntities.length} bilgi grubunda kişisel veri bulundu</b> (isim, e-posta, telefon vb.).
                  Bunları güvenlik için <b>otomatik maskeledik</b> — asistan sayabilir ama kişilerin bilgilerini gösteremez.
                </span>
              </div>
            )}

            {/* Açık bilgi grupları (aday entity'ler) */}
            <div>
              <div className="mb-2 flex items-center gap-2">
                <span className="text-sm font-semibold">Açık bilgi grupları</span>
                <Badge variant="secondary">{entities.length}</Badge>
              </div>
              {entities.length === 0 ? (
                <div className="text-muted-foreground rounded-md border border-dashed p-4 text-sm">
                  Henüz bilgi grubu açık değil — aşağıdan ekleyin.
                </div>
              ) : (
                <div className="flex flex-col gap-2">
                  {entities.map(([ek, ev]) => {
                    const friendly = humanize(ek)
                    const masked = maskedCount(ev.fields)
                    const a = advice[ev.table.toLowerCase()]
                    const editing = openEditor === ek
                    return (
                      <div key={ek} className="rounded-lg border">
                        <div className="flex flex-wrap items-center gap-2 px-3 py-2.5 text-sm">
                          <span className="font-semibold">{friendly}</span>
                          {(masked > 0 || a?.sensitive) && (
                            <Badge variant="outline" className="border-amber-500/50 text-amber-600 dark:text-amber-400">
                              kişisel veri{masked > 0 ? ' · gizli' : ''}
                            </Badge>
                          )}
                          <span className="text-muted-foreground font-mono text-xs">{ev.table}</span>
                          <span className="text-muted-foreground text-xs">
                            {Object.keys(ev.fields).length} alan{masked > 0 ? ` · ${masked} maskeli` : ''}
                          </span>
                          <span className="ml-auto flex gap-2">
                            <button
                              onClick={() => setOpenEditor(editing ? null : ek)}
                              className="text-primary text-xs font-semibold"
                            >
                              {editing ? 'Kapat' : 'Alanları düzenle'}
                            </button>
                            <button onClick={() => removeEntity(ek)} className="text-muted-foreground hover:text-destructive text-xs">
                              Kaldır
                            </button>
                          </span>
                        </div>
                        {editing && (
                          <div className="border-t">
                            {Object.entries(ev.fields).map(([fk, fv]) => {
                              const off = excluded.has(`${ek}.${fk}`)
                              return (
                                <div key={fk} className={`flex items-center gap-2 border-b px-3 py-1.5 text-sm last:border-b-0 ${off ? 'opacity-40' : ''}`}>
                                  <input type="checkbox" checked={!off} onChange={() => toggleField(ek, fk)} title="asistana göster" />
                                  <span className="w-40 truncate font-medium">{humanize(fk)}</span>
                                  <span className="text-muted-foreground w-36 truncate font-mono text-xs">{fv.column}</span>
                                  <select
                                    value={fv.expression ?? ''}
                                    onChange={(e) => setExpr(ek, fk, e.target.value)}
                                    disabled={off}
                                    className="border-input bg-background ml-auto rounded-md border px-2 py-1 text-xs"
                                  >
                                    {EXPRESSIONS.map((x) => (
                                      <option key={x} value={x}>
                                        {x ? x : '— (olduğu gibi)'}
                                      </option>
                                    ))}
                                  </select>
                                </div>
                              )
                            })}
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>
              )}
            </div>

            {/* Bilgi grubu ekle (şemadan) */}
            <Collapsible className="rounded-lg border">
              <CollapsibleTrigger className="hover:bg-muted/50 flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium">
                + Bilgi grubu ekle
                <span className="text-muted-foreground text-xs font-normal">— {schema?.tables.length ?? 0} kaynak arasından</span>
              </CollapsibleTrigger>
              <CollapsibleContent className="flex flex-col gap-2 px-3 pb-3 pt-1">
                <Input placeholder="ara, ör. katılımcı…" value={tableFilter} onChange={(e) => setTableFilter(e.target.value)} className="max-w-xs" />
                <div className="max-h-64 overflow-y-auto rounded-md border">
                  {schemaTables.map((t) => {
                    const tid = `${t.schema}.${t.name}`
                    const added = !!entityByTable[tid.toLowerCase()]
                    return (
                      <div key={tid} className="flex items-center gap-3 border-b px-3 py-2 text-sm last:border-b-0">
                        <span className="font-medium">{humanize(t.name)}</span>
                        <span className="text-muted-foreground font-mono text-xs">{tid}</span>
                        <span className="text-muted-foreground ml-auto text-xs">{t.columns.length} alan</span>
                        {added ? (
                          <Badge variant="secondary" className="text-xs">eklendi</Badge>
                        ) : (
                          <Button size="sm" variant="outline" disabled={busy} onClick={() => addTable(tid)}>
                            Ekle
                          </Button>
                        )}
                      </div>
                    )
                  })}
                </div>
              </CollapsibleContent>
            </Collapsible>

            {/* Asistan önerisi */}
            <div className="flex items-start gap-3 rounded-lg border border-violet-500/30 bg-violet-500/5 px-3 py-2.5 text-sm">
              <SparklesIcon className="mt-0.5 size-4 text-violet-500" />
              <div>
                <b>Ne açacağınızdan emin değil misiniz?</b> Asistan bilgi gruplarını inceleyip kişisel veri
                içerenleri işaretlesin ve uygun maskeleri önersin.{' '}
                <button onClick={runAdvise} disabled={busy} className="font-semibold text-violet-600 underline dark:text-violet-400">
                  {busy ? 'İnceliyor…' : 'Asistana incelet'}
                </button>
                {adviceMsg && <div className="text-muted-foreground mt-1 text-xs">{adviceMsg}</div>}
              </div>
            </div>

            <NavRow onBack={back} onNext={next} nextDisabled={entities.length === 0} />
          </CardContent>
        </Card>
      )}

      {/* ───────── Adım 3 · Neler yapsın ───────── */}
      {step === 2 && cand && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-lg">Asistan neleri yapabilsin?</CardTitle>
            <p className="text-muted-foreground text-sm">
              Açtığınız bilgilerle asistanın yapabileceği şeyler. İstemediklerinizi kapatabilirsiniz. Tümü salt-okumadır.
            </p>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            {cand.tools.length === 0 ? (
              <div className="text-muted-foreground rounded-md border border-dashed p-4 text-sm">
                Önce bir bilgi grubu açın (önceki adım) — yetenekler otomatik oluşur.
              </div>
            ) : (
              cand.tools.map((t) => {
                const friendly = humanize(t.entity || '')
                const cap = capability(t.name, friendly)
                const on = !disabledTools.has(t.name)
                return (
                  <div key={t.name} className={`flex items-center gap-4 rounded-lg border px-4 py-3 ${on ? '' : 'opacity-60'}`}>
                    <div className="flex-1">
                      <div className="text-sm font-semibold">{cap.label}</div>
                      {cap.example && (
                        <div className="text-muted-foreground text-xs">
                          Örnek: <i>“{cap.example}”</i>
                          {maskedCount(cand.entities[t.entity]?.fields || {}) > 0 && (
                            <span className="text-emerald-600 dark:text-emerald-400"> · kişisel alanlar maskeli</span>
                          )}
                        </div>
                      )}
                    </div>
                    <button
                      type="button"
                      role="switch"
                      aria-checked={on}
                      onClick={() => toggleTool(t.name)}
                      className={`relative h-6 w-11 shrink-0 rounded-full transition ${on ? 'bg-primary' : 'bg-muted-foreground/30'}`}
                    >
                      <span className={`absolute top-0.5 size-5 rounded-full bg-white transition ${on ? 'right-0.5' : 'left-0.5'}`} />
                    </button>
                  </div>
                )
              })
            )}
            <NavRow onBack={back} onNext={next} />
          </CardContent>
        </Card>
      )}

      {/* ───────── Adım 4 · Kim sorabilir (DÜRÜST PASİF) ───────── */}
      {step === 3 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-lg">Bu bağlantıya kimler soru sorabilir?</CardTitle>
            <p className="text-muted-foreground text-sm">Asistanla konuşup bu verileri görebilecek kişileri belirleyin.</p>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <div className="rounded-lg border border-sky-500/30 bg-sky-500/5 px-3 py-2.5 text-sm text-sky-800 dark:text-sky-300">
              ℹ Erişim şu an <b>Chimera Pota'daki kullanıcı/grup</b> ayarlarından yönetiliyor. Bağlantı-başına
              rol seçimi <b>yakında</b> buraya gelecek — aşağıdaki seçim henüz uygulanmıyor, planınızı belirtmek içindir.
            </div>
            {[
              { key: 'herkes' as const, title: 'Şirketteki herkes', desc: 'Asistana erişimi olan tüm çalışanlar sorabilir' },
              { key: 'roller' as const, title: 'Belirli roller', desc: 'Sadece seçtiğiniz ekipler (ör. Yönetim, Satış)' },
              { key: 'kisiler' as const, title: 'Belirli kişiler', desc: 'Tek tek kişi seçin' },
            ].map((o) => (
              <button
                key={o.key}
                onClick={() => setAccess(o.key)}
                className={`flex items-start gap-3 rounded-lg border px-4 py-3 text-left ${access === o.key ? 'border-primary bg-primary/5 ring-primary/20 ring-1' : 'hover:bg-muted/50'}`}
              >
                <span className={`mt-0.5 flex size-5 items-center justify-center rounded-full border-2 ${access === o.key ? 'border-primary' : 'border-muted-foreground/40'}`}>
                  {access === o.key && <span className="bg-primary size-2.5 rounded-full" />}
                </span>
                <span>
                  <span className="block text-sm font-semibold">{o.title}</span>
                  <span className="text-muted-foreground block text-xs">{o.desc}</span>
                </span>
              </button>
            ))}
            {totalMasked > 0 && access === 'herkes' && (
              <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2.5 text-sm text-amber-800 dark:text-amber-300">
                Bu bağlantı kişisel veri içeriyor. "Herkes"i seçerseniz tüm çalışanlar bu bilgileri sorabilir —
                <b> rol bazlı erişim öneririz.</b>
              </div>
            )}
            <NavRow onBack={back} onNext={next} />
          </CardContent>
        </Card>
      )}

      {/* ───────── Adım 5 · Dene & Yayına al ───────── */}
      {step === 4 && cand && finalManifest && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-lg">Yayına almadan önce gözden geçirin</CardTitle>
            <p className="text-muted-foreground text-sm">Asistanın bu bağlantıyla neyi görebileceğini onaylayın ve yayına alın.</p>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            {/* Görebilir / Göremez panosu */}
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-3">
                <div className="mb-2 text-xs font-bold uppercase tracking-wide text-emerald-700 dark:text-emerald-400">✓ Asistan görebilir</div>
                <ul className="text-sm text-emerald-900/80 dark:text-emerald-200/80">
                  {Object.entries(finalManifest.entities).map(([ek, ev]) => (
                    <li key={ek}>
                      • {humanize(ek)}
                      {maskedCount(ev.fields) > 0 && <span className="text-emerald-600 dark:text-emerald-400"> (kişisel alanlar gizli)</span>}
                    </li>
                  ))}
                  {Object.keys(finalManifest.entities).length === 0 && <li className="text-muted-foreground">— henüz bir şey açılmadı</li>}
                </ul>
              </div>
              <div className="rounded-lg border border-rose-500/30 bg-rose-500/5 p-3">
                <div className="mb-2 text-xs font-bold uppercase tracking-wide text-rose-700 dark:text-rose-400">✕ Asistan göremez</div>
                <ul className="text-sm text-rose-900/80 dark:text-rose-200/80">
                  {totalMasked > 0 && <li>• Kişilerin maskelenmiş bilgileri (ham hali)</li>}
                  <li>• Açılmayan {Math.max(0, (schema?.tables.length ?? 0) - Object.keys(finalManifest.entities).length)} bilgi grubu</li>
                  <li>• Hiçbir veriyi değiştiremez (salt-okuma)</li>
                </ul>
              </div>
            </div>

            {/* Neler değişecek */}
            <div className="rounded-lg border border-sky-500/30 bg-sky-500/5 px-3 py-2.5 text-sm">
              <div className="mb-1 font-semibold text-sky-700 dark:text-sky-400">Yayınlayınca neler değişecek?</div>
              <div className="text-muted-foreground">
                <span className="font-semibold text-emerald-600 dark:text-emerald-400">+ Eklenecek:</span>{' '}
                {diff.addedEnt.length ? diff.addedEnt.map(humanize).join(', ') : 'yok'}
                {diff.addedTool > 0 && ` · ${diff.addedTool} yeni yetenek`}
                {'  ·  '}
                <span className="font-semibold text-rose-600 dark:text-rose-400">− Kaldırılacak:</span>{' '}
                {diff.removedEnt.length ? diff.removedEnt.map(humanize).join(', ') : 'yok'}
                {diff.removedTool > 0 && ` · ${diff.removedTool} yetenek`}
                <div className="mt-1 text-xs">Bu değişiklik geri alınabilir ve etkinlik kaydına yazılır.</div>
              </div>
            </div>

            {/* Canlı deneme (yayındaki bağlantı — gerçek /try, LLM'siz) */}
            <div className="rounded-lg border p-3">
              <div className="mb-2 text-sm font-semibold">Yayındaki bağlantıyı deneyin <span className="text-muted-foreground text-xs font-normal">(gerçek veri, maskeli)</span></div>
              {liveTools.length === 0 ? (
                <div className="text-muted-foreground text-xs">Henüz yayında yetenek yok — yayına aldıktan sonra burada deneyebilirsiniz.</div>
              ) : (
                <div className="flex flex-col gap-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <select
                      value={previewTool}
                      onChange={(e) => { setPreviewTool(e.target.value); setPreview(null); setPreviewErr(null) }}
                      className="border-input bg-background h-9 rounded-md border px-3 text-sm"
                    >
                      {liveTools.map((t) => {
                        const cap = capability(t.name, humanize(t.name.replace(/^(count|list)_/, '')))
                        return <option key={t.name} value={t.name}>{cap.label}</option>
                      })}
                    </select>
                    {(liveTools.find((t) => t.name === previewTool)?.params || []).filter((p) => p.required).map((p) => (
                      <Input
                        key={p.name}
                        placeholder={p.name}
                        value={previewArgs[p.name] ?? ''}
                        onChange={(e) => setPreviewArgs((s) => ({ ...s, [p.name]: e.target.value }))}
                        className="h-9 w-40"
                      />
                    ))}
                    <Button size="sm" variant="secondary" onClick={runPreview}>Dene</Button>
                  </div>
                  {previewErr && <div className="text-destructive text-sm">{previewErr}</div>}
                  {preview && <PreviewResult res={preview} />}
                </div>
              )}
            </div>

            {published ? (
              <div className="flex items-center justify-between rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-4 py-3">
                <span className="flex items-center gap-2 text-sm font-medium text-emerald-700 dark:text-emerald-400">
                  <CheckCircle2Icon className="size-5" /> {published}
                </span>
                <Button onClick={onExit}>Bitir</Button>
              </div>
            ) : (
              <div className="flex items-center">
                <Button variant="outline" onClick={back}>
                  <ArrowLeftIcon className="size-4" /> Geri
                </Button>
                <Button className="ml-auto bg-emerald-600 hover:bg-emerald-700" onClick={publish} disabled={busy}>
                  {busy ? 'Yayınlanıyor…' : '✓ Yayına al'}
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function NavRow({ onBack, onNext, nextDisabled }: { onBack: () => void; onNext: () => void; nextDisabled?: boolean }) {
  return (
    <div className="flex items-center pt-1">
      <Button variant="outline" onClick={onBack}>
        <ArrowLeftIcon className="size-4" /> Geri
      </Button>
      <Button className="ml-auto" onClick={onNext} disabled={nextDisabled}>
        Devam <ArrowRightIcon className="size-4" />
      </Button>
    </div>
  )
}

function PreviewResult({ res }: { res: RunResult }) {
  if (res.count !== undefined && !res.rows) {
    return <div className="text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{res.count}</div>
  }
  if (res.rows) {
    if (res.rows.length === 0) return <div className="text-muted-foreground text-sm">0 kayıt</div>
    const cols = Object.keys(res.rows[0])
    return (
      <div className="overflow-x-auto rounded-md border">
        <table className="w-full text-xs">
          <thead className="bg-muted/50">
            <tr>{cols.map((c) => <th key={c} className="px-2 py-1 text-left font-medium">{c}</th>)}</tr>
          </thead>
          <tbody>
            {res.rows.slice(0, 5).map((r, i) => (
              <tr key={i} className="border-t">
                {cols.map((c) => <td key={c} className="px-2 py-1 font-mono">{r[c] === null || r[c] === undefined ? '∅' : typeof r[c] === 'object' ? JSON.stringify(r[c]) : String(r[c])}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }
  return <pre className="text-xs">{JSON.stringify(res, null, 2)}</pre>
}
