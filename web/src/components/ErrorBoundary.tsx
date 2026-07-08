// ErrorBoundary — bir bileşen çökerse BEYAZ EKRAN yerine dostça mesaj (kullanıcı: "hata almamalı").
// Deneme/ücretsiz sürümde bir hata tüm uygulamayı düşürmesin; kullanıcı "Tekrar dene" ile toparlasın.
import { Component, type ErrorInfo, type ReactNode } from 'react'

import { Button } from '@/components/ui/button'

export class ErrorBoundary extends Component<{ children: ReactNode }, { err: Error | null }> {
  state = { err: null as Error | null }

  static getDerivedStateFromError(err: Error) {
    return { err }
  }

  componentDidCatch(err: Error, info: ErrorInfo) {
    // Yerel yüz — konsola yaz (operatör inceler); kullanıcıya teknik ayrıntı gösterme.
    console.error('Masha UI hata:', err, info.componentStack)
  }

  render() {
    if (this.state.err) {
      return (
        <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 p-6 text-center">
          <div className="text-lg font-semibold">Bir şeyler ters gitti</div>
          <p className="text-muted-foreground max-w-md text-sm">
            Bu ekran yüklenirken bir sorun oluştu. Verileriniz güvende — hiçbir şey değişmedi. Tekrar
            deneyebilir ya da başka bir bölüme geçebilirsiniz.
          </p>
          <Button onClick={() => this.setState({ err: null })}>Tekrar dene</Button>
        </div>
      )
    }
    return this.props.children
  }
}
