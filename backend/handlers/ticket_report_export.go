package handlers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/xuri/excelize/v2"
)

// ticketReportI18n holds the labels of the ticket KPI report exports.
type ticketReportI18n struct {
	Title, Period, KeyFigures                              string
	Created, Resolved, Backlog, MajorIncidents             string
	MTTR, FirstResponse, RespSLA, ResSLA, FCR, Reopen      string
	Ageing, ByPriority, ByType, ByCategory, ByCustomer     string
	ColName, ColTickets, ColResolved, ColBreached, ColMTTR string
	Uncategorised, Inbox                                   string
	Critical, High, Medium, Low                            string
	Incident, Problem, ServiceRequest, ChangeRequest       string
	Age0, Age1, Age3, Age7                                 string
}

var ticketReportTranslations = map[string]ticketReportI18n{
	"en": {"Ticket report", "Period", "Key figures", "Created", "Resolved", "Open backlog", "Major incidents",
		"Mean time to resolve", "Mean time to first response", "Response SLA met", "Resolution SLA met", "First-contact resolution", "Reopen rate",
		"Backlog age", "By priority", "By type", "By category", "By customer",
		"Name", "Tickets", "Resolved", "SLA breached", "MTTR", "Uncategorised", "Inbox",
		"Critical", "High", "Medium", "Low", "Incident", "Problem", "Service request", "Change request",
		"< 1 day", "1–3 days", "3–7 days", "> 7 days"},
	"nl": {"Ticketrapport", "Periode", "Kerncijfers", "Aangemaakt", "Opgelost", "Openstaand", "Grote incidenten",
		"Gemiddelde oplostijd", "Gemiddelde tijd tot eerste reactie", "Reactie-SLA gehaald", "Oplos-SLA gehaald", "Opgelost bij eerste contact", "Heropend",
		"Leeftijd openstaand", "Per prioriteit", "Per type", "Per categorie", "Per klant",
		"Naam", "Tickets", "Opgelost", "SLA overschreden", "Oplostijd", "Zonder categorie", "Inbox",
		"Kritiek", "Hoog", "Gemiddeld", "Laag", "Incident", "Probleem", "Serviceverzoek", "Wijzigingsverzoek",
		"< 1 dag", "1–3 dagen", "3–7 dagen", "> 7 dagen"},
	"de": {"Ticketbericht", "Zeitraum", "Kennzahlen", "Erstellt", "Gelöst", "Offener Rückstand", "Major Incidents",
		"Mittlere Lösungszeit", "Mittlere Zeit bis zur ersten Antwort", "Reaktions-SLA erfüllt", "Lösungs-SLA erfüllt", "Lösung beim Erstkontakt", "Wiedereröffnungsrate",
		"Alter des Rückstands", "Nach Priorität", "Nach Typ", "Nach Kategorie", "Nach Kunde",
		"Name", "Tickets", "Gelöst", "SLA verletzt", "MTTR", "Ohne Kategorie", "Posteingang",
		"Kritisch", "Hoch", "Mittel", "Niedrig", "Störung", "Problem", "Serviceanfrage", "Änderungsanfrage",
		"< 1 Tag", "1–3 Tage", "3–7 Tage", "> 7 Tage"},
	"fr": {"Rapport des tickets", "Période", "Chiffres clés", "Créés", "Résolus", "En attente", "Incidents majeurs",
		"Délai moyen de résolution", "Délai moyen de première réponse", "SLA de réponse respecté", "SLA de résolution respecté", "Résolution au premier contact", "Taux de réouverture",
		"Ancienneté en attente", "Par priorité", "Par type", "Par catégorie", "Par client",
		"Nom", "Tickets", "Résolus", "SLA dépassé", "MTTR", "Sans catégorie", "Boîte de réception",
		"Critique", "Haute", "Moyenne", "Basse", "Incident", "Problème", "Demande de service", "Demande de changement",
		"< 1 jour", "1–3 jours", "3–7 jours", "> 7 jours"},
	"es": {"Informe de tickets", "Periodo", "Cifras clave", "Creados", "Resueltos", "Pendientes", "Incidentes graves",
		"Tiempo medio de resolución", "Tiempo medio hasta la primera respuesta", "SLA de respuesta cumplido", "SLA de resolución cumplido", "Resolución en el primer contacto", "Tasa de reapertura",
		"Antigüedad pendiente", "Por prioridad", "Por tipo", "Por categoría", "Por cliente",
		"Nombre", "Tickets", "Resueltos", "SLA incumplido", "MTTR", "Sin categoría", "Bandeja de entrada",
		"Crítica", "Alta", "Media", "Baja", "Incidente", "Problema", "Solicitud de servicio", "Solicitud de cambio",
		"< 1 día", "1–3 días", "3–7 días", "> 7 días"},
	"da": {"Ticketrapport", "Periode", "Nøgletal", "Oprettet", "Løst", "Åbne", "Større hændelser",
		"Gennemsnitlig løsningstid", "Gennemsnitlig tid til første svar", "Svar-SLA overholdt", "Løsnings-SLA overholdt", "Løst ved første kontakt", "Genåbningsrate",
		"Alder af åbne", "Efter prioritet", "Efter type", "Efter kategori", "Efter kunde",
		"Navn", "Tickets", "Løst", "SLA overskredet", "MTTR", "Uden kategori", "Indbakke",
		"Kritisk", "Høj", "Middel", "Lav", "Hændelse", "Problem", "Serviceanmodning", "Ændringsanmodning",
		"< 1 dag", "1–3 dage", "3–7 dage", "> 7 dage"},
	"sv": {"Ärenderapport", "Period", "Nyckeltal", "Skapade", "Lösta", "Öppna", "Större incidenter",
		"Genomsnittlig lösningstid", "Genomsnittlig tid till första svar", "Svars-SLA uppfyllt", "Lösnings-SLA uppfyllt", "Löst vid första kontakt", "Återöppningsgrad",
		"Ålder på öppna", "Per prioritet", "Per typ", "Per kategori", "Per kund",
		"Namn", "Ärenden", "Lösta", "SLA överskridet", "MTTR", "Utan kategori", "Inkorg",
		"Kritisk", "Hög", "Medel", "Låg", "Incident", "Problem", "Tjänsteförfrågan", "Ändringsförfrågan",
		"< 1 dag", "1–3 dagar", "3–7 dagar", "> 7 dagar"},
	"nb": {"Saksrapport", "Periode", "Nøkkeltall", "Opprettet", "Løst", "Åpne", "Store hendelser",
		"Gjennomsnittlig løsningstid", "Gjennomsnittlig tid til første svar", "Svar-SLA oppfylt", "Løsnings-SLA oppfylt", "Løst ved første kontakt", "Gjenåpningsrate",
		"Alder på åpne", "Etter prioritet", "Etter type", "Etter kategori", "Etter kunde",
		"Navn", "Saker", "Løst", "SLA brutt", "MTTR", "Uten kategori", "Innboks",
		"Kritisk", "Høy", "Middels", "Lav", "Hendelse", "Problem", "Tjenesteforespørsel", "Endringsforespørsel",
		"< 1 dag", "1–3 dager", "3–7 dager", "> 7 dager"},
	"fi": {"Tikettiraportti", "Ajanjakso", "Tunnusluvut", "Luotu", "Ratkaistu", "Avoimet", "Suuret häiriöt",
		"Keskimääräinen ratkaisuaika", "Keskimääräinen aika ensimmäiseen vastaukseen", "Vaste-SLA täyttyi", "Ratkaisu-SLA täyttyi", "Ratkaistu ensikontaktissa", "Uudelleenavausaste",
		"Avoimien ikä", "Prioriteetin mukaan", "Tyypin mukaan", "Luokan mukaan", "Asiakkaan mukaan",
		"Nimi", "Tiketit", "Ratkaistu", "SLA ylittyi", "MTTR", "Ei luokkaa", "Saapuneet",
		"Kriittinen", "Korkea", "Keskitaso", "Matala", "Häiriö", "Ongelma", "Palvelupyyntö", "Muutospyyntö",
		"< 1 päivä", "1–3 päivää", "3–7 päivää", "> 7 päivää"},
	"is": {"Beiðnaskýrsla", "Tímabil", "Lykiltölur", "Stofnaðar", "Leystar", "Opnar", "Stór atvik",
		"Meðaltími til lausnar", "Meðaltími til fyrsta svars", "Svar-SLA uppfyllt", "Lausnar-SLA uppfyllt", "Leyst við fyrstu snertingu", "Enduropnunarhlutfall",
		"Aldur opinna", "Eftir forgangi", "Eftir tegund", "Eftir flokki", "Eftir viðskiptavini",
		"Heiti", "Beiðnir", "Leystar", "SLA brotið", "MTTR", "Án flokks", "Innhólf",
		"Bráður", "Hár", "Miðlungs", "Lágur", "Atvik", "Vandamál", "Þjónustubeiðni", "Breytingabeiðni",
		"< 1 dagur", "1–3 dagar", "3–7 dagar", "> 7 dagar"},
	"pt": {"Relatório de tickets", "Período", "Indicadores", "Criados", "Resolvidos", "Pendentes", "Incidentes graves",
		"Tempo médio de resolução", "Tempo médio até à primeira resposta", "SLA de resposta cumprido", "SLA de resolução cumprido", "Resolução no primeiro contacto", "Taxa de reabertura",
		"Antiguidade pendente", "Por prioridade", "Por tipo", "Por categoria", "Por cliente",
		"Nome", "Tickets", "Resolvidos", "SLA violado", "MTTR", "Sem categoria", "Caixa de entrada",
		"Crítica", "Alta", "Média", "Baixa", "Incidente", "Problema", "Pedido de serviço", "Pedido de alteração",
		"< 1 dia", "1–3 dias", "3–7 dias", "> 7 dias"},
	"it": {"Report dei ticket", "Periodo", "Indicatori chiave", "Creati", "Risolti", "In sospeso", "Incidenti gravi",
		"Tempo medio di risoluzione", "Tempo medio alla prima risposta", "SLA di risposta rispettato", "SLA di risoluzione rispettato", "Risoluzione al primo contatto", "Tasso di riapertura",
		"Anzianità in sospeso", "Per priorità", "Per tipo", "Per categoria", "Per cliente",
		"Nome", "Ticket", "Risolti", "SLA violato", "MTTR", "Senza categoria", "Posta in arrivo",
		"Critica", "Alta", "Media", "Bassa", "Incidente", "Problema", "Richiesta di servizio", "Richiesta di modifica",
		"< 1 giorno", "1–3 giorni", "3–7 giorni", "> 7 giorni"},
}

func ticketReportI18nFor(lang string) ticketReportI18n {
	if tr, ok := ticketReportTranslations[lang]; ok {
		return tr
	}
	return ticketReportTranslations["en"]
}

// bucketLabel translates priority/type keys and fills in the "no category" /
// "inbox" placeholders.
func (tr ticketReportI18n) bucketLabel(kind string, b ticketReportBucket) string {
	names := map[string]string{
		"critical": tr.Critical, "high": tr.High, "medium": tr.Medium, "low": tr.Low,
		"incident": tr.Incident, "problem": tr.Problem, "service_request": tr.ServiceRequest, "change_request": tr.ChangeRequest,
	}
	switch kind {
	case "priority", "type":
		if n, ok := names[b.Key]; ok {
			return n
		}
	case "category":
		if b.Key == "" {
			return tr.Uncategorised
		}
	case "customer":
		if b.Key == "" {
			return tr.Inbox
		}
	}
	return b.Label
}

func fmtOptMinutes(m *int) string {
	if m == nil {
		return "—"
	}
	if *m >= 24*60 {
		return fmt.Sprintf("%dd %dh", *m/(24*60), (*m%(24*60))/60)
	}
	return fmtMinutes(*m)
}

func fmtOptPct(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", *p)
}

func (r *ticketReport) keyFigures(tr ticketReportI18n) [][2]string {
	return [][2]string{
		{tr.Created, fmt.Sprint(r.Created)},
		{tr.Resolved, fmt.Sprint(r.Resolved)},
		{tr.Backlog, fmt.Sprint(r.Backlog)},
		{tr.MajorIncidents, fmt.Sprint(r.MajorIncidents)},
		{tr.MTTR, fmtOptMinutes(r.MTTRMinutes)},
		{tr.FirstResponse, fmtOptMinutes(r.MeanFirstResponseMinutes)},
		{tr.RespSLA, fmtOptPct(r.ResponseCompliance)},
		{tr.ResSLA, fmtOptPct(r.ResolutionCompliance)},
		{tr.FCR, fmtOptPct(r.FCRRate)},
		{tr.Reopen, fmtOptPct(r.ReopenRate)},
	}
}

func (r *ticketReport) breakdowns(tr ticketReportI18n) []struct {
	kind, title string
	rows        []ticketReportBucket
} {
	return []struct {
		kind, title string
		rows        []ticketReportBucket
	}{
		{"priority", tr.ByPriority, r.ByPriority},
		{"type", tr.ByType, r.ByType},
		{"category", tr.ByCategory, r.ByCategory},
		{"customer", tr.ByCustomer, r.ByCustomer},
	}
}

func ticketReportFilename(r *ticketReport, ext string) string {
	return fmt.Sprintf("ticket-report-%s-%s.%s", r.From, r.To, ext)
}

func sendTicketReportFile(c *gin.Context, data []byte, filename, mime string) {
	if c.Query("base64") == "1" {
		c.JSON(http.StatusOK, gin.H{"data": base64.StdEncoding.EncodeToString(data)})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, mime, data)
}

// GetTicketReportPDF GET /api/v1/helpdesk/reports/tickets/pdf?lang=…&font=…
func GetTicketReportPDF(c *gin.Context) {
	if requireNotCustomerRole(middleware.GetGlobalRole(c)) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	r, status, msg := assembleTicketReport(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	var user models.User
	database.DB.First(&user, middleware.GetUserID(c))
	fontFamily := "FreeSans"
	if user.Font != "" {
		fontFamily = pdfFontFamily(user.Font)
	}
	if fam, ok := pdfFontFromParam(c.Query("font")); ok {
		fontFamily = fam
	}
	tr := ticketReportI18nFor(c.Query("lang"))
	generated := pdfI18nFromLang(c.Query("lang"))

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AliasNbPages("{nb}")
	pdf.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	pdf.SetAutoPageBreak(true, 20)
	for fam, files := range pdfFonts {
		pdf.AddUTF8FontFromBytes(fam, "", mustFont(files[0]))
		pdf.AddUTF8FontFromBytes(fam, "B", mustFont(files[1]))
	}
	company := r.CompanyName
	if company == "" {
		company = "WarmDesk"
	}
	period := r.From + " – " + r.To
	pdf.SetTitle(tr.Title+" — "+period, true)
	pdf.SetCreator(company, true)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont(fontFamily, "", 8)
		setTxt(pdf, clrMuted)
		pdf.CellFormat(pdfBodyW/2, 5, company+" — "+tr.Title, "", 0, "L", false, 0, "")
		pdf.CellFormat(pdfBodyW/2, 5, fmt.Sprintf("%s %d / {nb}", generated.Page, pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()

	// Header: logo, company, title, period.
	logoLoaded := false
	if raw, ext, ok := resolveLogoBytes(r.CompanyLogo); ok {
		logoLoaded = renderLogoIntoPDF(pdf, raw, ext, pdfMargin, pdfMargin)
	}
	textX := pdfMargin
	if logoLoaded {
		textX = pdfMargin + 32
	}
	textW := pdfPageW - pdfMargin - textX
	pdf.SetXY(textX, pdfMargin)
	if r.CompanyName != "" {
		pdf.SetFont(fontFamily, "B", 13)
		setTxt(pdf, clrPrimary)
		pdf.CellFormat(textW, 7, r.CompanyName, "", 2, "L", false, 0, "")
		pdf.SetX(textX)
	}
	pdf.SetFont(fontFamily, "B", 17)
	setTxt(pdf, clrText)
	pdf.CellFormat(textW, 9, tr.Title, "", 2, "L", false, 0, "")
	pdf.SetX(textX)
	pdf.SetFont(fontFamily, "", 10)
	setTxt(pdf, clrMuted)
	pdf.CellFormat(textW, 5.5, tr.Period+": "+period, "", 2, "L", false, 0, "")
	pdf.SetX(textX)
	pdf.SetFont(fontFamily, "", 8)
	pdf.CellFormat(textW, 4.5, generated.Generated+" "+r.GeneratedAt, "", 2, "L", false, 0, "")
	ruleY := pdf.GetY() + 2
	if logoLoaded && ruleY < pdfMargin+22 {
		ruleY = pdfMargin + 22
	}
	setDraw(pdf, clrPrimary)
	pdf.SetLineWidth(0.4)
	pdf.Line(pdfMargin, ruleY, pdfPageW-pdfMargin, ruleY)
	pdf.SetY(ruleY + 5)

	section := func(title string) {
		pdf.Ln(2)
		setFill(pdf, clrPrimary)
		setTxt(pdf, rgb{255, 255, 255})
		pdf.SetFont(fontFamily, "B", 10)
		pdf.CellFormat(pdfBodyW, 7, "  "+title, "", 1, "L", true, 0, "")
		setTxt(pdf, clrText)
		pdf.Ln(1)
	}
	pair := func(label, value string) {
		pdf.SetFont(fontFamily, "", 10)
		pdf.CellFormat(pdfBodyW*0.6, 6, label, "B", 0, "L", false, 0, "")
		pdf.SetFont(fontFamily, "B", 10)
		pdf.CellFormat(pdfBodyW*0.4, 6, value, "B", 1, "R", false, 0, "")
	}
	setDraw(pdf, clrMuted)
	pdf.SetLineWidth(0.1)

	section(tr.KeyFigures)
	for _, kv := range r.keyFigures(tr) {
		pair(kv[0], kv[1])
	}
	section(tr.Ageing)
	for i, label := range []string{tr.Age0, tr.Age1, tr.Age3, tr.Age7} {
		pair(label, fmt.Sprint(r.Ageing[i].Count))
	}

	widths := []float64{pdfBodyW * 0.40, pdfBodyW * 0.15, pdfBodyW * 0.15, pdfBodyW * 0.15, pdfBodyW * 0.15}
	for _, bd := range r.breakdowns(tr) {
		if len(bd.rows) == 0 {
			continue
		}
		section(bd.title)
		pdf.SetFont(fontFamily, "B", 9)
		for i, h := range []string{tr.ColName, tr.ColTickets, tr.ColResolved, tr.ColBreached, tr.ColMTTR} {
			align := "R"
			if i == 0 {
				align = "L"
			}
			pdf.CellFormat(widths[i], 6, h, "B", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetFont(fontFamily, "", 9)
		for _, b := range bd.rows {
			cells := []string{truncStr(tr.bucketLabel(bd.kind, b), 48), fmt.Sprint(b.Count), fmt.Sprint(b.Resolved), fmt.Sprint(b.Breached), fmtOptMinutes(b.MTTRMinutes)}
			for i, v := range cells {
				align := "R"
				if i == 0 {
					align = "L"
				}
				pdf.CellFormat(widths[i], 5.5, v, "B", 0, align, false, 0, "")
			}
			pdf.Ln(-1)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pdf generation failed"})
		return
	}
	sendTicketReportFile(c, buf.Bytes(), ticketReportFilename(r, "pdf"), "application/pdf")
}

// GetTicketReportXLSX GET /api/v1/helpdesk/reports/tickets/xlsx?lang=…
func GetTicketReportXLSX(c *gin.Context) {
	if requireNotCustomerRole(middleware.GetGlobalRole(c)) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	r, status, msg := assembleTicketReport(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	tr := ticketReportI18nFor(c.Query("lang"))

	f := excelize.NewFile()
	defer f.Close()
	sheet := tr.Title
	if len([]rune(sheet)) > 31 {
		sheet = string([]rune(sheet)[:31])
	}
	f.SetSheetName("Sheet1", sheet)
	bold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})

	row := 1
	title := tr.Title
	if r.CompanyName != "" {
		title = r.CompanyName + " — " + tr.Title
	}
	f.SetCellValue(sheet, cell(1, row), title)
	f.SetCellStyle(sheet, cell(1, row), cell(1, row), bold)
	row++
	f.SetCellValue(sheet, cell(1, row), tr.Period+": "+r.From+" – "+r.To)
	row += 2

	heading := func(s string) {
		f.SetCellValue(sheet, cell(1, row), s)
		f.SetCellStyle(sheet, cell(1, row), cell(1, row), bold)
		row++
	}
	heading(tr.KeyFigures)
	for _, kv := range r.keyFigures(tr) {
		f.SetCellValue(sheet, cell(1, row), kv[0])
		f.SetCellValue(sheet, cell(2, row), kv[1])
		row++
	}
	row++
	heading(tr.Ageing)
	for i, label := range []string{tr.Age0, tr.Age1, tr.Age3, tr.Age7} {
		f.SetCellValue(sheet, cell(1, row), label)
		f.SetCellValue(sheet, cell(2, row), r.Ageing[i].Count)
		row++
	}
	for _, bd := range r.breakdowns(tr) {
		if len(bd.rows) == 0 {
			continue
		}
		row++
		heading(bd.title)
		for i, h := range []string{tr.ColName, tr.ColTickets, tr.ColResolved, tr.ColBreached, tr.ColMTTR + " (min)"} {
			f.SetCellValue(sheet, cell(i+1, row), h)
		}
		f.SetCellStyle(sheet, cell(1, row), cell(5, row), bold)
		row++
		for _, b := range bd.rows {
			f.SetCellValue(sheet, cell(1, row), tr.bucketLabel(bd.kind, b))
			f.SetCellValue(sheet, cell(2, row), b.Count)
			f.SetCellValue(sheet, cell(3, row), b.Resolved)
			f.SetCellValue(sheet, cell(4, row), b.Breached)
			if b.MTTRMinutes != nil {
				f.SetCellValue(sheet, cell(5, row), *b.MTTRMinutes)
			}
			row++
		}
	}
	f.SetColWidth(sheet, "A", "A", 40)
	f.SetColWidth(sheet, "B", "E", 16)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "xlsx generation failed"})
		return
	}
	sendTicketReportFile(c, buf.Bytes(), ticketReportFilename(r, "xlsx"), xlsxMIME)
}
