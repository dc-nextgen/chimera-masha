// Masha agent yerel API istemcisi. Uretimde ayni origin (Go binary dist'i sunar + API),
// dev'de Vite proxy → 127.0.0.1:8787 (vite.config.ts).

export type Health = {
  ok: boolean
  connector: string
  erp_kind: string
  db: boolean
  tools: string[]
  audit_head: string
  tunnel?: string // "off" | "connecting" | "connected" | "conflict" (frpc sidecar catisma tespiti)
  tunnel_msg?: string
}

export type Filter = { name: string; field: string; op: string; required?: boolean }

// Araçlar ekranı için birleşik araç metadata (OpenAPI'den; mssql+erpnext aynı şekil).
export type ToolParam = { name: string; type: string; required: boolean }
export type Tool = { name: string; description: string; params: ToolParam[] }
export type ToolsResp = { connector: string; label: string; kind: string; tools: Tool[] }

export type Column = { name: string; type: string; nullable: boolean }
export type Schema = { tables: { schema: string; name: string; columns: Column[] }[] }

export type RunResult = { count?: number; rows?: Record<string, unknown>[] }

// ── Onboarding (Faz 2): şema → aday manifest → operatör düzenle → uygula ──
export type MField = { column: string; expression?: string }
export type MEntity = { table: string; fields: Record<string, MField> }
export type MTool = {
  name: string
  kind: string
  entity: string
  description: string
  select?: string[]
  filters?: Filter[]
  limit?: number
}
export type Manifest = {
  name: string
  label: string
  erp_kind: string
  prompt?: string
  db: { driver: string; read_only: boolean }
  entities: Record<string, MEntity>
  tools: MTool[]
}
export type Selection = { name?: string; label?: string; tables?: string[] }
export type ApplyResult = { ok: boolean; connector: string; tools: string[] }

// DB bağlantısı (ekrandan). Kimlik YERELDE kalır — buluta gitmez (§3).
export type DBFields = {
  host: string
  port?: string
  database: string
  user: string
  password: string
  encrypt?: string
  trust_server_cert?: boolean
}
export type DBStatus = { connected: boolean; can_connect: boolean }

// ErpNext bağlantısı (ekrandan). Kimlik YERELDE kalır (§3).
export type ErpFields = { url: string; api_key: string; api_secret: string }

// Çok-bağlantı (§19.2): kayıtlı bağlantılar (Auroville DB + erpnext + …).
export type ConnInfo = {
  name: string
  label: string
  kind: string
  server_label: string
  connected: boolean
}

// Denetim kaydı (Etkinlik akışı + "son test"). Esnek şekil — kayıt keyleri sabit değil
// (decision/source/server/tool/err/host/database/url/connector…). ts + hash her kayıtta.
export type AuditRecord = {
  ts?: string
  decision?: string
  source?: string
  server?: string
  tool?: string
  err?: string
  hash?: string
  [k: string]: unknown
}

// LLM danışman (Faz 2.4b): şema→tablo sınıflandırma + hassas-flag + PII/isim önerisi.
export type TableAdvice = {
  table: string
  entity: string
  kind: string // business | user | permission | audit | lookup | other
  sensitive: boolean
  note: string
  fields?: { column: string; expression: string }[]
}
export type AdviseResult = { tables: TableAdvice[] }

// Plan / deneme (satış yüzeyi) — Plan/Yükselt ekranı. Sır YOK; deneme durumu + talep kanalı.
export type PlanInfo = {
  plan: string // "" (normal) | "trial"
  trial_limit_usd: string
  contact_email: string
  request_url: string
}

// desteklenen expression seçenekleri (expression.Apply ile birebir).
export const EXPRESSIONS = [
  '',
  'mask:email',
  'mask:phone',
  'mask:tckn',
  'mask:iban',
  'mask:card',
  'format:date',
  'format:datetime',
  'format:money',
] as const

// Yerel yüz auth (opsiyonel): login token'ı sessionStorage'da; her gated isteğe Bearer eklenir.
// TLS yok → güvenilir LAN varsayımı (§17.9). 401 → token temizlenir (App login'e döner).
let authToken = sessionStorage.getItem('masha_tok') || ''
export function setAuthToken(t: string) {
  authToken = t
  sessionStorage.setItem('masha_tok', t)
}
export function clearAuthToken() {
  authToken = ''
  sessionStorage.removeItem('masha_tok')
}
export function getAuthToken(): string {
  return authToken
}

// ?conn=<label> ekle (per-connection op scoping); boşsa primary.
function q(conn?: string): string {
  return conn ? `?conn=${encodeURIComponent(conn)}` : ''
}

async function j<T>(url: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    ...((init?.headers as Record<string, string>) || {}),
    ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
  }
  const r = await fetch(url, { ...init, headers })
  if (r.status === 401) {
    clearAuthToken()
    throw new Error('401 kimlik gerekli')
  }
  const d = await r.json().catch(() => ({}))
  if (!r.ok) throw new Error((d as { error?: string }).error || `HTTP ${r.status}`)
  return d as T
}

export const api = {
  health: () => j<Health>('/healthz'),
  tools: (conn?: string) => j<ToolsResp>('/tools' + q(conn)),
  schema: (conn?: string) => j<Schema>('/schema' + q(conn)),
  run: (tool: string, args: Record<string, unknown>, conn?: string) =>
    j<RunResult>('/try' + q(conn), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tool, args }),
    }),
  suggest: (sel: Selection) =>
    j<Manifest>('/onboard/suggest', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(sel),
    }),
  apply: (m: Manifest) =>
    j<ApplyResult>('/onboard/apply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(m),
    }),
  advise: (sel: Selection) =>
    j<AdviseResult>('/onboard/advise', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(sel),
    }),
  connections: () => j<{ connections: ConnInfo[] }>('/connections'),
  dbStatus: () => j<DBStatus>('/db/status'),
  dbConnect: (f: DBFields) =>
    j<{ ok: boolean; connected: boolean }>('/db/connect', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(f),
    }),
  erpnextConnect: (f: ErpFields) =>
    j<{ ok: boolean; connected: boolean }>('/erpnext/connect', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(f),
    }),
  exportManifest: () => j<Manifest>('/onboard/export'),
  auditLog: (conn?: string, limit = 50) => {
    const p = new URLSearchParams()
    if (conn) p.set('conn', conn)
    p.set('limit', String(limit))
    return j<{ records: AuditRecord[] }>('/audit/log?' + p.toString())
  },
  plan: () => j<PlanInfo>('/plan'),
  authStatus: () => j<{ auth_required: boolean }>('/auth/status'),
  login: async (password: string): Promise<{ token: string }> => {
    // login açık uç: 401=parola yanlış (token temizleme mantığına sokma → net hata mesajı).
    const r = await fetch('/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password }),
    })
    const d = (await r.json().catch(() => ({}))) as { token?: string; error?: string }
    if (!r.ok || !d.token) throw new Error(d.error || 'giriş başarısız')
    setAuthToken(d.token)
    return { token: d.token }
  },
}
