package documents

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := NewState(path)
	s.Files["a.txt"] = FileState{Hash: "h1", FileID: "f1", KnowledgeID: "kb1"}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Files["a.txt"]; got.Hash != "h1" || got.FileID != "f1" || got.KnowledgeID != "kb1" {
		t.Fatalf("beklenmedik state: %+v", got)
	}
}

func TestStateLoadMissingIsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "yok.json")
	s, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Files) != 0 {
		t.Fatalf("bos state bekleniyordu: %+v", s.Files)
	}
}

func TestStateFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix izin biti windows'ta anlamli degil")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s := NewState(path)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("beklenmedik izin: %v", perm)
	}
}
