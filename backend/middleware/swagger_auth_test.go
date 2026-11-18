package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/tonk/warmdesk/services"
)

func TestSwaggerAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := services.NewAuthService("test-secret-key-that-is-long-enough-32")
	r := gin.New()
	r.GET("/swagger/index.html", SwaggerLoginRedirect(), Auth(svc), SwaggerAuth(), func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	token := func(role string) string {
		tok, err := svc.IssueAccessToken(1, "u", role)
		assert.NoError(t, err)
		return tok
	}
	do := func(accept, bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	w := do("text/html", "")
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
	assert.Equal(t, http.StatusUnauthorized, do("application/json", "").Code)
	assert.Equal(t, http.StatusOK, do("", token("admin")).Code)
	assert.Equal(t, http.StatusOK, do("", token("apidocs")).Code)
	for _, role := range []string{"user", "viewer", "metrics", "backup", "customer"} {
		assert.Equal(t, http.StatusForbidden, do("", token(role)).Code, role)
	}
}
