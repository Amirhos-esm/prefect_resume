package pdf

import (
	_ "embed"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/mehran-prs/gopersian"
	"resume/internal/models"
)

// Vazirmatn is an OFL-licensed Persian/Arabic typeface embedded so PDF output
// remains portable and does not depend on fonts installed on the server.
//
//go:embed fonts/Vazirmatn-Regular.ttf
var vazirmatn []byte

func Write(w io.Writer, r models.Resume, fontPath, uploadPath string) error {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(18, 16, 18)
	p.SetAutoPageBreak(true, 16)
	family := "Arial"
	if isRTL(r) {
		p.AddUTF8FontFromBytes("vazirmatn", "", vazirmatn)
		family = "vazirmatn"
	} else if font := findFont(fontPath); font != "" {
		p.AddUTF8Font("resume", "", font)
		family = "resume"
	}

	modern := r.PDFTemplate == "modern"
	p.AddPage()
	if modern {
		p.SetFillColor(28, 44, 39)
		p.Rect(0, 0, 210, 43, "F")
		p.SetTextColor(255, 255, 255)
		p.SetXY(18, 13)
	} else {
		p.SetTextColor(25, 31, 29)
	}
	if r.Profile.Avatar != "" {
		img := filepath.Join(uploadPath, filepath.Base(r.Profile.Avatar))
		if _, err := os.Stat(img); err == nil {
			x := 166.0
			if isRTL(r) {
				x = 18
			}
			p.ImageOptions(img, x, 9, 26, 30, false, fpdf.ImageOptions{ReadDpi: true}, 0, "")
		}
	}
	p.SetFont(family, "", 24)
	p.CellFormat(0, 10, visual(r.Profile.Name, r), "", 1, align(r), false, 0, "")
	p.SetFont(family, "", 12)
	p.CellFormat(0, 7, visual(r.Profile.Title, r), "", 1, align(r), false, 0, "")
	if modern {
		p.SetTextColor(35, 43, 40)
		p.SetY(50)
	} else {
		p.Ln(3)
	}
	p.SetFont(family, "", 9)
	multiline(p, joinNonEmpty(" · ", r.Profile.Email, r.Profile.Phone, r.Profile.Location), 5, r)
	p.Ln(4)
	if r.Profile.About != "" {
		heading(p, family, sectionTitle("about", r), modern, r)
		body(p, family, plainMarkdown(r.Profile.About), r)
		p.Ln(3)
	}
	for _, kind := range []string{"experience", "education", "skill", "project", "certification", "language"} {
		items := r.Entries[kind]
		if len(items) == 0 {
			continue
		}
		heading(p, family, sectionTitle(kind, r), modern, r)
		for _, e := range items {
			if e.Image != "" && kind == "project" {
				img := filepath.Join(uploadPath, filepath.Base(e.Image))
				if _, err := os.Stat(img); err == nil {
					y := p.GetY()
					// Reserve a dedicated image row before rendering project text.
					x := 18.0
					if isRTL(r) {
						x = 138
					} // A4 width - right margin - image width.
					p.ImageOptions(img, x, y, 54, 30, false, fpdf.ImageOptions{ReadDpi: true}, 0, "")
					p.SetY(y + 33)
				}
			}
			p.SetFont(family, "", 11)
			p.CellFormat(0, 6, visual(e.Title, r), "", 1, align(r), false, 0, "")
			p.SetFont(family, "", 9)
			meta := joinNonEmpty(" · ", e.Subtitle, dateRange(e, r), e.Location, e.Proficiency)
			if meta != "" {
				p.SetTextColor(95, 105, 101)
				multiline(p, meta, 5, r)
				p.SetTextColor(35, 43, 40)
			}
			if e.Description != "" {
				body(p, family, plainMarkdown(e.Description), r)
			}
			if e.Technologies != "" {
				p.SetFont(family, "", 8)
				multiline(p, e.Technologies, 4, r)
			}
			p.Ln(3)
		}
	}
	return p.Output(w)
}

func heading(p *fpdf.Fpdf, font, title string, modern bool, r models.Resume) {
	if p.GetY() > 265 {
		p.AddPage()
	}
	if modern {
		p.SetTextColor(50, 100, 83)
	} else {
		p.SetTextColor(30, 35, 33)
	}
	p.SetFont(font, "", 14)
	p.CellFormat(0, 8, visual(title, r), "B", 1, align(r), false, 0, "")
	p.SetTextColor(35, 43, 40)
	p.Ln(2)
}

func body(p *fpdf.Fpdf, font, text string, r models.Resume) {
	p.SetFont(font, "", 9)
	multiline(p, text, 5, r)
}

// multiline wraps logical Persian text first and shapes/reorders each visual
// line separately. Reordering a whole paragraph before wrapping would place
// the final words on the first rendered line.
func multiline(p *fpdf.Fpdf, text string, lineHeight float64, r models.Resume) {
	if !isRTL(r) {
		p.MultiCell(0, lineHeight, text, "", "L", false)
		return
	}
	pageWidth, _ := p.GetPageSize()
	left, _, right, _ := p.GetMargins()
	maxWidth := pageWidth - left - right
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			p.Ln(lineHeight)
			continue
		}
		line := ""
		for _, word := range words {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if line != "" && p.GetStringWidth(visual(candidate, r)) > maxWidth {
				p.CellFormat(0, lineHeight, visual(line, r), "", 1, "R", false, 0, "")
				line = word
			} else {
				line = candidate
			}
		}
		if line != "" {
			p.CellFormat(0, lineHeight, visual(line, r), "", 1, "R", false, 0, "")
		}
	}
}

func visual(text string, r models.Resume) string {
	if !isRTL(r) || text == "" {
		return text
	}
	shaped, err := gopersian.Bidi(text)
	if err != nil {
		return gopersian.Shape(text)
	}
	return shaped
}
func isRTL(r models.Resume) bool { return r.Language.Direction == "rtl" }
func align(r models.Resume) string {
	if isRTL(r) {
		return "R"
	}
	return "L"
}
func sectionTitle(kind string, r models.Resume) string {
	if r.Language.Code == "fa" {
		return map[string]string{"about": "درباره من", "experience": "سوابق کاری", "education": "تحصیلات", "skill": "مهارت‌ها", "project": "پروژه‌ها", "certification": "گواهینامه‌ها", "language": "زبان‌ها"}[kind]
	}
	return map[string]string{"about": "About", "experience": "Experience", "education": "Education", "skill": "Skills", "project": "Projects", "certification": "Certifications", "language": "Languages"}[kind]
}
func dateRange(e models.Entry, r models.Resume) string {
	if e.Current {
		if isRTL(r) {
			return strings.TrimSpace(e.StartDate + " — اکنون")
		}
		return strings.TrimSpace(e.StartDate + " — Present")
	}
	if e.StartDate != "" && e.EndDate != "" {
		return e.StartDate + " — " + e.EndDate
	}
	return e.StartDate + e.EndDate
}
func joinNonEmpty(sep string, values ...string) string {
	var out []string
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, sep)
}

var markdownLink = regexp.MustCompile(`\[([^]]+)\]\(([^)]+)\)`)

func plainMarkdown(value string) string {
	value = markdownLink.ReplaceAllString(value, "$1 ($2)")
	replacer := strings.NewReplacer("**", "", "__", "", "`", "", "### ", "", "## ", "", "# ", "")
	return replacer.Replace(value)
}
func findFont(configured string) string {
	candidates := []string{configured}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, `C:\Windows\Fonts\arial.ttf`, `C:\Windows\Fonts\tahoma.ttf`)
	} else {
		candidates = append(candidates, "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf")
	}
	for _, path := range candidates {
		if path != "" {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path
			}
		}
	}
	return ""
}
