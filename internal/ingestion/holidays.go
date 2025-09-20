package ingestion

import "time"

// LastNBusinessDays returns the last n Brazilian business days (most recent first).
// It excludes Saturdays, Sundays, and BR national/movable holidays.
func LastNBusinessDays(n int, from time.Time) []time.Time {
	out := make([]time.Time, 0, n)
	d := truncateToDate(from)

	for len(out) < n {
		if isBusinessDayBR(d) {
			out = append(out, d)
		}
		d = d.AddDate(0, 0, -1)
	}
	return out
}

func truncateToDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// isBusinessDayBR returns true if date is a business day in Brazil.
func isBusinessDayBR(d time.Time) bool {
	// Weekend
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}

	// National fixed holidays
	fixed := map[string]struct{}{
		"01-01": {}, // New Year
		"04-21": {}, // Tiradentes
		"05-01": {}, // Labor Day
		"09-07": {}, // Independence Day
		"10-12": {}, // Our Lady Aparecida
		"11-02": {}, // All Souls' Day
		"11-15": {}, // Republic Proclamation
		"12-25": {}, // Christmas
	}
	key := d.Format("01-02")
	if _, ok := fixed[key]; ok {
		return false
	}

	// Movable holidays (computed from Easter)
	y := d.Year()
	easter := easterSunday(y)

	// Carnival Monday & Tuesday (48/47 days before Easter Sunday)
	carnivalMon := easter.AddDate(0, 0, -48)
	carnivalTue := easter.AddDate(0, 0, -47)
	// Good Friday (2 days before Easter)
	goodFriday := easter.AddDate(0, 0, -2)
	// Corpus Christi (60 days after Easter)
	corpusChristi := easter.AddDate(0, 0, 60)

	movables := map[time.Time]struct{}{
		truncateToDate(carnivalMon):   {},
		truncateToDate(carnivalTue):   {},
		truncateToDate(goodFriday):    {},
		truncateToDate(corpusChristi): {},
	}
	if _, ok := movables[truncateToDate(d)]; ok {
		return false
	}

	return true
}

// easterSunday returns the date of Easter Sunday for a given year
// (Meeus/Jones/Butcher algorithm).
func easterSunday(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h + l - 7*m + 114) % 31) + 1

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
}
