// Package pricesync contains the automatic price update logic: the schedule
// calculation, the persisted run state and the run manager.
package pricesync

import (
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	log "github.com/sirupsen/logrus"
)

const (
	// defaultSlotHour and defaultSlotMinute describe the standard slot used
	// when no (or an invalid) price update time is configured.
	defaultSlotHour   = 22
	defaultSlotMinute = 30

	// defaultSlotLayout is the expected format of the configured time.
	defaultSlotLayout = "15:04"
)

// defaultSlotWeekdays are the weekdays used when no (or only invalid) days
// are configured.
var defaultSlotWeekdays = []time.Weekday{
	time.Monday,
	time.Tuesday,
	time.Wednesday,
	time.Thursday,
	time.Friday,
}

// weekdayNames maps the configured day names to weekdays.
var weekdayNames = map[string]time.Weekday{
	"mon":       time.Monday,
	"monday":    time.Monday,
	"tue":       time.Tuesday,
	"tuesday":   time.Tuesday,
	"wed":       time.Wednesday,
	"wednesday": time.Wednesday,
	"thu":       time.Thursday,
	"thursday":  time.Thursday,
	"fri":       time.Friday,
	"friday":    time.Friday,
	"sat":       time.Saturday,
	"saturday":  time.Saturday,
	"sun":       time.Sunday,
	"sunday":    time.Sunday,
}

// Schedule describes when automatic price updates may run. It is derived from
// the price_update configuration and is free of side effects.
type Schedule struct {
	// Active is true when automatic price updates are enabled.
	Active bool
	// Location is the time zone the slot is expressed in.
	Location *time.Location
	// Hour and Minute are the wall clock time of the slot.
	Hour   int
	Minute int
	// Weekdays holds the weekdays the slot runs on.
	Weekdays map[time.Weekday]bool
}

// ParseSchedule builds a Schedule from the price update configuration. Invalid
// time or day values are logged as a warning and fall back to the defaults so
// that a broken configuration never prevents the server from starting.
func ParseSchedule(cfg config.PriceUpdateConfig, loc *time.Location) Schedule {
	if loc == nil {
		loc = time.Local
	}

	schedule := Schedule{
		Active:   cfg.Enabled != config.No,
		Location: loc,
		Hour:     defaultSlotHour,
		Minute:   defaultSlotMinute,
		Weekdays: weekdaySet(defaultSlotWeekdays),
	}

	if cfg.Time != "" {
		parsed, err := time.Parse(defaultSlotLayout, cfg.Time)
		if err != nil {
			log.Warnf("Invalid price update time %q, falling back to %02d:%02d", cfg.Time, defaultSlotHour, defaultSlotMinute)
		} else {
			schedule.Hour = parsed.Hour()
			schedule.Minute = parsed.Minute()
		}
	}

	if len(cfg.Days) > 0 {
		days := make(map[time.Weekday]bool, len(cfg.Days))
		for _, day := range cfg.Days {
			weekday, ok := parseWeekday(day)
			if !ok {
				log.Warnf("Unknown price update day %q, ignoring it", day)
				continue
			}
			days[weekday] = true
		}
		if len(days) == 0 {
			log.Warnf("No valid price update days configured, falling back to Monday to Friday")
		} else {
			schedule.Weekdays = days
		}
	}

	return schedule
}

// MostRecentSlot returns the latest slot that already happened at or before
// now. The second return value is false when the schedule is inactive or has
// no slot at all.
func (s Schedule) MostRecentSlot(now time.Time) (time.Time, bool) {
	if !s.Active || len(s.Weekdays) == 0 {
		return time.Time{}, false
	}

	location := s.location()
	local := now.In(location)
	year, month, day := local.Date()

	for offset := 0; offset <= 7; offset++ {
		candidate := time.Date(year, month, day-offset, s.Hour, s.Minute, 0, 0, location)
		if !s.Weekdays[candidate.Weekday()] {
			continue
		}
		if !candidate.After(local) {
			return candidate, true
		}
	}

	return time.Time{}, false
}

// NextSlot returns the first slot that happens after now. The second return
// value is false when the schedule is inactive or has no slot at all.
func (s Schedule) NextSlot(now time.Time) (time.Time, bool) {
	if !s.Active || len(s.Weekdays) == 0 {
		return time.Time{}, false
	}

	location := s.location()
	local := now.In(location)
	year, month, day := local.Date()

	for offset := 0; offset <= 7; offset++ {
		candidate := time.Date(year, month, day+offset, s.Hour, s.Minute, 0, 0, location)
		if !s.Weekdays[candidate.Weekday()] {
			continue
		}
		if candidate.After(local) {
			return candidate, true
		}
	}

	return time.Time{}, false
}

// location returns the time zone of the schedule, defaulting to the local one.
func (s Schedule) location() *time.Location {
	if s.Location == nil {
		return time.Local
	}
	return s.Location
}

// parseWeekday maps a configured day name to a weekday.
func parseWeekday(day string) (time.Weekday, bool) {
	weekday, ok := weekdayNames[strings.ToLower(strings.TrimSpace(day))]
	return weekday, ok
}

// weekdaySet turns a list of weekdays into a set.
func weekdaySet(days []time.Weekday) map[time.Weekday]bool {
	set := make(map[time.Weekday]bool, len(days))
	for _, day := range days {
		set[day] = true
	}
	return set
}
