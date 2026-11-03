package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/config"
)

func TestIsValidAvatarURL(t *testing.T) {
	valid := []string{"", "/uploads/abc123.png", "http://example.com/a.png", "https://example.com/a.png", "HTTPS://Example.com/A.png"}
	for _, u := range valid {
		assert.True(t, isValidAvatarURL(u), "expected %q to be valid", u)
	}

	invalid := []string{"javascript:alert(1)", "ftp://example.com/a.png", "not-a-url", "uploads/abc123.png"}
	for _, u := range invalid {
		assert.False(t, isValidAvatarURL(u), "expected %q to be invalid", u)
	}
}

func TestDeleteOldUpload(t *testing.T) {
	dir := t.TempDir()
	prevCfg := attachmentCfg
	attachmentCfg = &config.Config{UploadDir: dir}
	defer func() { attachmentCfg = prevCfg }()

	writeFile := func(name string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("x"), 0644))
		return path
	}

	t.Run("removes the old file when replaced by a different one", func(t *testing.T) {
		oldPath := writeFile("old1.png")
		deleteOldUpload("/uploads/old1.png", "/uploads/new1.png")
		_, err := os.Stat(oldPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("removes the old file when cleared", func(t *testing.T) {
		oldPath := writeFile("old2.png")
		deleteOldUpload("/uploads/old2.png", "")
		_, err := os.Stat(oldPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("no-op when the URL is unchanged", func(t *testing.T) {
		oldPath := writeFile("same.png")
		deleteOldUpload("/uploads/same.png", "/uploads/same.png")
		_, err := os.Stat(oldPath)
		assert.NoError(t, err)
	})

	t.Run("no-op when there was no previous URL", func(t *testing.T) {
		// Must not panic or attempt to delete anything for an empty old value.
		deleteOldUpload("", "/uploads/new.png")
	})

	t.Run("no-op for an external (non-local) old URL", func(t *testing.T) {
		deleteOldUpload("https://example.com/old.png", "/uploads/new.png")
	})

	t.Run("no-op when the old file no longer exists on disk", func(t *testing.T) {
		deleteOldUpload("/uploads/does-not-exist.png", "/uploads/new.png")
	})
}

func postImage(t *testing.T, name string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", name)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/upload/image", &body)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	UploadImage(c)
	return w
}

// A minimal valid 1x1 PNG.
var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0xf0,
	0x1f, 0x00, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99, 0x3d, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestUploadImage_AcceptsPNGByteForByte(t *testing.T) {
	dir := t.TempDir()
	prevCfg := attachmentCfg
	attachmentCfg = &config.Config{UploadDir: dir, MaxUploadMB: 5}
	defer func() { attachmentCfg = prevCfg }()

	w := postImage(t, "logo.png", tinyPNG)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		URL string `json:"url"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	saved, err := os.ReadFile(filepath.Join(dir, filepath.Base(resp.URL)))
	require.NoError(t, err)
	assert.Equal(t, tinyPNG, saved, "stored bytes must equal the uploaded bytes")
}

func TestUploadImage_RejectsMangledPNGAndReportsType(t *testing.T) {
	dir := t.TempDir()
	prevCfg := attachmentCfg
	attachmentCfg = &config.Config{UploadDir: dir, MaxUploadMB: 5}
	defer func() { attachmentCfg = prevCfg }()

	// What a UTF-8 round-trip does to a PNG: invalid bytes become U+FFFD.
	mangled := []byte(strings.ToValidUTF8(string(tinyPNG), "�"))
	w := postImage(t, "logo.png", mangled)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "got ")
	assert.NotContains(t, w.Body.String(), "got image/png")
}
