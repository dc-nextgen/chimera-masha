// Bağlantılar (master) — kaynak listesi (tıkla → detay) + yeni bağlantı ekle.
// Kimlik YERELDE kalır (§3). Sohbet DB/ERP'ye YALNIZ bu ajan üzerinden erer. conns = App tek-kaynak.
import { AddConnection } from '@/components/ConnectForms'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { ConnInfo } from '@/lib/api'

const kindLabel: Record<string, string> = { mssql: 'SQL Server', erpnext: 'ErpNext' }

export function Baglantilar({
  conns,
  onSelect,
  onRefresh,
}: {
  conns: ConnInfo[]
  onSelect: (c: ConnInfo) => void
  onRefresh: () => void
}) {
  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-4">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Kaynaklar</CardTitle>
          <p className="text-muted-foreground text-sm">
            Bir kaynağa tıkla → asistanın ne görebileceğini kur, araçları dene, etkinliği izle. Sohbet bu
            kaynaklara yalnız bu ajan üzerinden erer.
          </p>
        </CardHeader>
        <CardContent className="p-0">
          {conns.length === 0 ? (
            <div className="text-muted-foreground p-4 text-sm">henüz bağlantı yok — aşağıdan ekle</div>
          ) : (
            conns.map((c) => (
              <button
                key={c.server_label}
                onClick={() => onSelect(c)}
                className="hover:bg-muted/50 flex w-full items-center gap-3 border-b px-4 py-3 text-left text-sm last:border-b-0"
              >
                <span className="font-medium">{c.label || c.name}</span>
                <Badge variant="outline" className="text-xs">
                  {kindLabel[c.kind] ?? c.kind}
                </Badge>
                <span className="text-muted-foreground font-mono text-xs">{c.server_label}</span>
                <Badge variant={c.connected ? 'default' : 'destructive'} className="ml-auto">
                  {c.connected ? 'bağlı' : 'bağlı değil'}
                </Badge>
                <span className="text-muted-foreground">›</span>
              </button>
            ))
          )}
        </CardContent>
      </Card>

      <AddConnection onDone={onRefresh} />
    </div>
  )
}
