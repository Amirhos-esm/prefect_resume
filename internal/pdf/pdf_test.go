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

func TestPDFIncludesClickableSocialLinks(t *testing.T) {
	r := models.Resume{
		Language: models.Language{Code: "en", Direction: "ltr"},
		Profile:  models.Profile{Name: "Alex Morgan"},
		Entries:  map[string][]models.Entry{},
		Socials: []models.Social{
			{Label: "GitHub", URL: "https://github.com/example", Enabled: true},
			{Label: "Hidden", URL: "https://example.com/hidden", Enabled: false},
		},
	}
	var out bytes.Buffer
	if err := Write(&out, r, "", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("https://github.com/example")) {
		t.Fatal("enabled social URL is missing from PDF hyperlink annotations")
	}
	if bytes.Contains(out.Bytes(), []byte("https://example.com/hidden")) {
		t.Fatal("disabled social URL was included in PDF")
	}
}

func TestSidebarTemplateSupportsLTRAndRTL(t *testing.T) {
	for _, direction := range []string{"ltr", "rtl"} {
		r := models.Resume{
			PDFTemplate: "sidebar",
			Language:    models.Language{Code: "en", Direction: direction},
			Profile: models.Profile{
				Name:  "Alex Morgan",
				Title: "Software Engineer",
				About: "Engineer focused on maintainable products and dependable systems.",
				Email: "alex@example.com",
			},
			Entries: map[string][]models.Entry{
				"skill":      {{Title: "Go"}, {Title: "SQLite"}},
				"language":   {{Title: "English", Proficiency: "Fluent"}},
				"experience": {{Title: "Senior Engineer", Subtitle: "Example Co", StartDate: "2022", Current: true, Description: "Built reliable services."}},
			},
			Socials: []models.Social{{Label: "GitHub", URL: "https://github.com/example", Enabled: true}},
		}
		var out bytes.Buffer
		if err := Write(&out, r, "", t.TempDir()); err != nil {
			t.Fatalf("%s sidebar PDF: %v", direction, err)
		}
		if !bytes.HasPrefix(out.Bytes(), []byte("%PDF")) || out.Len() < 3000 {
			t.Fatalf("%s sidebar output is not a valid-sized PDF", direction)
		}
		if !bytes.Contains(out.Bytes(), []byte("https://github.com/example")) {
			t.Fatalf("%s sidebar PDF lost social hyperlink", direction)
		}
	}
}
