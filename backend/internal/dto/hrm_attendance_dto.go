package dto

// AttendanceSortFields whitelists the register's `sort` values; the first is the
// default.
var AttendanceSortFields = []string{"work_date", "created_at", "status", "worked_minutes"}

/*
AttendanceResponse is one employee's record for one day.

work_date is yyyy-mm-dd — the business day the work belongs to, which for a
night shift is not the calendar day the clock-out happened on. The punch
timestamps are epoch milliseconds, like every other instant in this API.
*/
type AttendanceResponse struct {
	ID           string  `json:"id"`
	EmployeeID   string  `json:"employee_id"`
	EmployeeName *string `json:"employee_name,omitempty"`
	EmployeeCode *string `json:"employee_code,omitempty"`
	ShiftID      *string `json:"shift_id,omitempty"`

	WorkDate   string `json:"work_date"`
	ClockInAt  *int64 `json:"clock_in_at,omitempty"`
	ClockOutAt *int64 `json:"clock_out_at,omitempty"`

	Status string `json:"status"`
	Source string `json:"source"`

	WorkedMinutes     int `json:"worked_minutes"`
	LateMinutes       int `json:"late_minutes"`
	EarlyLeaveMinutes int `json:"early_leave_minutes"`
	OvertimeMinutes   int `json:"overtime_minutes"`

	Note      *string `json:"note,omitempty"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
}

/*
ClockRequest punches an employee in or out.

employee_id is optional: omitted, the server resolves the employee linked to the
authenticated user, which is how a cashier punches themselves in from the till.
Supplying one is a supervisor punching somebody else in and is gated on
hrm.manage.

at is optional too and defaults to the server's clock. A client-supplied time is
honoured so a terminal that was offline can post the real punch rather than the
moment it reconnected.
*/
type ClockRequest struct {
	EmployeeID string `json:"employee_id" validate:"omitempty,uuid"`
	At         *int64 `json:"at" validate:"omitempty,min=0"`
}

/*
AttendanceRecordRequest writes a day by hand.

Both times are optional: a day marked absent or on leave has none. An explicit
status wins over the one the hours would imply — a supervisor marking a short
day as present is making a decision the arithmetic must not overrule.
*/
type AttendanceRecordRequest struct {
	EmployeeID string  `json:"employee_id" validate:"required,uuid"`
	WorkDate   string  `json:"work_date" validate:"required,datetime=2006-01-02"`
	ClockInAt  *int64  `json:"clock_in_at" validate:"omitempty,min=0"`
	ClockOutAt *int64  `json:"clock_out_at" validate:"omitempty,min=0"`
	Status     string  `json:"status" validate:"omitempty,oneof=present late half_day absent on_leave holiday weekly_off"`
	Note       *string `json:"note" validate:"omitempty,max=500"`
}

/*
AttendanceListQuery is the GET /hrm/attendance query string.
*/
type AttendanceListQuery struct {
	HRMListQuery

	EmployeeID string `query:"employee_id" validate:"omitempty,uuid"`
	Status     string `query:"status" validate:"omitempty,oneof=present late half_day absent on_leave holiday weekly_off"`
}

/*
AttendanceSummaryQuery bounds a summary or monthly report.

Either a from/to window or a year/month pair; the month form exists because
"March 2026" is what a report screen actually asks for, and making the client
compute the last day of the month to express it would be a needless trap.
*/
type AttendanceSummaryQuery struct {
	EmployeeID string `query:"employee_id" validate:"omitempty,uuid"`
	From       string `query:"from" validate:"omitempty,datetime=2006-01-02"`
	To         string `query:"to" validate:"omitempty,datetime=2006-01-02"`
	Year       int    `query:"year" validate:"omitempty,min=2000,max=2200"`
	Month      int    `query:"month" validate:"omitempty,min=1,max=12"`
}

/*
AttendanceSummaryRow is one employee's totals for the requested window. Day
counts are fractional because a half day is 0.5.
*/
type AttendanceSummaryRow struct {
	EmployeeID   string  `json:"employee_id"`
	EmployeeName *string `json:"employee_name,omitempty"`
	EmployeeCode *string `json:"employee_code,omitempty"`

	PresentDays float64 `json:"present_days"`
	LateDays    float64 `json:"late_days"`
	HalfDays    float64 `json:"half_days"`
	AbsentDays  float64 `json:"absent_days"`
	LeaveDays   float64 `json:"leave_days"`

	WorkedMinutes   int64 `json:"worked_minutes"`
	OvertimeMinutes int64 `json:"overtime_minutes"`
	LateMinutes     int64 `json:"late_minutes"`
}

// The window plus its rows is AttendanceReportResponse (hrm_report_dto.go): the
// summary endpoint and the attendance report return the identical shape, so
// there is one type rather than two that could drift.
