package documents

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FileState — bir dosyanin son senkron durumu. §3: bu durum YERELDE kalir (0600 JSON).
type FileState struct {
	Hash        string `json:"hash"`         // sha256(hex) — HAM dosya baytlarindan (dedup icin).
	FileID      string `json:"file_id"`      // OWUI /api/v1/files donen id.
	KnowledgeID string `json:"knowledge_id"` // hangi knowledge'a eklendi (birden fazla olabilir gelecekte; PoC=tek).
}

// State — relpath -> FileState. Disk uzerinde atomik (temp+rename) yazilir, 0600.
type State struct {
	path  string
	Files map[string]FileState `json:"files"`
}

// NewState — bos bir state olusturur (path sadece Save icin tutulur).
func NewState(path string) *State {
	return &State{path: path, Files: map[string]FileState{}}
}

// LoadState — path'ten state okur; dosya yoksa BOS state doner (ilk calistirma normaldir).
func LoadState(path string) (*State, error) {
	s := NewState(path)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	if s.Files == nil {
		s.Files = map[string]FileState{}
	}
	s.path = path
	return s, nil
}

// Save — state'i ATOMIK (gecici dosya + rename) ve 0600 izinle path'e yazar.
func (s *State) Save() error {
	if s.path == "" {
		return nil // path verilmemisse (ör. sadece testte) sessizce atla.
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
