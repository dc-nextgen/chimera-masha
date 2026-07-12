// Package documents — Masha'nin UCUNCU sirket-bilgisi kaynagi (DB ve ERP-API'den sonra).
//
// Musteri Masha'ya YEREL bir DIZIN gosterir; Masha bu dizini IZLER (fsnotify), degisen
// dosyalardan metin CIKARIR, metni yerelde PII-MASKELER (§3/§17.7 anlatisi: "ham veri
// yerelde kalir, PII-temiz baglam gider" — variant b), ve maskelenmis metni Open WebUI'nin
// knowledge-base'ine (RAG, Qdrant) PUSH eder. Masha burada ince kalir: izle+cikar+maskele+
// gonder. Agir is (embed/chunk/retrieve/atif) OWUI'de. Bu paket toolserver/registry
// Connector arayuzunu UYGULAMAZ (Call/Health/Close YOK) — sorgu degil, arka-plan SENKRON
// kaynagidir; main.go'da ayri baslatilir (bu adimda main.go'ya DOKUNULMUYOR).
package documents

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/fumiama/go-docx"
	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
)

// ErrUnsupported — bu uzanti icin metin cikarimi desteklenmiyor (ör. .doc/.ppt/.xls, taranmis
// PDF, resim). Sync katmani bunu HATA degil, dostane bir SKIP olarak ele alir (durust log).
var ErrUnsupported = errors.New("documents: desteklenmeyen dosya turu")

// maxFileSize — bellekte sinirsiz buyumeyi onlemek icin bir tavan. Asan dosyalar SKIP edilir
// (loglanir, hata firlatilmaz) — PoC kapsaminda basit ama durust bir sinir.
const maxFileSize = 25 * 1024 * 1024 // 25MB

// ErrTooLarge — dosya boyut tavanini asti (bkz. maxFileSize).
var ErrTooLarge = errors.New("documents: dosya cok buyuk (25MB tavan)")

// ExtractText — path'teki dosyadan DUZ METIN cikarir. Uzantiya gore dispatch eder.
func ExtractText(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > maxFileSize {
		return "", ErrTooLarge
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md", ".csv", ".log":
		return extractRaw(path)
	case ".xlsx":
		return extractXLSX(path)
	case ".docx":
		return extractDOCX(path)
	case ".pdf":
		return extractPDF(path)
	default:
		return "", ErrUnsupported
	}
}

func extractRaw(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// extractXLSX — excelize ile tum sayfalari, tum hucreleri dolasir; hucreleri TAB, satirlari
// NEWLINE ile birlestirir. Formul/stil bilgisi atlanir — yalnizca gorunen metin degeri.
func extractXLSX(path string) (string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var b strings.Builder
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue // bir sayfa okunamazsa digerlerini kaybetme (kismi metin > hic metin).
		}
		for _, row := range rows {
			b.WriteString(strings.Join(row, "\t"))
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

// extractDOCX — go-docx ile paragraf metnini toplar.
func extractDOCX(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}

	doc, err := docx.Parse(f, fi.Size())
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for _, it := range doc.Document.Body.Items {
		if p, ok := it.(*docx.Paragraph); ok {
			for _, c := range p.Children {
				r, ok := c.(*docx.Run)
				if !ok {
					continue
				}
				for _, rc := range r.Children {
					if t, ok := rc.(*docx.Text); ok {
						b.WriteString(t.Text)
					}
				}
			}
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

// extractPDF — ledongthuc/pdf ile YALNIZ metin-tabanli PDF'lerden metin cikarir. Taranmis
// (goruntu) PDF'ler icin OCR YOK — bos/eksik metin donebilir (durust sinir, §PoC).
// TODO(pdf-fixture): testte gercek bir PDF programatik uretmek pratik degil (binary format);
// dispatch/ErrUnsupported yolu raw-text ve bilinmeyen-uzanti testleriyle kapsanir, PDF'in
// kendisi hermetik testte SENTETIK EDILMEDI — canli bir ornek dosyayla manuel dogrulanmali.
func extractPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var b strings.Builder
	totalPage := r.NumPage()
	for i := 1; i <= totalPage; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		content, err := p.GetPlainText(nil)
		if err != nil {
			continue // sayfa okunamazsa digerlerini kaybetme.
		}
		b.WriteString(content)
		b.WriteString("\n")
	}
	return b.String(), nil
}
