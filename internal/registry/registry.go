// Package registry — CANLI bağlantı kaydı (§19.2 çok-bağlantı). Ajan tek connector değil, birden
// çok bağlantı tutar (Auroville DB + erpnext + …); tool-server `server_label`'a göre yönlendirir
// (tek tünel çok label taşır — yol zaten [server_label, tool]). Ekle/kaldır thread-safe (atomik yayın).
package registry

import (
	"sort"
	"sync"

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/connector"
)

// Connection — bir bağlantı: ad/etiket + kind (mssql|erpnext|…) + server_label (yol/OWUI tool_id) + backend.
type Connection struct {
	Name        string
	Label       string
	Kind        string
	ServerLabel string // tool-server yol segmenti; OWUI tool_id = "server:<ServerLabel>"
	Conn        connector.Connector
}

type Registry struct {
	mu    sync.RWMutex
	conns map[string]*Connection // key = ServerLabel
}

func New() *Registry {
	return &Registry{conns: map[string]*Connection{}}
}

// Add — bağlantıyı ekle/güncelle (ServerLabel anahtar).
func (r *Registry) Add(c *Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conns[c.ServerLabel] = c
}

// Remove — bağlantıyı kaldır (backend'i KAPATMAZ; çağıran Close eder).
func (r *Registry) Remove(serverLabel string) *Connection {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := r.conns[serverLabel]
	delete(r.conns, serverLabel)
	return c
}

// Get — ServerLabel'a göre bağlantı (yoksa nil).
func (r *Registry) Get(serverLabel string) *Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conns[serverLabel]
}

// List — bağlantılar (ServerLabel'a göre deterministik sıralı).
func (r *Registry) List() []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Connection, 0, len(r.conns))
	for _, c := range r.conns {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ServerLabel < out[j].ServerLabel })
	return out
}
