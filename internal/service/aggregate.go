package service

import (
	"context"
	"time"

	"github.com/guttosm/b3pulse/internal/domain/models"
	"github.com/guttosm/b3pulse/internal/storage"
)

// AggregateService defines business logic for computing aggregates.
type AggregateService interface {
	GetAggregate(ctx context.Context, ticker string, startDate *time.Time, endDate *time.Time) (*models.Aggregate, error)
}

type aggregateService struct {
	repo storage.TradesRepository
}

func NewAggregateService(repo storage.TradesRepository) AggregateService {
	return &aggregateService{repo: repo}
}

func (s *aggregateService) GetAggregate(ctx context.Context, ticker string, startDate *time.Time, endDate *time.Time) (*models.Aggregate, error) {
	return s.repo.GetAggregateByTicker(ticker, startDate, endDate)
}
