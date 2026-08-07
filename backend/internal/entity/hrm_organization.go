package entity

import (
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Shared lifecycle states for HRM master data. Deliberately narrower than
// UserStatus* — a job title is either in use or retired, with no middle state.
const (
	HRMStatusActive   = "active"
	HRMStatusInactive = "inactive"
)

// Designation is a job title: "Cashier", "Store Manager", "Stock Keeper".
// Employees reference it rather than carrying a free-text title, so a rename
// reaches every payroll and report at once.
type Designation struct {
	IDMixin
	Timestamps
	SoftDelete

	BusinessID  uuid.UUID `gorm:"column:business_id;not null"`
	Name        string    `gorm:"column:name;not null"`
	Description *string   `gorm:"column:description"`
	Status      string    `gorm:"column:status;not null"`
}

func (Designation) TableName() string { return "hrm_designations" }

// Shift is a working pattern: when the day starts and ends, how long the break
// is, how much lateness is forgiven, and which weekdays are off.
//
// StartTime/EndTime are clock times with no date and no timezone
// (datatypes.Time maps to Postgres TIME) — a shift starts at 08:00 whatever the
// calendar says. EndTime earlier than or equal to StartTime means the shift
// crosses midnight; CrossesMidnight reports that so the attendance arithmetic
// does not have to re-derive it at every call site.
type Shift struct {
	IDMixin
	Timestamps
	SoftDelete

	BusinessID   uuid.UUID      `gorm:"column:business_id;not null"`
	Name         string         `gorm:"column:name;not null"`
	StartTime    datatypes.Time `gorm:"column:start_time;not null"`
	EndTime      datatypes.Time `gorm:"column:end_time;not null"`
	BreakMinutes int            `gorm:"column:break_minutes;not null"`
	GraceMinutes int            `gorm:"column:grace_minutes;not null"`
	// Weekday numbers 0 (Sunday) - 6 (Saturday), matching JavaScript's
	// Date#getDay() and the frontend's Shift.weekly_off.
	WeeklyOff datatypes.JSON `gorm:"column:weekly_off;not null;default:'[]'"`
	Status    string         `gorm:"column:status;not null"`
}

func (Shift) TableName() string { return "hrm_shifts" }

// CrossesMidnight reports whether the shift ends on the following calendar day.
func (s Shift) CrossesMidnight() bool { return s.EndTime <= s.StartTime }

// ScheduledMinutes is the paid length of the shift: the span from start to end
// less the unpaid break. Overtime is measured against this, so a shift with a
// break longer than its span (a data-entry error) floors at zero rather than
// making every attendance row look like overtime.
func (s Shift) ScheduledMinutes() int {
	span := minutesOfDay(s.EndTime) - minutesOfDay(s.StartTime)
	if span <= 0 {
		span += 24 * 60
	}
	span -= s.BreakMinutes
	if span < 0 {
		return 0
	}
	return span
}

// StartMinutes is the shift's start as minutes since midnight, the unit every
// attendance calculation works in.
func (s Shift) StartMinutes() int { return minutesOfDay(s.StartTime) }

// WeeklyOffDays decodes the stored weekday list. A malformed or absent value
// yields no days off rather than an error: a bad JSON blob must not stop a
// cashier clocking in.
func (s Shift) WeeklyOffDays() []int {
	if len(s.WeeklyOff) == 0 {
		return nil
	}
	var days []int
	if err := json.Unmarshal(s.WeeklyOff, &days); err != nil {
		return nil
	}
	return days
}

// IsWeeklyOff reports whether weekday (0 = Sunday, matching time.Weekday and
// JavaScript's Date#getDay) is a rostered day off.
func (s Shift) IsWeeklyOff(weekday int) bool {
	for _, d := range s.WeeklyOffDays() {
		if d == weekday {
			return true
		}
	}
	return false
}

// minutesOfDay converts a TIME column's duration-since-midnight into whole
// minutes, which is the unit every attendance calculation works in.
func minutesOfDay(t datatypes.Time) int {
	return int(t / 60_000_000_000)
}
