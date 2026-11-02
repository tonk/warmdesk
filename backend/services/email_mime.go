package services

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime/quotedprintable"
	"regexp"
	"strings"
)

// inlineImage is an image that was embedded in an HTML body as a data: URI and
// is sent as a separate MIME part referenced via cid: instead. Most mail
// clients (Gmail, Outlook, many webmail clients) refuse to render data: URIs in
// <img> tags, while cid: references to inline parts work everywhere — so this
// is what makes the company logo show up regardless of what the recipient's PC
// can reach.
type inlineImage struct {
	cid         string
	contentType string
	data        string // base64, without line breaks
}

// mailAttachment is a regular (non-inline) file attachment.
type mailAttachment struct {
	filename    string
	contentType string
	data        []byte
}

var dataURIImgRe = regexp.MustCompile(`(src=["'])data:(image/[a-zA-Z0-9.+-]+);base64,([A-Za-z0-9+/=\s]+)(["'])`)

// extractInlineImages replaces every data: image URI in htmlBody with a cid:
// reference and returns the images to attach. Identical images share a CID.
func extractInlineImages(htmlBody string) (string, []inlineImage) {
	var images []inlineImage
	seen := map[string]string{}
	out := dataURIImgRe.ReplaceAllStringFunc(htmlBody, func(m string) string {
		sub := dataURIImgRe.FindStringSubmatch(m)
		data := strings.Join(strings.Fields(sub[3]), "")
		key := sub[2] + ";" + data
		cid, ok := seen[key]
		if !ok {
			cid = fmt.Sprintf("wd-img-%d@warmdesk", len(images)+1)
			seen[key] = cid
			images = append(images, inlineImage{cid: cid, contentType: sub[2], data: data})
		}
		return sub[1] + "cid:" + cid + sub[4]
	})
	return out, images
}

func newBoundary(tag string) string {
	rb := make([]byte, 12)
	rand.Read(rb) //nolint:errcheck
	return "==WarmDesk_" + tag + "_" + hex.EncodeToString(rb) + "=="
}

func imageExt(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	default:
		return "png"
	}
}

// writeBase64 writes data as base64 in 76-character lines (RFC 2045).
func writeBase64(b *strings.Builder, encoded string) {
	for i := 0; i < len(encoded); i += 76 {
		end := min(i+76, len(encoded))
		b.WriteString(encoded[i:end])
		b.WriteString("\r\n")
	}
}

// writeTextPart writes a quoted-printable text part (headers + body).
// Quoted-printable keeps every line within the SMTP line-length limit without
// altering the content, unlike hard folding.
func writeTextPart(b *strings.Builder, mediaType, body string) {
	fmt.Fprintf(b, "Content-Type: %s; charset=UTF-8\r\n", mediaType)
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	var buf bytes.Buffer
	w := quotedprintable.NewWriter(&buf)
	w.Write([]byte(body)) //nolint:errcheck
	w.Close()             //nolint:errcheck
	b.Write(buf.Bytes())
	b.WriteString("\r\n")
}

// writeAlternative writes a multipart/alternative entity (Content-Type header
// included) with a plain-text and an HTML version. When the HTML contains
// data: images, the HTML version becomes a multipart/related entity carrying
// those images as inline parts.
func writeAlternative(b *strings.Builder, textBody, htmlBody string) {
	htmlBody, images := extractInlineImages(htmlBody)

	alt := newBoundary("alt")
	fmt.Fprintf(b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", alt)
	fmt.Fprintf(b, "--%s\r\n", alt)
	writeTextPart(b, "text/plain", textBody)
	fmt.Fprintf(b, "--%s\r\n", alt)
	if len(images) == 0 {
		writeTextPart(b, "text/html", htmlBody)
	} else {
		rel := newBoundary("rel")
		fmt.Fprintf(b, "Content-Type: multipart/related; type=\"text/html\"; boundary=%q\r\n\r\n", rel)
		fmt.Fprintf(b, "--%s\r\n", rel)
		writeTextPart(b, "text/html", htmlBody)
		for i, img := range images {
			fmt.Fprintf(b, "--%s\r\n", rel)
			fmt.Fprintf(b, "Content-Type: %s\r\n", img.contentType)
			b.WriteString("Content-Transfer-Encoding: base64\r\n")
			fmt.Fprintf(b, "Content-ID: <%s>\r\n", img.cid)
			fmt.Fprintf(b, "Content-Disposition: inline; filename=\"image%d.%s\"\r\n\r\n", i+1, imageExt(img.contentType))
			writeBase64(b, img.data)
		}
		fmt.Fprintf(b, "--%s--\r\n", rel)
	}
	fmt.Fprintf(b, "--%s--\r\n", alt)
}

// buildMIMEMessage assembles a complete RFC 5322 message. headers holds the
// top-level headers (From, To, Subject, …) without MIME headers. The body is a
// multipart/alternative (text + HTML with inline images), wrapped in
// multipart/mixed when att is non-nil.
func buildMIMEMessage(headers []string, textBody, htmlBody string, att *mailAttachment) string {
	var b strings.Builder
	for _, h := range headers {
		b.WriteString(h)
		b.WriteString("\r\n")
	}
	b.WriteString("MIME-Version: 1.0\r\n")

	if att == nil {
		writeAlternative(&b, textBody, htmlBody)
		return b.String()
	}

	mixed := newBoundary("mixed")
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", mixed)
	fmt.Fprintf(&b, "--%s\r\n", mixed)
	writeAlternative(&b, textBody, htmlBody)
	fmt.Fprintf(&b, "--%s\r\n", mixed)
	fmt.Fprintf(&b, "Content-Type: %s\r\n", att.contentType)
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	fmt.Fprintf(&b, "Content-Disposition: attachment; filename=%q\r\n\r\n", att.filename)
	writeBase64(&b, base64.StdEncoding.EncodeToString(att.data))
	fmt.Fprintf(&b, "--%s--\r\n", mixed)
	return b.String()
}
