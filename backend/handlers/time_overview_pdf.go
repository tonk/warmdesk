package handlers

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
)

// GetOverviewNormPDF renders the Norm overview (GET /overview/norm) of one
// user's year as a landscape A4 PDF: the year cards, the norm basis line, the
// month table with totals and the public holidays on weekdays.
// Route: GET /api/v1/time-entries/overview/norm/pdf
// Query params: year, user_id (admins / time-tracking viewers), lang, font.
func GetOverviewNormPDF(c *gin.Context) {
	sc, status, msg := loadOverviewScope(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	o := buildNormOverview(sc)

	userID := middleware.GetUserID(c)
	var requester models.User
	database.DB.First(&requester, userID)

	fontFamily := "FreeSans"
	if requester.Font != "" {
		fontFamily = pdfFontFamily(requester.Font)
	}
	if fam, ok := pdfFontFromParam(c.Query("font")); ok {
		fontFamily = fam
	}
	tr := pdfI18nFromLang(c.Query("lang"))
	lb := normPDFLabelsFromLang(c.Query("lang"))

	employee := userDisplayName(sc.UserID)

	settings := loadAllSettings()
	company := settings[settingCompanyName]
	if company == "" {
		company = "WarmDesk"
	}
	dateLayout := jsDateOnlyToGoLayout(requester.DateTimeFormat)

	buf, err := buildNormOverviewPDF(o, lb, tr, fontFamily, company, settings[settingCompanyLogo], employee, dateLayout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pdf generation failed"})
		return
	}
	filename := fmt.Sprintf("norm-overview-%d.pdf", o.Year)
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

func normPDFLabelsFromLang(lang string) normPDFLabels {
	code := strings.ToLower(lang)
	if i := strings.IndexByte(code, '-'); i > 0 {
		code = code[:i]
	}
	if l, ok := normPDFTranslations[code]; ok {
		return l
	}
	return normPDFTranslations["en"]
}

// fmtHMSigned formats minutes as H:MM, negative with "-", positive with "+"
// (like fmtSignedHM in the frontend); 0 is "0:00".
func fmtHMSigned(m int) string {
	switch {
	case m > 0:
		return "+" + fmtHM(m)
	case m < 0:
		return "-" + fmtHM(-m)
	}
	return "0:00"
}

// fmtHM formats a non-negative number of minutes as H:MM.
func fmtHM(m int) string {
	if m < 0 {
		return "-" + fmtHM(-m)
	}
	return fmt.Sprintf("%d:%02d", m/60, m%60)
}

// fmtDays formats a day count with at most two decimals (1, 1.5, 0.25).
func fmtDays(d float64) string {
	return strconv.FormatFloat(math.Round(d*100)/100, 'f', -1, 64)
}

// wrapWords breaks text into lines of at most w mm in the current font,
// only between words. gofpdf's SplitLines measures UTF-8 fonts per byte and
// breaks words like "Viðmiðsdagar" in the middle, so GetStringWidth is used.
// A single word wider than w gets a line of its own.
func wrapWords(pdf *gofpdf.Fpdf, text string, w float64) []string {
	var lines []string
	cur := ""
	for _, word := range strings.Fields(text) {
		next := word
		if cur != "" {
			next = cur + " " + word
		}
		if cur != "" && pdf.GetStringWidth(next) > w {
			lines = append(lines, cur)
			cur = word
			continue
		}
		cur = next
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// capitalize upper-cases the first letter ("januari" → "Januari").
func capitalize(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[:1])) + string(r[1:])
}

var (
	nClrPos = rgb{22, 128, 61} // green — booked over the norm
	nClrNeg = gClrDanger       // red — booked under the norm
)

func diffColor(m int) rgb {
	switch {
	case m > 0:
		return nClrPos
	case m < 0:
		return nClrNeg
	}
	return gClrText
}

func buildNormOverviewPDF(o NormOverview, lb normPDFLabels, tr pdfI18n, ff, company, logo, employee, dateLayout string) (bytes.Buffer, error) {
	pdf := newLandscapePDF(ff)
	title := fmt.Sprintf("%s — %s %d", lb.Title, lb.Norm, o.Year)
	pdf.SetTitle(title, true)
	pdf.SetAuthor(employee, true)
	pdf.SetSubject(company+" — "+title, true)
	pdf.SetCreator(company, true)
	setGridFooter(pdf, ff, company, title, tr)
	pdf.AddPage()

	y := drawGridDocHeader(pdf, ff, logo, company, employee, title)

	// ── Year cards ───────────────────────────────────────────────────────────
	type card struct {
		label, value string
		color        rgb
	}
	cards := []card{
		{lb.NormTime, fmtHM(o.Total.NormMinutes), gClrText},
		{lb.Booked, fmtHM(o.Total.BookedMinutes), gClrText},
		{lb.Diff, fmtHMSigned(o.Total.DiffMinutes), diffColor(o.Total.DiffMinutes)},
		{lb.NormToDate, fmtHM(o.NormToDateMinutes), gClrText},
		{lb.BookedToDate, fmtHM(o.BookedToDateMinute), gClrText},
		{lb.DiffToDate, fmtHMSigned(o.DiffToDateMinutes), diffColor(o.DiffToDateMinutes)},
		{lb.Planned, fmtHM(o.PlannedMinutes), gClrText},
		{lb.Days, strconv.Itoa(o.TotalDays), gClrText},
	}
	const gap = 2.0
	cardW := (gBodyW - gap*float64(len(cards)-1)) / float64(len(cards))
	const cardH = 14.0
	for i, cd := range cards {
		x := gMargin + float64(i)*(cardW+gap)
		setFill(pdf, gClrHdrFill)
		setDraw(pdf, rgb{220, 225, 235})
		pdf.SetLineWidth(0.2)
		pdf.RoundedRect(x, y, cardW, cardH, 1.5, "1234", "FD")
		pdf.SetXY(x+2, y+1.5)
		pdf.SetFont(ff, "", 7)
		setTxt(pdf, gClrMuted)
		pdf.CellFormat(cardW-4, 4, truncStr(cd.label, 34), "", 2, "L", false, 0, "")
		pdf.SetX(x + 2)
		pdf.SetFont(ff, "B", 12)
		setTxt(pdf, cd.color)
		pdf.CellFormat(cardW-4, 7, cd.value, "", 0, "L", false, 0, "")
	}
	y += cardH + 3

	// ── Norm basis line ──────────────────────────────────────────────────────
	vacDays := 0.0
	if o.NormMinutesPerDay > 0 {
		vacDays = float64(o.Total.VacationMinutes) / float64(o.NormMinutesPerDay)
	}
	basis := strings.NewReplacer(
		"{hours}", fmtDays(o.HoursPerWeek),
		"{perDay}", fmtHM(o.NormMinutesPerDay),
		"{days}", fmtDays(vacDays),
	).Replace(lb.NormBasis)
	pdf.SetXY(gMargin, y)
	pdf.SetFont(ff, "", 8.5)
	setTxt(pdf, gClrMuted)
	pdf.CellFormat(gBodyW, 5, basis, "", 1, "L", false, 0, "")
	y = pdf.GetY() + 2

	// ── Month table ──────────────────────────────────────────────────────────
	heads := []string{lb.Month, lb.NormDays, lb.NormTime, lb.Worked, lb.OfWhichTravel, lb.OfWhichDeclarable,
		lb.Kilometres, lb.Sick, lb.Vacation, lb.Booked, lb.Diff, lb.BookedDays, lb.UnbookedNormDays}
	const colMonth = 31.0
	colW := (gBodyW - colMonth) / float64(len(heads)-1)
	width := func(i int) float64 {
		if i == 0 {
			return colMonth
		}
		return colW
	}

	// Header: wrapped labels, all cells as tall as the tallest one. A label
	// whose longest word does not fit gets a smaller font instead of being
	// broken in the middle of that word ("Niet-geboekte").
	const lineH = 3.6
	sizes := make([]float64, len(heads))
	maxLines := 1
	for i, h := range heads {
		size := 7.5
		for ; size > 5; size -= 0.5 {
			pdf.SetFont(ff, "B", size)
			fits := true
			for _, word := range strings.Fields(h) {
				if pdf.GetStringWidth(word) > width(i)-2 {
					fits = false
					break
				}
			}
			if fits {
				break
			}
		}
		sizes[i] = size
		pdf.SetFont(ff, "B", size)
		if n := len(wrapWords(pdf, h, width(i)-2)); n > maxLines {
			maxLines = n
		}
	}
	hdrH := float64(maxLines)*lineH + 2
	x := gMargin
	for i, h := range heads {
		w := width(i)
		setFill(pdf, gClrPrimary)
		pdf.Rect(x, y, w, hdrH, "F")
		setTxt(pdf, gClrWhite)
		align := "R"
		if i == 0 {
			align = "L"
		}
		pdf.SetFont(ff, "B", sizes[i])
		lines := wrapWords(pdf, h, w-2)
		ty := y + (hdrH-float64(len(lines))*lineH)/2
		for _, ln := range lines {
			pdf.SetXY(x+1, ty)
			pdf.CellFormat(w-2, lineH, ln, "", 0, align, false, 0, "")
			ty += lineH
		}
		x += w
	}
	y += hdrH

	const rowH = 6.0
	row := func(label string, r normMonthRow, days int, bold, alt bool) {
		style := ""
		if bold {
			style = "B"
		}
		fill := alt || bold
		switch {
		case bold:
			setFill(pdf, gClrTotFill)
		case alt:
			setFill(pdf, gClrAlt)
		}
		setDraw(pdf, rgb{220, 225, 235})
		pdf.SetLineWidth(0.1)
		cells := []string{label, strconv.Itoa(r.NormDays), fmtHM(r.NormMinutes), fmtHM(r.WorkedMinutes),
			fmtHM(r.TravelMinutes), fmtHM(r.TravelDeclarableMinutes), fmtDistance(r.Distance),
			fmtHM(r.SickMinutes), fmtHM(r.VacationMinutes), fmtHM(r.BookedMinutes), fmtHMSigned(r.DiffMinutes),
			strconv.Itoa(days), strconv.Itoa(r.UnbookedNormDays)}
		pdf.SetXY(gMargin, y)
		for i, v := range cells {
			align := "R"
			if i == 0 {
				align = "L"
			}
			setTxt(pdf, gClrText)
			if i == 10 {
				setTxt(pdf, diffColor(r.DiffMinutes))
			}
			pdf.SetFont(ff, style, 8)
			pdf.CellFormat(width(i), rowH, v, "B", 0, align, fill, 0, "")
		}
		y += rowH
	}
	for i, m := range o.Months {
		row(capitalize(tr.MonthsFull[m.Month-1]), m, m.BookedDays, false, i%2 == 1)
	}
	// Days in the totals row are distinct dates (total_days), not a sum of rows.
	row(lb.Total, o.Total, o.TotalDays, true, false)

	// ── Notes ────────────────────────────────────────────────────────────────
	y += 2
	pdf.SetXY(gMargin, y)
	pdf.SetFont(ff, "", 8)
	setTxt(pdf, gClrMuted)
	pdf.CellFormat(gBodyW, 5, strings.ReplaceAll(lb.DaysNote, "{n}", strconv.Itoa(o.TotalDays)), "", 1, "L", false, 0, "")
	if o.Total.PastUnbookedNormDays > 0 {
		pdf.SetX(gMargin)
		setTxt(pdf, gClrDanger)
		pdf.CellFormat(gBodyW, 5, strings.ReplaceAll(lb.PastUnbooked, "{n}", strconv.Itoa(o.Total.PastUnbookedNormDays)), "", 1, "L", false, 0, "")
	}

	// ── Public holidays on weekdays ──────────────────────────────────────────
	pdf.Ln(3)
	pdf.SetX(gMargin)
	pdf.SetFont(ff, "B", 10)
	setTxt(pdf, gClrText)
	pdf.CellFormat(gBodyW, 6, lb.HolidaysUsed, "", 1, "L", false, 0, "")
	pdf.SetFont(ff, "", 8)
	if len(o.HolidayWeekdays) == 0 {
		pdf.SetX(gMargin)
		setTxt(pdf, gClrMuted)
		pdf.CellFormat(gBodyW, 5, lb.NoHolidays, "", 1, "L", false, 0, "")
	} else {
		// Three columns: "date  weekday · name".
		const cols = 3
		hw := gBodyW / cols
		for i, ds := range o.HolidayWeekdays {
			if i%cols == 0 {
				pdf.SetX(gMargin)
			}
			label := ds
			if d, err := time.Parse("2006-01-02", ds); err == nil {
				label = d.Format(dateLayout) + "  " + tr.DaysFull[(int(d.Weekday())+6)%7]
			}
			if name := o.HolidayNames[ds]; name != "" {
				label += " · " + name
			}
			setTxt(pdf, gClrText)
			ln := 0
			if i%cols == cols-1 || i == len(o.HolidayWeekdays)-1 {
				ln = 1
			}
			pdf.CellFormat(hw, 5, truncStr(label, 60), "", ln, "L", false, 0, "")
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return bytes.Buffer{}, err
	}
	return buf, nil
}
