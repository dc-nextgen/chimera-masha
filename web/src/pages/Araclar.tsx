import { useEffect, useState } from 'react'

import { api, type RunResult, type Tool } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

function fmt(v: unknown): string {
  if (v === null || v === undefined) return '∅'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

function Result({ res }: { res: RunResult }) {
  if (res.count !== undefined && !res.rows) {
    return <div className="text-3xl font-semibold text-emerald-600">{res.count}</div>
  }
  if (res.rows) {
    if (res.rows.length === 0) return <div className="text-muted-foreground text-sm">0 kayıt</div>
    const cols = Object.keys(res.rows[0])
    return (
      <div>
        <div className="text-muted-foreground mb-1 text-xs">
          {res.count} kayıt (ilk {res.rows.length})
        </div>
        <div className="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                {cols.map((c) => (
                  <TableHead key={c}>{c}</TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {res.rows.map((r, i) => (
                <TableRow key={i}>
                  {cols.map((c) => (
                    <TableCell key={c} className="font-mono text-xs">
                      {fmt(r[c])}
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </div>
    )
  }
  return <pre className="text-xs">{JSON.stringify(res, null, 2)}</pre>
}

function ToolCard({ tool, conn }: { tool: Tool; conn?: string }) {
  const [args, setArgs] = useState<Record<string, string>>({})
  const [res, setRes] = useState<RunResult | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const run = async () => {
    setLoading(true)
    setErr(null)
    setRes(null)
    const a: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(args)) if (v.trim()) a[k] = v.trim()
    try {
      setRes(await api.run(tool.name, a, conn))
    } catch (e) {
      setErr(String(e))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center gap-2">
          <CardTitle className="font-mono text-base">{tool.name}</CardTitle>
          <Button size="sm" className="ml-auto" onClick={run} disabled={loading}>
            {loading ? '…' : 'Çalıştır'}
          </Button>
        </div>
        <CardDescription>{tool.description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {tool.params && tool.params.length > 0 && (
          <div className="flex flex-wrap gap-3">
            {tool.params.map((p) => (
              <div key={p.name} className="grid gap-1">
                <Label className="text-muted-foreground text-xs">
                  {p.name}
                  {p.required ? ' *' : ''}
                </Label>
                <Input
                  className="h-8 w-44"
                  placeholder={p.type === 'object' ? 'JSON {…}' : p.type}
                  value={args[p.name] ?? ''}
                  onChange={(e) => setArgs((s) => ({ ...s, [p.name]: e.target.value }))}
                />
              </div>
            ))}
          </div>
        )}
        {err && <div className="text-destructive text-sm">{err}</div>}
        {res && <Result res={res} />}
      </CardContent>
    </Card>
  )
}

export function Araclar({ conn }: { conn?: string }) {
  const [tools, setTools] = useState<Tool[]>([])
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    api
      .tools(conn)
      .then((r) => setTools(r.tools))
      .catch((e) => setErr(String(e)))
  }, [conn])

  if (err) return <div className="text-destructive text-sm">{err}</div>
  if (tools.length === 0)
    return <div className="text-muted-foreground text-sm">Bu bağlantıda araç yok.</div>
  return (
    <div className="grid gap-4">
      {tools.map((t) => (
        <ToolCard key={t.name} tool={t} conn={conn} />
      ))}
    </div>
  )
}
