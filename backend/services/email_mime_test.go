package services

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func TestExtractInlineImages(t *testing.T) {
	in := `<img src="data:image/png;base64,` + testPNG + `"><img src='data:image/png;base64,` + testPNG + `'>` +
		`<img src="https://example.com/x.png">`
	out, imgs := extractInlineImages(in)
	require.Len(t, imgs, 1, "identical images share one part")
	assert.Equal(t, "image/png", imgs[0].contentType)
	assert.Equal(t, testPNG, imgs[0].data)
	assert.NotContains(t, out, "data:")
	assert.Equal(t, 2, strings.Count(out, "cid:"+imgs[0].cid))
	assert.Contains(t, out, `src="https://example.com/x.png"`)
}

type parsedPart struct {
	mediaType string
	header    map[string][]string
	body      []byte
}

// walk flattens a MIME tree into its leaf parts, decoding transfer encodings.
func walk(t *testing.T, ct string, cte string, r io.Reader, hdr map[string][]string, out *[]parsedPart) {
	mt, params, err := mime.ParseMediaType(ct)
	require.NoError(t, err)
	if strings.HasPrefix(mt, "multipart/") {
		mr := multipart.NewReader(r, params["boundary"])
		for {
			p, err := mr.NextRawPart()
			if err == io.EOF {
				return
			}
			require.NoError(t, err)
			walk(t, p.Header.Get("Content-Type"), p.Header.Get("Content-Transfer-Encoding"), p, p.Header, out)
		}
	}
	switch strings.ToLower(cte) {
	case "quoted-printable":
		r = quotedprintable.NewReader(r)
	case "base64":
		r = base64.NewDecoder(base64.StdEncoding, r)
	}
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	*out = append(*out, parsedPart{mediaType: mt, header: hdr, body: b})
}

func parseMessage(t *testing.T, raw string) (*mail.Message, []parsedPart) {
	m, err := mail.ReadMessage(strings.NewReader(raw))
	require.NoError(t, err)
	var parts []parsedPart
	walk(t, m.Header.Get("Content-Type"), m.Header.Get("Content-Transfer-Encoding"), m.Body, m.Header, &parts)
	return m, parts
}

func TestBuildMIMEMessage_inlineLogo(t *testing.T) {
	SetAppInfoReader(func() (string, string, string, string) {
		return "v1.0.0", "Acme", "data:image/png;base64," + testPNG, ""
	})
	defer resetAppInfoReader()

	long := strings.Repeat("word ", 400) // longer than an SMTP line
	raw := buildMIMEMessage([]string{"From: a@b.c", "To: d@e.f", "Subject: Hi"},
		"plain "+long, WrapHTML("Sub", "<tr><td>"+long+"</td></tr>"), nil)

	for _, line := range strings.Split(raw, "\r\n") {
		assert.LessOrEqual(t, len(line), 998)
	}

	m, parts := parseMessage(t, raw)
	assert.Equal(t, "1.0", m.Header.Get("MIME-Version"))
	require.Len(t, parts, 4) // text, html, company logo, WarmDesk icon

	assert.Equal(t, "text/plain", parts[0].mediaType)
	assert.Equal(t, "plain "+long, string(parts[0].body))

	assert.Equal(t, "text/html", parts[1].mediaType)
	htmlBody := string(parts[1].body)
	assert.NotContains(t, htmlBody, "data:image")
	assert.Contains(t, htmlBody, long)

	logo := parts[2]
	assert.Equal(t, "image/png", logo.mediaType)
	pngBytes, _ := base64.StdEncoding.DecodeString(testPNG)
	assert.Equal(t, pngBytes, logo.body)
	cid := strings.Trim(mail.Header(logo.header).Get("Content-Id"), "<>")
	assert.Contains(t, htmlBody, `src="cid:`+cid+`"`)
	assert.True(t, strings.HasPrefix(mail.Header(logo.header).Get("Content-Disposition"), "inline"))

	assert.Equal(t, "image/svg+xml", parts[3].mediaType)
}

func TestBuildMIMEMessage_noImages(t *testing.T) {
	raw := buildMIMEMessage([]string{"Subject: x"}, "text", "<p>html</p>", nil)
	assert.NotContains(t, raw, "multipart/related")
	_, parts := parseMessage(t, raw)
	require.Len(t, parts, 2)
	assert.Equal(t, "<p>html</p>", string(parts[1].body))
}

func TestBuildMIMEMessage_withAttachment(t *testing.T) {
	resetAppInfoReader()
	raw := buildMIMEMessage([]string{"Subject: x"}, "text", WrapHTML("Invoice", ""),
		&mailAttachment{filename: "INV-1.pdf", contentType: "application/pdf", data: []byte("%PDF-1.4")})
	m, parts := parseMessage(t, raw)
	assert.True(t, strings.HasPrefix(m.Header.Get("Content-Type"), "multipart/mixed"))
	last := parts[len(parts)-1]
	assert.Equal(t, "application/pdf", last.mediaType)
	assert.Equal(t, "%PDF-1.4", string(last.body))
	assert.Contains(t, mail.Header(last.header).Get("Content-Disposition"), "INV-1.pdf")
}

func TestTicketReply_plainTextStillReadableByIMAP(t *testing.T) {
	resetAppInfoReader()
	body := "Hello,\n\nYour issue <is> fixed.\n\nRegards"
	raw := buildMIMEMessage([]string{"Subject: Re: [#7] x"}, body, ticketReplyHTML(7, body), nil)
	m, err := mail.ReadMessage(strings.NewReader(raw))
	require.NoError(t, err)
	text, err := extractPlainText(m)
	require.NoError(t, err)
	assert.Equal(t, strings.ReplaceAll(body, "\n", "\r\n"), text)

	_, parts := parseMessage(t, raw)
	assert.Contains(t, string(parts[1].body), "Your issue &lt;is&gt; fixed.")
	assert.Contains(t, string(parts[1].body), "Ticket #7")
}
