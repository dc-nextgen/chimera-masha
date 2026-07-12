// Ayarlar — GERÇEK çalışma-zamanı config, salt-okur (§4 dürüstlük). Çoğu alan yalnız env/kurulumla
// değişir; sahte-düzenlenebilir toggle YOK — sessizce kalıcılaşmayan bir anahtar sunmak yerine
// küçük bir notla "kurulum/env ile değişir, yeniden başlatma gerekir" belirtiyoruz.
import { useEffect, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { api, type Settings } from '@/lib/api'

const ENV_NOTE = 'kurulum/env ile değişir, yeniden başlatma gerekir'

function EnvNote() {
  return <p className="text-muted-foreground text-xs">{ENV_NOTE}</p>
}

function tunnelBadge(state?: string, msg?: string) {
  switch (state) {
    case 'connected':
      return <Badge className="border-emerald-500/40 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">bağlı</Badge>
    case 'connecting':
      return <Badge variant="outline">bağlanıyor…</Badge>
    case 'conflict':
      return <Badge variant="destructive">çakışma{msg ? `: ${msg}` : ''}</Badge>
    case 'off':
      return <Badge variant="outline">kapalı</Badge>
    default:
      return <Badge variant="outline">{state || 'bilinmiyor'}</Badge>
  }
}

export function Ayarlar() {
  const [s, setSettings] = useState<Settings | null>(null)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    api
      .settings()
      .then((r) => {
        setSettings(r)
        setErr(null)
      })
      .catch((e) => setErr(String(e)))
  }, [])

  if (err) {
    return <div className="text-destructive mx-auto w-full max-w-2xl text-sm">Ayarlar yüklenemedi: {err}</div>
  }
  if (!s) {
    return <div className="text-muted-foreground mx-auto w-full max-w-2xl text-sm">Yükleniyor…</div>
  }

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
      <div>
        <h2 className="text-xl font-semibold">Ayarlar</h2>
        <p className="text-muted-foreground text-sm">
          Ajanın şu anki çalışma-zamanı ayarları — gerçek değerler. Buradan doğrudan değiştirilmez;
          çoğu ayar kurulum/env üzerinden yönetilir.
        </p>
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Sürüm</CardTitle>
        </CardHeader>
        <CardContent>
          <span className="font-mono text-sm">{s.version}</span>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Yerel yüz</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2 text-sm">
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground">Adres</span>
            <span className="font-mono">{s.web_addr}</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground">TLS</span>
            <Badge variant={s.web_tls ? 'default' : 'outline'}>{s.web_tls ? 'Açık' : 'Kapalı'}</Badge>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground">Kimlik doğrulama</span>
            <Badge variant={s.auth_enabled ? 'default' : 'outline'}>
              {s.auth_enabled ? 'Parola korumalı' : 'Korumasız'}
            </Badge>
          </div>
          <EnvNote />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Tünel</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2 text-sm">
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground">Mod</span>
            <span>{s.tunnel_mode === 'embed' ? 'Süreç-içi (embed)' : 'Sidecar'}</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground">Durum</span>
            {tunnelBadge(s.tunnel_state, s.tunnel_msg)}
          </div>
          <EnvNote />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Kimlik saklama</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2 text-sm">
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground">Depo</span>
            <span>{s.cred_store}</span>
          </div>
          <p className="text-muted-foreground text-xs">
            Bağlantı kimlikleri (DB/ErpNext parola-anahtarı) bu kutuda yerel olarak saklanır — buluta gönderilmez (§3).
          </p>
          <EnvNote />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Gizlilik / maskeleme</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2 text-sm">
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground">ErpNext PII maske</span>
            <Badge variant={s.erpnext_mask ? 'default' : 'outline'}>{s.erpnext_mask ? 'Açık' : 'Kapalı'}</Badge>
          </div>
          <EnvNote />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">ErpNext yazma</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2 text-sm">
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground">Yazma yüzeyi</span>
            <Badge variant={s.erpnext_write ? 'default' : 'outline'}>{s.erpnext_write ? 'Açık' : 'Salt-okunur'}</Badge>
          </div>
          {s.erpnext_write && (
            <div className="flex flex-wrap items-center gap-1">
              <span className="text-muted-foreground">İzinli doctype</span>
              {(s.erpnext_write_doctypes ?? []).length > 0 ? (
                (s.erpnext_write_doctypes ?? []).map((d) => (
                  <Badge key={d} variant="secondary">{d}</Badge>
                ))
              ) : (
                <Badge variant="outline">yok (hiçbir şey yazılamaz)</Badge>
              )}
            </div>
          )}
          <p className="text-muted-foreground text-xs">
            Yazma yalnız insan-onaylı akıştan (Telegram/onay) çağrılır; sohbet asistanı doğrudan yazamaz.
            Yalnız kayıt <em>oluşturma</em> (taslak); güncelleme/onaylama/silme yok.
          </p>
          <EnvNote />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">LLM danışman</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2 text-sm">
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground">Onboarding danışmanı</span>
            <Badge variant={s.llm_enabled ? 'default' : 'outline'}>{s.llm_enabled ? 'Açık' : 'Kapalı'}</Badge>
          </div>
          <EnvNote />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Plan</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center gap-2 text-sm">
          <span className="text-muted-foreground">Sürüm</span>
          <Badge variant={s.plan === 'trial' ? 'default' : 'outline'}>{s.plan === 'trial' ? 'Deneme' : 'Normal'}</Badge>
        </CardContent>
      </Card>
    </div>
  )
}
