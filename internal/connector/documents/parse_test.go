package documents

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fumiama/go-docx"
	"github.com/xuri/excelize/v2"
)

func TestExtractTextDOCX(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "belge.docx")

	doc := docx.New().WithDefaultTheme()
	p := doc.AddParagraph()
	p.AddText("Merhaba dunya, bu bir test belgesidir.")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.WriteTo(f); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	got, err := ExtractText(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Merhaba dunya") {
		t.Fatalf("docx icerigi eksik: %q", got)
	}
}

func TestExtractTextRaw(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notlar.txt")
	if err := os.WriteFile(path, []byte("merhaba dunya"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ExtractText(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "merhaba dunya" {
		t.Fatalf("beklenmedik icerik: %q", got)
	}
}

func TestExtractTextMarkdownCSVLog(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".md", ".csv", ".log"} {
		path := filepath.Join(dir, "x"+ext)
		if err := os.WriteFile(path, []byte("icerik-"+ext), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := ExtractText(path)
		if err != nil {
			t.Fatalf("%s: %v", ext, err)
		}
		if got != "icerik-"+ext {
			t.Fatalf("%s: beklenmedik icerik %q", ext, got)
		}
	}
}

func TestExtractTextXLSX(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tablo.xlsx")

	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetCellValue("Sheet1", "A1", "Musteri"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("Sheet1", "B1", "Tutar"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("Sheet1", "A2", "Acme A.S."); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("Sheet1", "B2", 1234); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	got, err := ExtractText(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Musteri") || !strings.Contains(got, "Acme A.S.") || !strings.Contains(got, "1234") {
		t.Fatalf("xlsx icerigi eksik: %q", got)
	}
}

func TestExtractTextUnsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "eski.doc")
	if err := os.WriteFile(path, []byte("binary-ish"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ExtractText(path)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("ErrUnsupported bekleniyordu, got: %v", err)
	}

	// diger bilinmeyen uzantilar da (resim, sunum vb.)
	for _, ext := range []string{".ppt", ".pptx", ".xls", ".png"} {
		p2 := filepath.Join(dir, "x"+ext)
		if err := os.WriteFile(p2, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ExtractText(p2); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("%s: ErrUnsupported bekleniyordu, got: %v", ext, err)
		}
	}
}

func TestExtractTextTooLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "buyuk.txt")
	big := make([]byte, maxFileSize+1)
	if err := os.WriteFile(path, big, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ExtractText(path)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("ErrTooLarge bekleniyordu, got: %v", err)
	}
}

// NOT (PDF): go-docx'un yazma API'si sayesinde .docx fixture'i programatik uretilip test edildi
// (yukarida). PDF icin ayni yol pratik degil (binary format sentezi zahmetli, kutuphanenin
// yazma API'si yok) — PDF yolu TODO: canli bir ornek dosyayla manuel dogrulama (bkz. parse.go
// extractPDF yorumu). Dispatch/ErrUnsupported yollari yukarida zaten kapsanmis durumda.
