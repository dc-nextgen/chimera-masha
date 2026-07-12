package documents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/expression"
)

// Report — bir SyncOnce cagrisinin durust ozeti (UI/log icin). Basarisiz/atlanan dosyalar
// SESSIZCE yutulmaz — Errors ve Skipped alanlarinda gorunur kalir.
type Report struct {
	Added   int
	Updated int
	Removed int
	Skipped int
	Errors  []string
}

// Syncer — Dir'i tarayip degisen dosyalari OWUI knowledge'ina PUSH eder (metin, maskelenmis).
type Syncer struct {
	Dir         string // izlenen yerel dizin.
	KnowledgeID string // hedef OWUI knowledge id.
	Mask        bool   // true ise expression.MaskText uygulanir (varsayilan: acik).
	Client      *OWUIClient
	State       *State

	// Logf — opsiyonel; verilmezse log.Printf kullanilir (durust, sessiz-yutma yok).
	Logf func(format string, args ...any)
}

func (s *Syncer) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// SyncOnce — Dir'i tek seferlik tarar: yeni/degisen dosyalari yukler, silinenleri kaldirir.
func (s *Syncer) SyncOnce(ctx context.Context) (Report, error) {
	var rep Report
	seen := map[string]bool{}

	err := filepath.WalkDir(s.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", path, err))
			return nil // bir dosyadaki hata tum taramayi durdurmasin.
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(s.Dir, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		seen[rel] = true

		if err := s.syncFile(ctx, rel, path, &rep); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", rel, err))
		}
		return nil
	})
	if err != nil {
		return rep, err
	}

	// state'te olup artik diskte olmayan dosyalar => knowledge'dan da kaldir.
	for rel, st := range s.State.Files {
		if seen[rel] {
			continue
		}
		if err := s.removeFile(ctx, st); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s (silme): %v", rel, err))
			continue
		}
		delete(s.State.Files, rel)
		rep.Removed++
	}

	if err := s.State.Save(); err != nil {
		return rep, fmt.Errorf("state kaydedilemedi: %w", err)
	}
	return rep, nil
}

func (s *Syncer) syncFile(ctx context.Context, rel, path string, rep *Report) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	hashHex := hex.EncodeToString(hash[:])

	prev, existed := s.State.Files[rel]
	if existed && prev.Hash == hashHex {
		return nil // degismemis => dokunma (dedup).
	}

	text, err := ExtractText(path)
	if err != nil {
		if errors.Is(err, ErrUnsupported) || errors.Is(err, ErrTooLarge) {
			s.logf("documents: atlandi (%s): %v", rel, err)
			rep.Skipped++
			return nil
		}
		return err
	}

	if s.Mask {
		text = expression.MaskText(text)
	}

	// onceden yuklenmis bir surum varsa, yeni surumu eklemeden once eskisini kaldir
	// (OWUI'de yetim/duplike dosya birikmesin).
	if existed && prev.FileID != "" {
		if err := s.removeFile(ctx, prev); err != nil {
			s.logf("documents: eski surum kaldirilamadi (%s): %v", rel, err)
		}
	}

	uploadName := rel
	if !strings.HasSuffix(strings.ToLower(uploadName), ".txt") {
		uploadName += ".txt" // OWUI'ye her zaman metin olarak gideriz (masked context).
	}

	fileID, err := s.Client.UploadFile(ctx, uploadName, []byte(text))
	if err != nil {
		return fmt.Errorf("yukleme: %w", err)
	}
	if err := s.Client.AddFileToKnowledge(ctx, s.KnowledgeID, fileID); err != nil {
		return fmt.Errorf("knowledge'a ekleme: %w", err)
	}

	s.State.Files[rel] = FileState{Hash: hashHex, FileID: fileID, KnowledgeID: s.KnowledgeID}
	if existed {
		rep.Updated++
	} else {
		rep.Added++
	}
	return nil
}

func (s *Syncer) removeFile(ctx context.Context, st FileState) error {
	if st.FileID == "" {
		return nil
	}
	if err := s.Client.RemoveFileFromKnowledge(ctx, st.KnowledgeID, st.FileID); err != nil {
		return err
	}
	return s.Client.DeleteFile(ctx, st.FileID)
}
