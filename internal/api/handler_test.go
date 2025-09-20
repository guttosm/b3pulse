package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guttosm/b3pulse/internal/domain/models"
	"github.com/guttosm/b3pulse/internal/service"
)

type mockAggService struct {
	resp *models.Aggregate
	err  error
}

func (m *mockAggService) GetAggregate(_ context.Context, _ string, _ *time.Time, _ *time.Time) (*models.Aggregate, error) {
	return m.resp, m.err
}

var _ service.AggregateService = (*mockAggService)(nil)

func setupRouterWithMock(s service.AggregateService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(s)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.GET("/aggregate", h.GetAggregate)
	return r
}

func TestGetAggregate_TableDriven(t *testing.T) {
	cases := []struct {
		name   string
		svc    *mockAggService
		query  string
		status int
		assert func(t *testing.T, body []byte)
	}{
		{
			name:   "missing ticker",
			svc:    &mockAggService{},
			query:  "/api/v1/aggregate",
			status: http.StatusBadRequest,
		},
		{
			name:   "invalid date format",
			svc:    &mockAggService{},
			query:  "/api/v1/aggregate?ticker=PETR4&data_inicio=2025/09/01",
			status: http.StatusBadRequest,
		},
		{
			name:   "not found",
			svc:    &mockAggService{resp: nil, err: nil},
			query:  "/api/v1/aggregate?ticker=VALE3",
			status: http.StatusNotFound,
		},
		{
			name:   "internal error",
			svc:    &mockAggService{resp: nil, err: errors.New("db down")},
			query:  "/api/v1/aggregate?ticker=VALE3",
			status: http.StatusInternalServerError,
		},
		{
			name:   "success",
			svc:    &mockAggService{resp: &models.Aggregate{Ticker: "PETR4", MaxRangeValue: 10.5, MaxDailyVolume: 123}},
			query:  "/api/v1/aggregate?ticker=petr4&data_inicio=2025-09-01",
			status: http.StatusOK,
			assert: func(t *testing.T, body []byte) {
				var out models.Aggregate
				if err := json.Unmarshal(body, &out); err != nil {
					t.Fatalf("invalid json: %v", err)
				}
				if out.Ticker != "PETR4" || out.MaxRangeValue != 10.5 || out.MaxDailyVolume != 123 {
					t.Fatalf("unexpected body: %+v", out)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := setupRouterWithMock(tc.svc)
			req := httptest.NewRequest(http.MethodGet, tc.query, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("expected %d, got %d", tc.status, w.Code)
			}
			if tc.assert != nil {
				tc.assert(t, w.Body.Bytes())
			}
		})
	}
}
