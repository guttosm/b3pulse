package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name    string
		pingErr bool
		path    string
		want    int
	}{
		{name: "healthz ok", pingErr: false, path: "/healthz", want: 200},
		{name: "readyz ok", pingErr: false, path: "/readyz", want: 200},
		{name: "readyz degraded", pingErr: true, path: "/readyz", want: 503},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ping func() error
			if tc.path == "/readyz" {
				if tc.pingErr {
					ping = func() error { return assertErr{} }
				} else {
					ping = func() error { return nil }
				}
			}

			r := gin.New()
			NewHealthHandler(ping).Register(r)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("want %d got %d", tc.want, w.Code)
			}
		})
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "err" }
