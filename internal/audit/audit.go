// Package audit — hash-ZINCIRLI denetim kaydi (tamper-evident).
//
// telemetry.py hash-zincir mantiginin Go portu. Her kayit
// prev-hash'e baglanir: hash = sha256(prev_hash + canonical_json(rec)). Zincir kirilirsa
// (silme/degistirme) sonraki dogrulama fark eder. Faz 1 = yerel dosya + stdout; off-box
// akitma (collector) Faz 2.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// recentCap — yerel yuz "Etkinlik" akisi icin bellekte tutulan son kayit sayisi (ring).
// Denetim-KAYDI dosyada tam (hash-zincir); bu yalniz UI gosterimi icin ucuz bir pencere.
const recentCap = 200

type Auditor struct {
	mu      sync.Mutex
	prev    string
	file    *os.File
	nowFn   func() time.Time // test icin enjekte edilebilir
	genesis string
	recent  []map[string]any // son kayitlar (eskiden yeniye; recentCap ile sinirli)
}

// New — audit dosyasini acar (append). path bos => yalniz stdout.
func New(path string) (*Auditor, error) {
	a := &Auditor{prev: "genesis", genesis: "genesis", nowFn: time.Now}
	if path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, err
		}
		a.file = f
	}
	return a, nil
}

// Record bir kaydi zincire ekler. rec keyleri sabit degil (esnek); ts + hash eklenir.
func (a *Auditor) Record(rec map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry := map[string]any{}
	for k, v := range rec {
		entry[k] = v
	}
	entry["ts"] = a.nowFn().UTC().Format(time.RFC3339)
	entry["prev"] = a.prev

	// canonical: json.Marshal alfabetik key sirasi verir => deterministik.
	payload, _ := json.Marshal(entry)
	sum := sha256.Sum256(append([]byte(a.prev), payload...))
	h := hex.EncodeToString(sum[:])
	entry["hash"] = h
	a.prev = h

	// UI penceresi: son kaydi ring'e ekle (kopya degil — entry artik degismez).
	a.recent = append(a.recent, entry)
	if len(a.recent) > recentCap {
		a.recent = a.recent[len(a.recent)-recentCap:]
	}

	line, _ := json.Marshal(entry)
	line = append(line, '\n')
	os.Stdout.Write(line)
	if a.file != nil {
		a.file.Write(line)
	}
}

// Head — zincirin son hash'i (canlilik/entegrasyon gostergesi icin).
func (a *Auditor) Head() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.prev
}

// Recent — son kayitlar YENIDEN ESKIYE (limit<=0 => hepsi, recentCap ile sinirli). server bos
// degilse yalniz o server_label kayitlari (per-connection Etkinlik). Yerel yuz gosterimi icin.
func (a *Auditor) Recent(limit int, server string) []map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]map[string]any, 0, len(a.recent))
	for i := len(a.recent) - 1; i >= 0; i-- {
		e := a.recent[i]
		if server != "" {
			if s, _ := e["server"].(string); s != server {
				continue
			}
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (a *Auditor) Close() error {
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}
