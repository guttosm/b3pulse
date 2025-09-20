package dto

// AggregateResponse represents the JSON structure returned by the
// GET /api/v1/aggregate endpoint.
//
// Fields match the API contract and may differ from internal domain models.
// This ensures loose coupling between the API surface and business logic.
type AggregateResponse struct {
	Ticker         string  `json:"ticker" example:"PETR4"`            // Stock ticker requested
	MaxRangeValue  float64 `json:"max_range_value" example:"20.50"`   // Maximum price observed in the period
	MaxDailyVolume int64   `json:"max_daily_volume" example:"150000"` // Maximum daily traded volume in the period
}
