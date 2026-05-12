// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"fmt"
	"strings"
)

// Calendar enumerates calendar systems supported by Linea time
// ranges (CCGGS §11.2). The default is GregorianProleptic.
type Calendar string

const (
	// CalendarGregorianProleptic is the default calendar (proleptic
	// Gregorian, i.e. Gregorian extended back before its adoption).
	CalendarGregorianProleptic Calendar = "gregorian-proleptic"
	// CalendarJulian is the Julian calendar.
	CalendarJulian Calendar = "julian"
	// CalendarHijri is the Islamic Hijri calendar.
	CalendarHijri Calendar = "hijri"
)

// IsKnown reports whether c is a calendar value Linea recognises
// out of the box. Unknown calendars are permitted but MUST be
// documented by the deploying system.
func (c Calendar) IsKnown() bool {
	switch c {
	case CalendarGregorianProleptic, CalendarJulian, CalendarHijri:
		return true
	}
	return c != "" // any non-empty string is structurally valid
}

// YearBound represents one bound (earliest or latest) of a date
// range. A bound with KnownYear == false represents "Unknown".
//
// Years are signed integers in the chosen Calendar's year
// numbering. A value of 1 means year 1 in that calendar; 0 and
// negative values are permitted (proleptic / BCE / pre-epoch).
type YearBound struct {
	KnownYear bool
	Year      int
}

// UnknownYear returns a YearBound representing "Unknown".
func UnknownYear() YearBound { return YearBound{} }

// KnownYearBound returns a YearBound for the given year value.
func KnownYearBound(y int) YearBound { return YearBound{KnownYear: true, Year: y} }

// String renders the bound for diagnostics.
func (b YearBound) String() string {
	if !b.KnownYear {
		return "Unknown"
	}
	return fmt.Sprintf("%d", b.Year)
}

// TimeRange is an inclusive [Earliest, Latest] year range with an
// optional Circa flag, expressed in the given Calendar.
//
// Either bound may be UnknownYear(); when both are known, Earliest
// must be <= Latest.
type TimeRange struct {
	Earliest YearBound
	Latest   YearBound
	Calendar Calendar
	// Circa marks the range as an explicit approximation rather
	// than a documented hard range.
	Circa bool
}

// NewTimeRange constructs a validated TimeRange. An empty Calendar
// is normalised to CalendarGregorianProleptic.
func NewTimeRange(earliest, latest YearBound, cal Calendar, circa bool) (TimeRange, error) {
	if cal == "" {
		cal = CalendarGregorianProleptic
	}
	if !cal.IsKnown() {
		return TimeRange{}, fmt.Errorf("model: invalid calendar %q", cal)
	}
	if earliest.KnownYear && latest.KnownYear && earliest.Year > latest.Year {
		return TimeRange{}, fmt.Errorf(
			"model: time range earliest (%d) is after latest (%d)",
			earliest.Year, latest.Year,
		)
	}
	return TimeRange{
		Earliest: earliest,
		Latest:   latest,
		Calendar: cal,
		Circa:    circa,
	}, nil
}

// IsZero reports whether the range carries no information at all
// (both bounds Unknown, no calendar, not circa).
func (r TimeRange) IsZero() bool {
	return !r.Earliest.KnownYear && !r.Latest.KnownYear && r.Calendar == "" && !r.Circa
}

// String renders the range for diagnostics.
func (r TimeRange) String() string {
	var b strings.Builder
	if r.Circa {
		b.WriteString("c. ")
	}
	b.WriteString(r.Earliest.String())
	if r.Earliest != r.Latest {
		b.WriteString("–")
		b.WriteString(r.Latest.String())
	}
	if r.Calendar != "" && r.Calendar != CalendarGregorianProleptic {
		b.WriteString(" (")
		b.WriteString(string(r.Calendar))
		b.WriteString(")")
	}
	return b.String()
}
