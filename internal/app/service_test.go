package app

import (
	"context"
	"testing"
	"time"

	"github.com/guttosm/b3pulse/internal/domain/models"
)

type fakeRepoForService struct{}

func (fakeRepoForService) InsertTradesBatch([]models.Trade) error { return nil }
func (fakeRepoForService) GetAggregateByTicker(t string, s, e *time.Time) (*models.Aggregate, error) {
	return &models.Aggregate{Ticker: t, MaxRangeValue: 1.23, MaxDailyVolume: 456}, nil
}
func (fakeRepoForService) HasIngestionForDate(time.Time) (bool, error)     { return false, nil }
func (fakeRepoForService) UpsertIngestionLog(time.Time, string, int) error { return nil }
func (fakeRepoForService) DeleteTradesByDate(time.Time) error              { return nil }

func TestAggregateService_DelegatesToRepo(t *testing.T) {
	svc := NewAggregateService(fakeRepoForService{})
	start := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
	out, err := svc.GetAggregate(context.Background(), "PETR4", &start, nil)
	if err != nil || out == nil {
		t.Fatalf("unexpected err=%v out=%v", err, out)
	}
	if out.Ticker != "PETR4" || out.MaxRangeValue != 1.23 || out.MaxDailyVolume != 456 {
		t.Fatalf("unexpected aggregate: %+v", out)
	}
}
