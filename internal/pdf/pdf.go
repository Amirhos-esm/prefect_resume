package pdf

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-pdf/fpdf"
	"resume/internal/models"
)

func Write(w io.Writer, r models.Resume, fontPath, uploadPath string) error {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(18, 16, 18)
	p.SetAutoPageBreak(true, 16)
	family := "Arial"
	if font := findFont(fontPath); font != "" {
		p.AddUTF8Font("resume", "", font)
		family = "resume"
	}
	if r.Language.Direction == "rtl" {
		p.RTL()
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
			if r.Language.Direction == "rtl" {
				x = 18
			}
			p.ImageOptions(img, x, 9, 26, 30, false, fpdf.ImageOptions{ReadDpi: true}, 0, "")
		}
	}
	p.SetFont(family, "", 24)
	p.CellFormat(0, 10, r.Profile.Name, "", 1, align(r), false, 0, "")
	p.SetFont(family, "", 12)
	p.CellFormat(0, 7, r.Profile.Title, "", 1, align(r), false, 0, "")
	if modern {
		p.SetTextColor(35, 43, 40)
		p.SetY(50)
	} else {
		p.Ln(3)
	}
	p.SetFont(family, "", 9)
	p.MultiCell(0, 5, joinNonEmpty(" · ", r.Profile.Email, r.Profile.Phone, r.Profile.Location), "", align(r), false)
	p.Ln(4)
	if r.Profile.About != "" {
		heading(p, family, "About", modern, r)
		body(p, family, r.Profile.About, r)
		p.Ln(3)
	}
	sections := []struct{ k, t string }{{"experience", "Experience"}, {"education", "Education"}, {"skill", "Skills"}, {"project", "Projects"}, {"certification", "Certifications"}, {"language", "Languages"}}
	for _, s := range sections {
		items := r.Entries[s.k]
		if len(items) == 0 {
			continue
		}
		heading(p, family, s.t, modern, r)
		for _, e := range items {
			if e.Image != "" && s.k == "project" {
				img := filepath.Join(uploadPath, filepath.Base(e.Image))
				if _, err := os.Stat(img); err == nil {
					y := p.GetY()
					p.ImageOptions(img, 18, y, 28, 0, false, fpdf.ImageOptions{ReadDpi: true}, 0, "")
					p.SetX(50)
				}
			}
			p.SetFont(family, "", 11)
			p.CellFormat(0, 6, e.Title, "", 1, align(r), false, 0, "")
			p.SetFont(family, "", 9)
			meta := joinNonEmpty(" · ", e.Subtitle, dateRange(e), e.Location, e.Proficiency)
			if meta != "" {
				p.SetTextColor(95, 105, 101)
				p.CellFormat(0, 5, meta, "", 1, align(r), false, 0, "")
				p.SetTextColor(35, 43, 40)
			}
			if e.Description != "" {
				body(p, family, e.Description, r)
			}
			if e.Technologies != "" {
				p.SetFont(family, "", 8)
				p.MultiCell(0, 4, e.Technologies, "", align(r), false)
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
	p.CellFormat(0, 8, title, "B", 1, align(r), false, 0, "")
	p.SetTextColor(35, 43, 40)
	p.Ln(2)
}
func body(p *fpdf.Fpdf, font, text string, r models.Resume) {
	p.SetFont(font, "", 9)
	p.MultiCell(0, 5, text, "", align(r), false)
}
func align(r models.Resume) string {
	if r.Language.Direction == "rtl" {
		return "R"
	}
	return "L"
}
func dateRange(e models.Entry) string {
	if e.Current {
		return strings.TrimSpace(e.StartDate + " — Present")
	}
	if e.StartDate != "" && e.EndDate != "" {
		return e.StartDate + " — " + e.EndDate
	}
	return e.StartDate + e.EndDate
}
func joinNonEmpty(sep string, v ...string) string {
	var out []string
	for _, x := range v {
		if strings.TrimSpace(x) != "" {
			out = append(out, x)
		}
	}
	return strings.Join(out, sep)
}
func findFont(configured string) string {
	candidates := []string{configured}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, `C:\Windows\Fonts\arial.ttf`, `C:\Windows\Fonts\tahoma.ttf`)
	} else {
		candidates = append(candidates, "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf")
	}
	for _, p := range candidates {
		if p != "" {
			if s, e := os.Stat(p); e == nil && !s.IsDir() {
				return p
			}
		}
	}
	return ""
}
