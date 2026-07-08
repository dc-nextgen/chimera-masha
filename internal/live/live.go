// Package live — calisan manifest'in TEK atomik kaynagi. Onboarding "apply" bir kez Set()
// eder → connector (Call), toolserver (allowlist + openapi) ve webui (durum) AYNI ANDA yeni
// manifest'i gorur. Hot-reload (restart'siz yeniden-yapilandirma) icin.
//
// atomic.Pointer → kilitsiz okuma (her arac cagrisi Get()), nadir yazma (apply).
package live

import (
	"sync/atomic"

	"github.com/dc-nextgen/chimera-masha/internal/manifest"
)

type Manifest struct {
	p atomic.Pointer[manifest.Manifest]
}

// New — baslangic manifest'i ile (nil olabilir: introspect yolu manifest istemez).
func New(m *manifest.Manifest) *Manifest {
	l := &Manifest{}
	if m != nil {
		l.p.Store(m)
	}
	return l
}

// Get — yururlukteki manifest (nil olabilir).
func (l *Manifest) Get() *manifest.Manifest { return l.p.Load() }

// Set — yeni manifest'i atomik yayinla (apply). Cagirandan ONCE Validate() beklenir.
func (l *Manifest) Set(m *manifest.Manifest) { l.p.Store(m) }
