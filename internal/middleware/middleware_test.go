package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("missing request id header")
	}
}

func TestErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ErrorHandler)
	r.GET("/", func(c *gin.Context) { _ = c.Error(assertErr{}) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != 500 {
		t.Fatalf("code=%d", w.Code)
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }

func TestRecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RecoveryMiddleware())
	r.GET("/panic", func(c *gin.Context) { panic("boom") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != 500 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestRateLimiter(t *testing.T) {
	cases := []struct {
		name   string
		reqs   int
		lim    int
		expect int
	}{
		{name: "within limit", reqs: 2, lim: 3, expect: http.StatusOK},
		{name: "exceed limit", reqs: 5, lim: 3, expect: http.StatusTooManyRequests},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			window = time.Millisecond * 100
			limit = tc.lim
			r.Use(RateLimiter())
			r.GET("/", func(c *gin.Context) { c.String(200, "ok") })
			var last int
			for i := 0; i < tc.reqs; i++ {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
				last = w.Code
			}
			if last != tc.expect {
				t.Fatalf("expected %d, got %d", tc.expect, last)
			}
		})
	}
}

func TestAbortWithError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/err", func(c *gin.Context) {
		AbortWithError(c, http.StatusBadRequest, "bad stuff", assertErr{})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/err", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct == "" {
		t.Fatalf("expected content-type set")
	}
}
