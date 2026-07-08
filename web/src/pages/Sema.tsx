import { useState } from 'react'
import { ChevronRightIcon } from 'lucide-react'

import { api, type Schema } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'

export function Sema({ conn }: { conn?: string }) {
  const [schema, setSchema] = useState<Schema | null>(null)
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  const load = async () => {
    setLoading(true)
    setErr(null)
    try {
      setSchema(await api.schema(conn))
    } catch (e) {
      setErr(String(e))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Button onClick={load} disabled={loading}>
          {loading ? 'Yükleniyor…' : 'Şemayı tara'}
        </Button>
        {schema && <span className="text-muted-foreground text-sm">{schema.tables.length} kaynak</span>}
      </div>
      <p className="text-muted-foreground text-xs">
        İntrospeksiyon — yalnız yapı (tablo/kolon/tip), satır okumaz. Bu çıktı onboarding'de buluta gider
        (eşleme önerisi); veri gitmez.
      </p>

      {err && <div className="text-destructive text-sm">{err}</div>}

      <div className="space-y-1.5">
        {schema?.tables.map((t) => (
          <Collapsible key={`${t.schema}.${t.name}`} className="rounded-md border">
            <CollapsibleTrigger className="hover:bg-muted/50 flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm [&[data-state=open]>svg]:rotate-90">
              <ChevronRightIcon className="text-muted-foreground size-4 transition-transform" />
              <span className="font-mono">
                {t.schema}.{t.name}
              </span>
              <span className="text-muted-foreground text-xs">({t.columns.length} kolon)</span>
            </CollapsibleTrigger>
            <CollapsibleContent className="flex flex-wrap gap-1.5 px-3 pb-3 pt-1">
              {t.columns.map((c) => (
                <Badge key={c.name} variant="secondary" className="font-normal">
                  {c.name}
                  <span className="text-muted-foreground ml-1">{c.type}</span>
                </Badge>
              ))}
            </CollapsibleContent>
          </Collapsible>
        ))}
      </div>
    </div>
  )
}
