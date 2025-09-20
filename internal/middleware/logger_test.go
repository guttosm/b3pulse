package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guttosm/b3pulse/internal/logger"
)

func TestToString(t *testing.T) {
	if s := toString(nil); s != "" {
		t.Fatalf("nil -> %q, want empty", s)
	}
	if s := toString("abc"); s != "abc" {
		t.Fatalf("string -> %q, want 'abc'", s)
	}
	if s := toString(123); s != "" {
		t.Fatalf("non-string -> %q, want empty", s)
	}
}

func TestRequestLogger_Basic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// capture logs by setting pretty off and ensuring logger initialized
	logger.Init()
	router.Use(RequestID())
	router.Use(RequestLogger())
	router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	if rid := w.Header().Get("X-Request-ID"); rid == "" {
		t.Fatalf("missing X-Request-ID header")
	}
}

func TestRequestLogger_ClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	logger.Init()
	router.Use(RequestID(), RequestLogger())
	router.POST("/echo", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, string(b))
	})

	body := bytes.NewBufferString("hello")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/echo", body)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
}
