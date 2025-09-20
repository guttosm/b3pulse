package models

import "time"

// Trade represents a single row in the B3 business file.
// Each field matches one column in the .txt file.
//
// Column order:
//  1. ReferenceDate
//  2. InstrumentCode
//  3. UpdateAction
//  4. TradePrice
//  5. TradeQuantity
//  6. ClosingTime
//  7. TradeIdentifierCode
//  8. SessionType
//  9. TradeDate
//  10. BuyerParticipantCode
//  11. SellerParticipantCode
type Trade struct {
	ReferenceDate         time.Time
	InstrumentCode        string
	UpdateAction          string
	TradePrice            float64
	TradeQuantity         int64
	ClosingTime           time.Time
	TradeIdentifierCode   string
	SessionType           string
	TradeDate             time.Time
	BuyerParticipantCode  string
	SellerParticipantCode string
}
