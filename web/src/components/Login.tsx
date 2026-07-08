// Yerel yüz giriş ekranı (§17.9 — LAN'a açılınca /try + /onboard/* korumalı). Parola → oturum token'ı.
import { useState } from 'react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'

export function Login({ onSuccess }: { onSuccess: () => void }) {
  const [pw, setPw] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setErr(null)
    try {
      await api.login(pw)
      onSuccess()
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ''))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-base">Masha — Giriş</CardTitle>
          <p className="text-muted-foreground text-sm">Yerel yönetim yüzü parolayla korunuyor.</p>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="flex flex-col gap-3">
            <Input
              type="password"
              placeholder="parola"
              value={pw}
              onChange={(e) => setPw(e.target.value)}
              autoFocus
            />
            {err && <span className="text-destructive text-sm">{err}</span>}
            <Button type="submit" disabled={busy || !pw}>
              {busy ? 'Giriş yapılıyor…' : 'Giriş'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
