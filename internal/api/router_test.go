package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guttosm/b3pulse/internal/domain/models"
	"github.com/guttosm/b3pulse/internal/service"
)

// mockAggService implements service.AggregateService for testing router wiring
type mockAggServiceRouter struct {
	resp *models.Aggregate
	err  error
}

func (m *mockAggServiceRouter) GetAggregate(_ context.Context, _ string, _ *time.Time, _ *time.Time) (*models.Aggregate, error) {
	return m.resp, m.err
}

var _ service.AggregateService = (*mockAggServiceRouter)(nil)

func TestNewRouter_WiringAndMiddlewares(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Provide a service that returns a valid aggregate so handler returns 200
	svc := &mockAggServiceRouter{resp: &models.Aggregate{Ticker: "PETR4", MaxRangeValue: 12.3, MaxDailyVolume: 456}}
	h := NewHandler(svc)
	r := NewRouter(h)

	// Hit the aggregate route through the router created by NewRouter
	req := httptest.NewRequest(http.MethodGet, "/api/v1/aggregate?ticker=PETR4&data_inicio=2025-09-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Ensure RequestID middleware injected header
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("expected X-Request-ID header to be set")
	}

	// Ensure JSON body has the aggregate fields
	var out models.Aggregate
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if out.Ticker != "PETR4" || out.MaxRangeValue != 12.3 || out.MaxDailyVolume != 456 {
		t.Fatalf("unexpected body: %+v", out)
	}
}
