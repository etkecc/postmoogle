package crontab

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// schedule is a parsed 5-field cron spec. Each field is a lookup table indexed by
// the time value (minute[t.Minute()], month[int(t.Month())], ...), so match is a set
// of index reads. domStar and dowStar record whether the day-of-month and day-of-week
// fields were "*", which selects between the Vixie union and intersection in match.
type schedule struct {
	minute, hour, dom, month, dow []bool
	domStar, dowStar              bool
}

// match reports whether the schedule fires at t. t must already be in the target
// location; the caller (runDue) passes now.In(c.loc) so field extraction is zone-correct.
//
// Day handling follows Vixie cron: when both day-of-month and day-of-week are restricted
// (neither is "*"), a day matches if it satisfies EITHER field; when at least one is "*",
// both must match. "1 0 1 * 1" therefore fires on the 1st of every month AND every Monday,
// not only when the 1st is a Monday. Collapsing domStar/dowStar into a plain AND is the
// silent-wrong bug this split exists to prevent.
func (s *schedule) match(t time.Time) bool {
	if !s.minute[t.Minute()] || !s.hour[t.Hour()] || !s.month[int(t.Month())] {
		return false
	}
	if !s.domStar && !s.dowStar {
		return s.dom[t.Day()] || s.dow[int(t.Weekday())]
	}
	return s.dom[t.Day()] && s.dow[int(t.Weekday())]
}

// parse turns a standard 5-field cron spec ("min hour dom month dow") into a schedule.
// Fields are numeric only (no JAN/MON names). Day-of-week accepts 0-7 with both 0 and 7
// meaning Sunday. Calendar-impossible dates (e.g. "0 0 30 2 *") parse without error and
// simply never fire, matching standard cron.
func parse(spec string) (*schedule, error) {
	fields := strings.Fields(spec)
	if len(fields) != 5 {
		return nil, fmt.Errorf("crontab: expected 5 fields, got %d in %q", len(fields), spec)
	}
	minute, err := parseField(fields[0], 0, 59, "minute")
	if err != nil {
		return nil, err
	}
	hour, err := parseField(fields[1], 0, 23, "hour")
	if err != nil {
		return nil, err
	}
	dom, err := parseField(fields[2], 1, 31, "day-of-month")
	if err != nil {
		return nil, err
	}
	month, err := parseField(fields[3], 1, 12, "month")
	if err != nil {
		return nil, err
	}
	dow, err := parseField(fields[4], 0, 7, "day-of-week")
	if err != nil {
		return nil, err
	}
	if dow[7] { // normalize 7 (Sunday) onto 0, since time.Weekday() is 0-6
		dow[0] = true
	}
	return &schedule{
		minute:  minute,
		hour:    hour,
		dom:     dom,
		month:   month,
		dow:     dow,
		domStar: fields[2] == "*",
		dowStar: fields[4] == "*",
	}, nil
}

// parseField parses one comma-separated cron field into a lookup table of length maxVal+1.
// name identifies the field in error messages ("day-of-month field value 40 out of range").
func parseField(spec string, minVal, maxVal int, name string) ([]bool, error) {
	set := make([]bool, maxVal+1)
	for term := range strings.SplitSeq(spec, ",") {
		if err := parseTerm(term, minVal, maxVal, name, set); err != nil {
			return nil, err
		}
	}
	return set, nil
}

// parseTerm sets the bits for a single field term: "*", "a", "a-b", "*/N", "a/N" (= a-max/N),
// or "a-b/N". A missing step defaults to 1. Step must be a positive integer; "*/0" is rejected
// (the infinite-loop trap), as are out-of-range values and reversed ranges.
func parseTerm(term string, minVal, maxVal int, name string, set []bool) error {
	rangePart, stepStr, hasStep := strings.Cut(term, "/")
	step := 1
	if hasStep {
		n, err := strconv.Atoi(stepStr)
		if err != nil || n <= 0 {
			return fmt.Errorf("crontab: %s field invalid step %q", name, stepStr)
		}
		step = n
	}
	lo, hi, err := parseRange(rangePart, minVal, maxVal, name, hasStep)
	if err != nil {
		return err
	}
	for v := lo; v <= hi; v += step {
		set[v] = true
	}
	return nil
}

// parseRange resolves the low and high bounds of a term's range part. hasStep is true when
// the term carried a "/N", which turns a bare value "a" into the open range "a-max".
func parseRange(part string, minVal, maxVal int, name string, hasStep bool) (lo, hi int, err error) {
	if part == "*" {
		return minVal, maxVal, nil
	}
	if loStr, hiStr, found := strings.Cut(part, "-"); found {
		lo, err := parseValue(loStr, minVal, maxVal, name)
		if err != nil {
			return 0, 0, err
		}
		hi, err := parseValue(hiStr, minVal, maxVal, name)
		if err != nil {
			return 0, 0, err
		}
		if hi < lo {
			return 0, 0, fmt.Errorf("crontab: %s field reversed range %q", name, part)
		}
		return lo, hi, nil
	}
	v, err := parseValue(part, minVal, maxVal, name)
	if err != nil {
		return 0, 0, err
	}
	if hasStep {
		return v, maxVal, nil
	}
	return v, v, nil
}

// parseValue parses a single integer field value and checks it against the field range.
func parseValue(s string, minVal, maxVal int, name string) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("crontab: %s field invalid value %q", name, s)
	}
	if v < minVal || v > maxVal {
		return 0, fmt.Errorf("crontab: %s field value %d out of range [%d,%d]", name, v, minVal, maxVal)
	}
	return v, nil
}
