package mapper

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"
)

/*
Helpers shared by the HRM mappers. The product and order mappers already own
epochMillis, isoDatePtr, parseISODate and the uuid/cents conversions; these add
the shapes HRM needs on top — required (non-pointer) calendar dates, optional
instants, and the "HH:mm" clock times shifts are expressed in.
*/

// clockLayout is the wire format for a shift's start and end: 24-hour, no
// seconds, no date, no zone.
const clockLayout = "15:04"

// isoDate formats a required calendar date. A zero time yields an empty string
// rather than "0001-01-01", which would read as a real date on the client.
func isoDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}

// parseISODateValue is parseISODate for a required field: an unparseable or
// empty value yields the zero time, which the validator has already ruled out
// by the time a request reaches a mapper.
func parseISODateValue(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// epochMillisPtr converts an optional instant. Nil in, nil out — the field is
// omitted from the JSON rather than sent as 0, which the client would read as
// the epoch.
func epochMillisPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}

// millisToTimePtr is epochMillisPtr's inverse, for the optional instants a
// request can carry (a punch recorded while the terminal was offline).
func millisToTimePtr(ms *int64) *time.Time {
	if ms == nil || *ms <= 0 {
		return nil
	}
	t := time.UnixMilli(*ms)
	return &t
}

// formatClock renders a TIME column as "HH:mm".
func formatClock(t datatypes.Time) string {
	minutes := int(t / 60_000_000_000)
	return time.Date(0, 1, 1, minutes/60, minutes%60, 0, 0, time.UTC).Format(clockLayout)
}

// parseClock is formatClock's inverse. The value is validated as
// datetime=15:04 before it gets here, so an unparseable string means midnight
// rather than an error.
func parseClock(s string) datatypes.Time {
	parsed, err := time.Parse(clockLayout, s)
	if err != nil {
		return 0
	}
	return datatypes.NewTime(parsed.Hour(), parsed.Minute(), 0, 0)
}

// intSliceToJSON encodes a weekday list for the weekly_off JSONB column, which
// is NOT NULL — an empty list becomes the literal [] rather than nil.
func intSliceToJSON(values []int) datatypes.JSON {
	if len(values) == 0 {
		return datatypes.JSON("[]")
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return datatypes.JSON("[]")
	}
	return datatypes.JSON(raw)
}

// jsonToIntSlice decodes it back. Malformed stored JSON yields an empty slice
// rather than an error: one bad roster must not fail a whole shift list.
func jsonToIntSlice(raw []byte) []int {
	out := []int{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return []int{}
	}
	return out
}

// stringPtr returns a pointer to s, or nil when s is empty — the shape the
// optional response fields take.
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
