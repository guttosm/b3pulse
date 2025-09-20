package ingestion

import (
	"testing"
	"time"
)

func TestIsBusinessDayBR_WeekendsAndFixed(t *testing.T) {
	// Weekend
	if isBusinessDayBR(time.Date(2025, 9, 21, 0, 0, 0, 0, time.Local)) { // Sunday
		t.Fatal("Sunday should not be business day")
	}
	// Fixed holiday 07-Sep (Independence Day)
	if isBusinessDayBR(time.Date(2025, 9, 7, 0, 0, 0, 0, time.Local)) {
		t.Fatal("Sept 7 should not be business day")
	}
}

func TestLastNBusinessDays_CountAndOrder(t *testing.T) {
	from := time.Date(2025, 9, 20, 12, 30, 0, 0, time.Local) // Sat
	days := LastNBusinessDays(5, from)
	if len(days) != 5 {
		t.Fatalf("want 5 got %d", len(days))
	}
	// Ensure strictly decreasing dates and no weekends
	for i := 0; i < len(days); i++ {
		if i > 0 && !days[i].Before(days[i-1]) {
			t.Fatal("dates should be strictly decreasing")
		}
		wd := days[i].Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			t.Fatal("weekend day returned")
		}
	}
}
