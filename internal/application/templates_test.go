package application

import (
	"html/template"
	"strings"
	"testing"

	"resume/internal/models"
	"resume/web"
)

func TestEmbeddedTemplatesParse(t *testing.T) {
	funcs := template.FuncMap{
		"rich":      richText,
		"section":   func(m map[string][]models.Entry, k string) []models.Entry { return m[k] },
		"dateRange": dateRange,
		"checked": func(v bool) string {
			if v {
				return "checked"
			}
			return ""
		},
		"selected": func(a, b string) string {
			if a == b {
				return "selected"
			}
			return ""
		},
		"dict": func(v ...any) map[string]any {
			m := map[string]any{}
			for i := 0; i+1 < len(v); i += 2 {
				if k, ok := v[i].(string); ok {
					m[k] = v[i+1]
				}
			}
			return m
		},
	}
	parsed, err := template.New("root").Funcs(funcs).ParseFS(web.Files, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	page := AdminPage{Section: "profile", CSRF: "test", Selected: models.Language{ID: 1, Code: "en", NativeName: "English"}, Resume: models.Resume{Profile: models.Profile{Avatar: "/uploads/current.png"}, Entries: map[string][]models.Entry{}}}
	var out strings.Builder
	if err = parsed.ExecuteTemplate(&out, "admin", page); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Current profile picture") {
		t.Fatal("current image preview was not rendered")
	}
}

func TestRichTextIsFormattedAndSanitized(t *testing.T) {
	got := string(richText("**Bold** [site](https://example.com) <script>alert(1)</script>"))
	if !strings.Contains(got, "<strong>Bold</strong>") {
		t.Fatalf("bold markdown was not rendered: %s", got)
	}
	if !strings.Contains(got, "href=\"https://example.com\"") {
		t.Fatalf("link markdown was not rendered: %s", got)
	}
	if strings.Contains(got, "<script") {
		t.Fatalf("unsafe script survived sanitization: %s", got)
	}
}

func TestPagination(t *testing.T) {
	tests := []struct{ total, requested, size, wantPage, wantPages int }{{0, 0, 25, 1, 1}, {1, 1, 25, 1, 1}, {26, 1, 25, 1, 2}, {26, 2, 25, 2, 2}, {26, 99, 25, 2, 2}, {51, -2, 25, 1, 3}}
	for _, tc := range tests {
		page, pages := pagination(tc.total, tc.requested, tc.size)
		if page != tc.wantPage || pages != tc.wantPages {
			t.Errorf("pagination(%d,%d,%d)=(%d,%d), want (%d,%d)", tc.total, tc.requested, tc.size, page, pages, tc.wantPage, tc.wantPages)
		}
	}
}
