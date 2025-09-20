package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/guttosm/b3pulse/internal/domain/models"
)

type stubRepo struct {
	agg *models.Aggregate
	err error
}

func (s *stubRepo) InsertTradesBatch(_ []models.Trade) error { return nil }
func (s *stubRepo) GetAggregateByTicker(_ string, _ *time.Time, _ *time.Time) (*models.Aggregate, error) {
	return s.agg, s.err
}
func (s *stubRepo) HasIngestionForDate(_ time.Time) (bool, error)         { return false, nil }
func (s *stubRepo) UpsertIngestionLog(_ time.Time, _ string, _ int) error { return nil }
func (s *stubRepo) DeleteTradesByDate(_ time.Time) error                  { return nil }

func TestAggregateService_TableDriven(t *testing.T) {
	cases := []struct {
		name    string
		repo    *stubRepo
		wantErr bool
	}{
		{
			name:    "success",
			repo:    &stubRepo{agg: &models.Aggregate{Ticker: "ABCD3", MaxRangeValue: 1.23, MaxDailyVolume: 45}},
			wantErr: false,
		},
		{
			name:    "error",
			repo:    &stubRepo{agg: nil, err: errors.New("boom")},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewAggregateService(tc.repo)
			out, err := svc.GetAggregate(context.Background(), "XXXX4", nil, nil)
			if tc.wantErr {
				if err == nil || out != nil {
					t.Fatalf("expected error, got out=%+v err=%v", out, err)
				}
			} else {
				if err != nil || out == nil {
					t.Fatalf("unexpected: out=%+v err=%v", out, err)
				}
			}
		})
	}
}
