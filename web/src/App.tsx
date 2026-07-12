import { useEffect, useState } from 'react'
import { ChevronLeftIcon } from 'lucide-react'

import { AppSidebar, type View } from '@/components/app-sidebar'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { Login } from '@/components/Login'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { SidebarInset, SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar'
import { TooltipProvider } from '@/components/ui/tooltip'
import { api, getAuthToken, type ConnInfo, type Health } from '@/lib/api'
import { Ayarlar } from '@/pages/Ayarlar'
import { Baglantilar } from '@/pages/Baglantilar'
import { ConnectionDetail } from '@/pages/ConnectionDetail'
import { Durum } from '@/pages/Durum'
import { Plan } from '@/pages/Plan'
import { Wizard } from '@/pages/Wizard'

// initialView — tray menüsü "Ayarlar" /ayarlar path'ini açar (SPA state-tabanlı, react-router yok).
// İlk yüklemede path'e bak; başka her şeyde varsayılan davranış aynı kalır (durum).
function initialView(): View {
  if (/^\/ayarlar\/?$/.test(window.location.pathname)) return 'ayarlar'
  return 'durum'
}

export default function App() {
  const [view, setView] = useState<View>(initialView)
  const [health, setHealth] = useState<Health | null>(null)
  const [authRequired, setAuthRequired] = useState<boolean | null>(null)
  const [authed, setAuthed] = useState(false)
  const [conns, setConns] = useState<ConnInfo[]>([])
  // Master-detail + sihirbaz: seçili bağlantı = server_label (conns'tan türetilir → durum tazelenince güncel).
  const [detailLabel, setDetailLabel] = useState<string | null>(null)
  const [wizardLabel, setWizardLabel] = useState<string | null>(null)

  const refreshHealth = () => api.health().then(setHealth).catch(() => {})
  const refreshConns = () =>
    api
      .connections()
      .then((r) => setConns(r.connections || []))
      .catch(() => {})

  useEffect(() => {
    api
      .authStatus()
      .then((s) => {
        setAuthRequired(s.auth_required)
        if (!s.auth_required || getAuthToken()) setAuthed(true)
      })
      .catch(() => setAuthRequired(false))
  }, [])

  useEffect(() => {
    if (!authed) return
    refreshConns()
    let live = true
    const tick = () =>
      api
        .health()
        .then((h) => live && setHealth(h))
        .catch((e) => {
          if (String(e).includes('401')) setAuthed(false)
        })
    tick()
    const id = setInterval(tick, 10_000)
    return () => {
      live = false
      clearInterval(id)
    }
  }, [authed])

  if (authRequired === null) return null
  if (authRequired && !authed) return <Login onSuccess={() => setAuthed(true)} />

  const detailConn = detailLabel ? conns.find((c) => c.server_label === detailLabel) : undefined
  const wizardConn = wizardLabel ? conns.find((c) => c.server_label === wizardLabel) : undefined
  const nav = (v: View) => {
    setView(v)
    setDetailLabel(null)
    setWizardLabel(null)
  }
  const onChanged = () => {
    refreshConns()
    refreshHealth()
  }

  return (
    <TooltipProvider delayDuration={0}>
      <SidebarProvider>
        <AppSidebar view={view} onNavigate={nav} health={health} />
        <SidebarInset>
          <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-2 h-4" />
            {wizardConn ? (
              <>
                <Button variant="ghost" size="sm" className="-ml-2 h-7 gap-1 px-2" onClick={() => setWizardLabel(null)}>
                  <ChevronLeftIcon className="size-4" />
                  {wizardConn.label || wizardConn.name}
                </Button>
                <span className="text-muted-foreground">/</span>
                <h1 className="text-sm font-medium">MCP kurulum sihirbazı</h1>
              </>
            ) : view === 'baglantilar' && detailConn ? (
              <>
                <Button variant="ghost" size="sm" className="-ml-2 h-7 gap-1 px-2" onClick={() => setDetailLabel(null)}>
                  <ChevronLeftIcon className="size-4" />
                  Bağlantılar
                </Button>
                <span className="text-muted-foreground">/</span>
                <h1 className="text-sm font-medium">{detailConn.label || detailConn.name}</h1>
              </>
            ) : (
              <h1 className="text-sm font-medium">
                {view === 'durum' ? 'Durum' : view === 'plan' ? 'Plan' : view === 'ayarlar' ? 'Ayarlar' : 'Bağlantılar'}
              </h1>
            )}
            <div className="ml-auto">
              {health && (
                <Badge variant={health.db ? 'default' : 'destructive'}>
                  {health.db ? 'agent · DB bağlı' : 'agent · DB yok'}
                </Badge>
              )}
            </div>
          </header>
          {health?.tunnel === 'conflict' && (
            <div className="flex items-start gap-3 border-b border-rose-500/40 bg-rose-500/10 px-4 py-3 text-sm text-rose-800 dark:text-rose-300">
              <span aria-hidden>⚠</span>
              <span>
                <b>Tünel çatışması:</b> {health.tunnel_msg || 'Bu bağlantı slotu başka bir makinede aktif.'}
              </span>
            </div>
          )}
          <main className="flex-1 p-4 md:p-6">
            <ErrorBoundary>
              {wizardConn ? (
                <Wizard conn={wizardConn} onExit={() => setWizardLabel(null)} onChanged={onChanged} />
              ) : (
                <>
                  {view === 'durum' && <Durum health={health} conns={conns} onSelect={(c) => { setView('baglantilar'); setDetailLabel(c.server_label) }} />}
                  {view === 'plan' && <Plan />}
                  {view === 'ayarlar' && <Ayarlar />}
                  {view === 'baglantilar' &&
                    (detailConn ? (
                      <ConnectionDetail
                        conn={detailConn}
                        onChanged={onChanged}
                        onOpenWizard={() => setWizardLabel(detailConn.server_label)}
                      />
                    ) : (
                      <Baglantilar conns={conns} onSelect={(c) => setDetailLabel(c.server_label)} onRefresh={refreshConns} />
                    ))}
                </>
              )}
            </ErrorBoundary>
          </main>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}
