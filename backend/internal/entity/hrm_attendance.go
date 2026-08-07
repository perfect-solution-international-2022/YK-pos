package entity

import (
	"time"

	"github.com/google/uuid"
)

// Attendance outcomes for one working day.
const (
	AttendanceStatusPresent   = "present"
	AttendanceStatusLate      = "late"
	AttendanceStatusHalfDay   = "half_day"
	AttendanceStatusAbsent    = "absent"
	AttendanceStatusOnLeave   = "on_leave"
	AttendanceStatusHoliday   = "holiday"
	AttendanceStatusWeeklyOff = "weekly_off"
)

// How the row got there: a punch at the till, a supervisor's correction, or a
// row the server wrote itself (leave approval, month-end rollup). Kept because
// "who said this person was absent" is the first question of any dispute.
const (
	AttendanceSourceClock  = "clock"
	AttendanceSourceManual = "manual"
	AttendanceSourceSystem = "system"
)

// Attendance is one employee's record for one calendar day. The
// (employee_id, work_date) unique index is what makes clock-in idempotent: a
// double-tap updates the existing row instead of creating a second half-day.
//
// The derived minute columns are stored, not computed on read. Payroll reads a
// whole month for every employee at once, and re-deriving shift arithmetic per
// row inside that query would make a payroll run scale with history rather than
// with headcount.
type Attendance struct {
	IDMixin
	Timestamps

	BusinessID uuid.UUID  `gorm:"column:business_id;not null"`
	EmployeeID uuid.UUID  `gorm:"column:employee_id;not null"`
	ShiftID    *uuid.UUID `gorm:"column:shift_id"`

	// The business day the work belongs to — a calendar date, deliberately not
	// derived from clock_in_at, so a night shift punched in at 22:00 and out at
	// 06:00 stays one day's work.
	WorkDate   time.Time  `gorm:"column:work_date;type:date;not null"`
	ClockInAt  *time.Time `gorm:"column:clock_in_at"`
	ClockOutAt *time.Time `gorm:"column:clock_out_at"`

	Status string `gorm:"column:status;not null"`
	Source string `gorm:"column:source;not null"`

	WorkedMinutes     int `gorm:"column:worked_minutes;not null"`
	LateMinutes       int `gorm:"column:late_minutes;not null"`
	EarlyLeaveMinutes int `gorm:"column:early_leave_minutes;not null"`
	OvertimeMinutes   int `gorm:"column:overtime_minutes;not null"`

	Note       *string    `gorm:"column:note"`
	RecordedBy *uuid.UUID `gorm:"column:recorded_by"`

	// Preloaded by the history and report reads, which show a name per row.
	Employee *Employee `gorm:"foreignKey:EmployeeID;references:ID"`
}

func (Attendance) TableName() string { return "hrm_attendance" }

// CountsAsWorked reports whether the day contributes to payable attendance.
// Paid leave is settled from the leave balance rather than here, so only
// genuinely worked states count.
func (a Attendance) CountsAsWorked() bool {
	switch a.Status {
	case AttendanceStatusPresent, AttendanceStatusLate, AttendanceStatusHalfDay:
		return true
	default:
		return false
	}
}
