package pdf

import (
	"bytes"
	"testing"

	"resume/internal/models"
)

func TestPersianVisualShaping(t *testing.T) {
	r := models.Resume{Language: models.Language{Code: "fa", Direction: "rtl"}}
	logical := "مهندس نرم‌افزار"
	got := visual(logical, r)
	if got == logical {
		t.Fatalf("expected shaped and reordered output, got unchanged text %q", got)
	}
	if got == "" {
		t.Fatal("shaped output is empty")
	}
}

func TestPersianPDFUsesEmbeddedFont(t *testing.T) {
	r := models.Resume{Language: models.Language{Code: "fa", Direction: "rtl"}, Profile: models.Profile{Name: "الکس مورگان", Title: "مهندس نرم‌افزار", About: "برای مسائل پیچیده، محصولات ساده و قابل اعتماد می‌سازم."}, Entries: map[string][]models.Entry{}}
	var out bytes.Buffer
	if err := Write(&out, r, "", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if out.Len() < 5000 {
		t.Fatalf("expected embedded-font PDF, got only %d bytes", out.Len())
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("%PDF")) {
		t.Fatal("output is not a PDF")
	}
}
