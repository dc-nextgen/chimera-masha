package documents

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// debounceWindow — bir degisiklik burstunu TEK SyncOnce'a coalesce etmek icin sessizlik penceresi.
const debounceWindow = 2 * time.Second

// Watcher — Dir'i (ve tum alt-dizinlerini) fsnotify ile izler; degisiklik burstlarini debounce
// edip Syncer.SyncOnce cagirir. fsnotify OZYINELI DEGILDIR — bu yuzden her (alt)dizini elle Add
// eder, yeni olusturulan dizinleri de calisirken ekler.
type Watcher struct {
	Syncer *Syncer

	// Logf — opsiyonel; verilmezse log.Printf.
	Logf func(format string, args ...any)
}

func (w *Watcher) logf(format string, args ...any) {
	if w.Logf != nil {
		w.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Run — baslangicta bir SyncOnce yapar, sonra ctx iptal edilene kadar izlemeye devam eder.
// Izleme hatalari FATAL DEGILDIR — loglanir, watcher calismaya devam eder.
func (w *Watcher) Run(ctx context.Context) error {
	if _, err := w.Syncer.SyncOnce(ctx); err != nil {
		w.logf("documents: ilk senkron hata: %v", err)
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fw.Close()

	if err := addRecursive(fw, w.Syncer.Dir); err != nil {
		return err
	}

	var timer *time.Timer
	debounced := make(chan struct{}, 1)

	resetTimer := func() {
		if timer == nil {
			timer = time.AfterFunc(debounceWindow, func() {
				select {
				case debounced <- struct{}{}:
				default:
				}
			})
			return
		}
		timer.Reset(debounceWindow)
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case ev, ok := <-fw.Events:
			if !ok {
				return nil
			}
			// yeni olusturulan dizinler de izlenmeli (fsnotify ozyineli degil).
			if ev.Op&fsnotify.Create != 0 {
				if info, statErr := os.Stat(ev.Name); statErr == nil && info.IsDir() {
					if err := addRecursive(fw, ev.Name); err != nil {
						w.logf("documents: alt-dizin izlenemedi (%s): %v", ev.Name, err)
					}
				}
			}
			resetTimer()

		case err, ok := <-fw.Errors:
			if !ok {
				return nil
			}
			w.logf("documents: izleme hatasi: %v", err) // non-fatal, izlemeye devam.

		case <-debounced:
			if _, err := w.Syncer.SyncOnce(ctx); err != nil {
				w.logf("documents: senkron hata: %v", err)
			}
		}
	}
}

func addRecursive(fw *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // erisilemeyen bir alt-dizin tum izlemeyi durdurmasin.
		}
		if d.IsDir() {
			if addErr := fw.Add(path); addErr != nil {
				log.Printf("documents: dizin izlenemedi (%s): %v", path, addErr)
			}
		}
		return nil
	})
}
