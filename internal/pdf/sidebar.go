package pdf

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
	"resume/internal/models"
)

const (
	sidebarWidth = 66.0
	slateR       = 39
	slateG       = 53
	slateB       = 72
	accentR      = 66
	accentG      = 184
	accentB      = 157
)

func writeSidebar(w io.Writer, r models.Resume, fontPath, uploadPath string) error {
	p, family := sidebarDocument(r, fontPath)
	sidebarSetMainMargins(p, r, true)
	p.SetAutoPageBreak(true, 13)
	p.SetHeaderFuncMode(func() {
		p.SetFillColor(slateR, slateG, slateB)
		if p.PageNo() == 1 {
			x := 210.0 - sidebarWidth
			if !isRTL(r) {
				x = 0
			}
			p.Rect(x, 0, sidebarWidth, 297, "F")
			p.SetFillColor(accentR, accentG, accentB)
			accentX := x - 1.2
			if !isRTL(r) {
				accentX = sidebarWidth
			}
			p.Rect(accentX, 0, 1.2, 297, "F")
			return
		}
		x := 205.0
		if !isRTL(r) {
			x = 0
		}
		p.Rect(x, 0, 5, 297, "F")
		p.SetFillColor(accentR, accentG, accentB)
		accentX := x - 1
		if !isRTL(r) {
			accentX = x + 5
		}
		p.Rect(accentX, 0, 1, 297, "F")
	}, true)
	p.SetFooterFunc(func() {
		p.SetY(-9)
		left, _, _, _ := p.GetMargins()
		p.SetX(left)
		p.SetFont(family, "", 7.5)
		p.SetTextColor(127, 137, 149)
		label := joinNonEmpty("  ·  ", r.Profile.Name, sidebarPageLabel(r, p.PageNo()))
		p.CellFormat(0, 4, visual(label, r), "", 0, align(r), false, 0, "")
	})
	p.SetAcceptPageBreakFunc(func() bool {
		sidebarSetMainMargins(p, r, false)
		return true
	})
	p.AddPage()
	sidebarPanel(p, family, r, uploadPath)
	sidebarSetMainMargins(p, r, true)
	p.SetY(12)
	p.SetTextColor(25, 29, 34)

	if strings.TrimSpace(r.Profile.About) != "" {
		sidebarSection(p, family, sectionTitle("about", r), r)
		p.SetFont(family, "", 9.8)
		p.SetTextColor(51, 58, 67)
		multiline(p, plainMarkdown(r.Profile.About), 5.6, r)
		p.Ln(3)
	}

	for _, kind := range []string{"education", "experience", "project", "certification"} {
		items := r.Entries[kind]
		if len(items) == 0 {
			continue
		}
		sidebarSection(p, family, sectionTitle(kind, r), r)
		for _, entry := range items {
			sidebarEntry(p, family, entry, kind, r)
		}
	}
	return p.Output(w)
}

func sidebarDocument(r models.Resume, fontPath string) (*fpdf.Fpdf, string) {
	p := fpdf.New("P", "mm", "A4", "")
	family := "Arial"
	if isRTL(r) {
		p.AddUTF8FontFromBytes("vazirmatn", "", vazirmatn)
		family = "vazirmatn"
	} else if font := findFont(fontPath); font != "" {
		p.AddUTF8Font("resume", "", font)
		family = "resume"
	}
	return p, family
}

func sidebarSetMainMargins(p *fpdf.Fpdf, r models.Resume, firstPage bool) {
	if !firstPage {
		if isRTL(r) {
			p.SetMargins(16, 13, 11)
		} else {
			p.SetMargins(11, 13, 16)
		}
		return
	}
	if isRTL(r) {
		p.SetMargins(16, 13, sidebarWidth+8)
	} else {
		p.SetMargins(sidebarWidth+8, 13, 16)
	}
}

func sidebarPanel(p *fpdf.Fpdf, family string, r models.Resume, uploadPath string) {
	x := 210.0 - sidebarWidth
	if !isRTL(r) {
		x = 0
	}
	center := x + sidebarWidth/2
	y := 11.0
	if r.Profile.Avatar != "" {
		img := filepath.Join(uploadPath, filepath.Base(r.Profile.Avatar))
		if _, err := os.Stat(img); err == nil {
			p.SetDrawColor(accentR, accentG, accentB)
			p.SetLineWidth(1.2)
			p.Circle(center, y+17, 17, "D")
			p.ClipCircle(center, y+17, 15.5, false)
			p.ImageOptions(img, center-15.5, y+1.5, 31, 31, false, fpdf.ImageOptions{ReadDpi: true}, 0, "")
			p.ClipEnd()
			y += 40
		} else {
			y += 8
		}
	}

	p.SetTextColor(255, 255, 255)
	name := visual(r.Profile.Name, r)
	sidebarFittingFont(p, family, name, sidebarWidth-12, 16.5, 12)
	y = sidebarCenteredCell(p, x+6, y, sidebarWidth-12, 8, name)
	p.SetTextColor(191, 226, 216)
	title := visual(r.Profile.Title, r)
	sidebarFittingFont(p, family, title, sidebarWidth-12, 9.2, 7.5)
	y = sidebarCenteredCell(p, x+6, y+1, sidebarWidth-12, 6, title)
	p.SetDrawColor(accentR, accentG, accentB)
	p.SetLineWidth(.7)
	p.Line(center-10, y+3, center+10, y+3)

	y += 12
	y = sidebarPanelHeading(p, family, x, y, sidebarLabel("contact", r), r)
	p.SetFont(family, "", 8.7)
	p.SetTextColor(224, 230, 237)
	for _, value := range []string{r.Profile.Phone, r.Profile.Email, r.Profile.Location} {
		if strings.TrimSpace(value) == "" || y > 276 {
			continue
		}
		y = sidebarListCell(p, x+7, y, sidebarWidth-14, 5.7, visual(value, r), r, "")
	}
	for _, social := range r.Socials {
		if !social.Enabled || strings.TrimSpace(social.Label) == "" || strings.TrimSpace(social.URL) == "" || y > 276 {
			continue
		}
		p.SetTextColor(111, 211, 184)
		y = sidebarListCell(p, x+7, y, sidebarWidth-14, 5.7, visual(social.Label, r), r, social.URL)
	}

	y += 7
	y = sidebarPanelHeading(p, family, x, y, sectionTitle("skill", r), r)
	p.SetFont(family, "", 8.7)
	p.SetTextColor(224, 230, 237)
	for _, entry := range r.Entries["skill"] {
		if y > 255 {
			break
		}
		label := entry.Title
		if entry.Proficiency != "" {
			label = joinNonEmpty(" - ", label, entry.Proficiency)
		}
		y = sidebarListCell(p, x+7, y, sidebarWidth-14, 5.7, visual(label, r), r, "")
	}

	if len(r.Entries["language"]) > 0 && y < 265 {
		y += 7
		y = sidebarPanelHeading(p, family, x, y, sectionTitle("language", r), r)
		p.SetFont(family, "", 8.7)
		p.SetTextColor(224, 230, 237)
		for _, entry := range r.Entries["language"] {
			if y > 282 {
				break
			}
			y = sidebarListCell(p, x+7, y, sidebarWidth-14, 5.7, visual(joinNonEmpty(" - ", entry.Title, entry.Proficiency), r), r, "")
		}
	}
}

func sidebarCenteredCell(p *fpdf.Fpdf, x, y, width, height float64, value string) float64 {
	p.SetXY(x, y)
	p.CellFormat(width, height, fitText(p, value, width), "", 1, "C", false, 0, "")
	return y + height
}

func sidebarFittingFont(p *fpdf.Fpdf, family, value string, maxWidth, startSize, minSize float64) {
	size := startSize
	p.SetFont(family, "", size)
	for p.GetStringWidth(value) > maxWidth && size > minSize {
		size -= .5
		p.SetFont(family, "", size)
	}
}

func sidebarListCell(p *fpdf.Fpdf, x, y, width, height float64, value string, r models.Resume, link string) float64 {
	markerX := x
	textX := x + 3
	textWidth := width - 3
	if isRTL(r) {
		markerX = x + width - 1.2
		textX = x
	}
	p.SetFillColor(accentR, accentG, accentB)
	p.Rect(markerX, y+height/2-.6, 1.2, 1.2, "F")
	p.SetXY(textX, y)
	p.CellFormat(textWidth, height, fitText(p, value, textWidth), "", 1, align(r), false, 0, link)
	return y + height
}

func sidebarPanelHeading(p *fpdf.Fpdf, family string, x, y float64, title string, r models.Resume) float64 {
	p.SetTextColor(255, 255, 255)
	p.SetFont(family, "", 11.5)
	p.SetXY(x+7, y)
	p.CellFormat(sidebarWidth-14, 7, visual(title, r), "", 1, align(r), false, 0, "")
	p.SetDrawColor(100, 119, 143)
	p.SetLineWidth(.35)
	p.Line(x+7, y+7.3, x+sidebarWidth-7, y+7.3)
	p.SetDrawColor(accentR, accentG, accentB)
	if isRTL(r) {
		p.Line(x+sidebarWidth-20, y+7.3, x+sidebarWidth-7, y+7.3)
	} else {
		p.Line(x+7, y+7.3, x+20, y+7.3)
	}
	return y + 10
}

func sidebarSection(p *fpdf.Fpdf, family, title string, r models.Resume) {
	if p.GetY() > 271 {
		sidebarSetMainMargins(p, r, false)
		p.AddPage()
		p.SetY(13)
	}
	left, _, right, _ := p.GetMargins()
	pageWidth, _ := p.GetPageSize()
	p.SetX(left)
	p.SetFillColor(237, 241, 245)
	p.SetTextColor(slateR, slateG, slateB)
	p.SetFont(family, "", 12.5)
	p.CellFormat(pageWidth-left-right, 8.5, visual(title, r), "", 1, align(r), true, 0, "")
	p.SetFillColor(accentR, accentG, accentB)
	accentX := left
	if isRTL(r) {
		accentX = pageWidth - right - 2
	}
	p.Rect(accentX, p.GetY()-8.5, 2, 8.5, "F")
	p.SetTextColor(25, 29, 34)
	p.Ln(3.5)
}

func sidebarEntry(p *fpdf.Fpdf, family string, entry models.Entry, kind string, r models.Resume) {
	if p.GetY() > 250 {
		sidebarSetMainMargins(p, r, false)
		p.AddPage()
		p.SetY(13)
		sidebarSection(p, family, sectionTitle(kind, r), r)
	}
	left, _, right, _ := p.GetMargins()
	pageWidth, _ := p.GetPageSize()
	width := pageWidth - left - right
	dateWidth := 34.0
	date := dateRange(entry, r)
	p.SetX(left)
	p.SetTextColor(30, 36, 44)
	p.SetFont(family, "", 11.2)
	if isRTL(r) {
		p.SetTextColor(accentR-15, accentG-45, accentB-35)
		p.CellFormat(dateWidth, 6.5, visual(date, r), "", 0, "L", false, 0, "")
		p.SetTextColor(30, 36, 44)
		p.CellFormat(width-dateWidth, 6.5, visual(entry.Title, r), "", 1, "R", false, 0, "")
	} else {
		p.CellFormat(width-dateWidth, 6.5, entry.Title, "", 0, "L", false, 0, "")
		p.SetTextColor(accentR-15, accentG-45, accentB-35)
		p.CellFormat(dateWidth, 6.5, date, "", 1, "R", false, 0, "")
		p.SetTextColor(30, 36, 44)
	}
	meta := joinNonEmpty(" · ", entry.Subtitle, entry.Location, entry.Proficiency)
	if meta != "" {
		p.SetTextColor(91, 101, 113)
		p.SetFont(family, "", 9.1)
		multiline(p, meta, 5, r)
	}
	if entry.Description != "" {
		p.SetTextColor(56, 63, 72)
		p.SetFont(family, "", 9.2)
		multiline(p, plainMarkdown(entry.Description), 5.1, r)
	}
	if entry.Extra != "" {
		p.SetFont(family, "", 8.5)
		multiline(p, plainMarkdown(entry.Extra), 4.6, r)
	}
	if entry.Technologies != "" {
		p.SetTextColor(72, 117, 108)
		p.SetFont(family, "", 8.3)
		multiline(p, entry.Technologies, 4.6, r)
	}
	if entry.URL != "" || entry.SecondaryURL != "" {
		p.SetTextColor(accentR-10, accentG-25, accentB-20)
		p.SetFont(family, "", 8)
		if entry.URL != "" {
			p.CellFormat(p.GetStringWidth(sidebarLabel("website", r)), 4.5, visual(sidebarLabel("website", r), r), "", 0, "L", false, 0, entry.URL)
		}
		if entry.SecondaryURL != "" {
			p.CellFormat(p.GetStringWidth("  GitHub"), 4.5, "  GitHub", "", 0, "L", false, 0, entry.SecondaryURL)
		}
		p.Ln(5)
	}
	p.SetTextColor(25, 29, 34)
	p.SetDrawColor(222, 227, 232)
	p.SetLineWidth(.25)
	p.Line(left, p.GetY()+2, pageWidth-right, p.GetY()+2)
	p.Ln(5.5)
}

func sidebarLabel(key string, r models.Resume) string {
	if r.Language.Code == "fa" {
		return map[string]string{
			"contact": "اطلاعات تماس",
			"website": "وب‌سایت",
		}[key]
	}
	return map[string]string{
		"contact": "Contact",
		"website": "Website",
	}[key]
}

func sidebarPageLabel(r models.Resume, page int) string {
	if r.Language.Code == "fa" {
		number := strconv.Itoa(page)
		number = strings.NewReplacer("0", "۰", "1", "۱", "2", "۲", "3", "۳", "4", "۴", "5", "۵", "6", "۶", "7", "۷", "8", "۸", "9", "۹").Replace(number)
		return "صفحه " + number
	}
	return "Page " + strconv.Itoa(page)
}
