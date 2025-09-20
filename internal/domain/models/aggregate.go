package models

// Aggregate represents the result of aggregated queries
// over trades for a specific ticker.
//
// Fields:
//   - Ticker: The ticker symbol used in the aggregation (e.g., "VALE3").
//   - MaxRangeValue: The maximum unit price observed in the selected period.
//   - MaxDailyVolume: The maximum number of assets traded in a single day
//     during the selected period.
//
// This model is returned by the API when querying /api/v1/aggregate.
//
// swagger:model Aggregate
type Aggregate struct {
	Ticker         string  `json:"ticker" example:"PETR4"`
	MaxRangeValue  float64 `json:"max_range_value" example:"20.50"`
	MaxDailyVolume int64   `json:"max_daily_volume" example:"150000"`
}
