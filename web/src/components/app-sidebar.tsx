import { DatabaseIcon, GaugeIcon, PlugIcon, RocketIcon } from 'lucide-react'

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from '@/components/ui/sidebar'
import type { Health } from '@/lib/api'

// Master-detail (§19.2): sidebar sade+stabil. Kur/Araçlar/Şema = bağlantı DETAYINDA sekme.
export type View = 'durum' | 'baglantilar' | 'plan'

const items: { key: View; title: string; icon: typeof GaugeIcon }[] = [
  { key: 'durum', title: 'Durum', icon: GaugeIcon },
  { key: 'baglantilar', title: 'Bağlantılar', icon: PlugIcon },
  { key: 'plan', title: 'Plan', icon: RocketIcon },
]

export function AppSidebar({
  view,
  onNavigate,
  health,
}: {
  view: View
  onNavigate: (v: View) => void
  health: Health | null
}) {
  // Menü seçince sidebar'ı kapat YALNIZ mobilde (sheet, içeriğin üstünde overlay). Masaüstü inset =
  // içeriğin YANINDA, üstünde değil → menü seçince kapatma (kullanıcı geri-bildirimi 2026-07-06).
  const { setOpenMobile, isMobile } = useSidebar()
  const go = (v: View) => {
    onNavigate(v)
    if (isMobile) setOpenMobile(false)
  }
  return (
    <Sidebar variant="inset">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" className="pointer-events-none">
              <div className="bg-sidebar-primary text-sidebar-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg">
                <DatabaseIcon className="size-4" />
              </div>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-semibold">Masha</span>
                <span className="text-muted-foreground truncate text-xs">DB connector</span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {items.map((it) => (
                <SidebarMenuItem key={it.key}>
                  <SidebarMenuButton
                    isActive={view === it.key}
                    tooltip={it.title}
                    onClick={() => go(it.key)}
                  >
                    <it.icon />
                    <span>{it.title}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter>
        <div className="text-muted-foreground px-2 py-1 text-xs">
          {health ? (
            <>
              <div className="truncate">
                {health.connector} · {health.tools.length} araç
              </div>
              {health.audit_head && (
                <div className="truncate">audit: {health.audit_head.slice(0, 10)}</div>
              )}
            </>
          ) : (
            <span>bağlanıyor…</span>
          )}
        </div>
      </SidebarFooter>
    </Sidebar>
  )
}
