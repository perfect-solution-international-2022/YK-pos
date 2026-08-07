package dto

/*
Report responses are read-only aggregates. They carry no ids to act on and no
pagination: each one is a whole picture — a year, a window, a headcount — and a
page of it would be meaningless.
*/

// EmployeeReportResponse is the headcount picture.
type EmployeeReportResponse struct {
	TotalEmployees  int64 `json:"total_employees"`
	ActiveEmployees int64 `json:"active_employees"`
	// The monthly wage bill: the sum of active employees' basic salaries, in
	// integer cents. Basic only — overtime and bonuses belong to a payroll run,
	// not to a headcount.
	MonthlySalaryCents int64 `json:"monthly_salary_cents"`

	ByDesignation    []DesignationHeadcountRow `json:"by_designation"`
	ByEmploymentType []EmploymentTypeRow       `json:"by_employment_type"`
}

type DesignationHeadcountRow struct {
	DesignationID   string `json:"designation_id"`
	DesignationName string `json:"designation_name"`
	Total           int64  `json:"total"`
	ActiveTotal     int64  `json:"active_total"`
	SalaryCents     int64  `json:"salary_cents"`
}

type EmploymentTypeRow struct {
	EmploymentType string `json:"employment_type"`
	Total          int64  `json:"total"`
}

// AttendanceReportResponse reuses the summary row shape, so a client renders the
// register the same way whether it came from the summary endpoint or the report.
type AttendanceReportResponse struct {
	From string                 `json:"from"`
	To   string                 `json:"to"`
	Rows []AttendanceSummaryRow `json:"rows"`
}

// LeaveReportResponse is one window's leave usage by type.
type LeaveReportResponse struct {
	From string           `json:"from"`
	To   string           `json:"to"`
	Rows []LeaveReportRow `json:"rows"`
}

type LeaveReportRow struct {
	LeaveTypeID   string  `json:"leave_type_id"`
	LeaveTypeName string  `json:"leave_type_name"`
	IsPaid        bool    `json:"is_paid"`
	Requests      int64   `json:"requests"`
	ApprovedDays  float64 `json:"approved_days"`
	PendingDays   float64 `json:"pending_days"`
	RejectedCount int64   `json:"rejected_count"`
}

/*
PayrollReportResponse is one year of monthly totals.

Draft runs are included: a manager comparing this month against last needs the
figure they are about to approve. The run's own status says which is which.
*/
type PayrollReportResponse struct {
	Year   int                 `json:"year"`
	Rows   []PayrollReportRow  `json:"rows"`
	Totals PayrollReportTotals `json:"totals"`
}

type PayrollReportRow struct {
	PeriodMonth      int   `json:"period_month"`
	Employees        int64 `json:"employees"`
	GrossCents       int64 `json:"gross_cents"`
	DeductionCents   int64 `json:"deduction_cents"`
	NetCents         int64 `json:"net_cents"`
	OvertimeCents    int64 `json:"overtime_cents"`
	BonusCents       int64 `json:"bonus_cents"`
	EPFEmployerCents int64 `json:"epf_employer_cents"`
	ETFCents         int64 `json:"etf_cents"`
}

// PayrollReportTotals is the year summed, so the client does not re-add twelve
// rows and risk disagreeing with the server about the answer.
type PayrollReportTotals struct {
	GrossCents       int64 `json:"gross_cents"`
	DeductionCents   int64 `json:"deduction_cents"`
	NetCents         int64 `json:"net_cents"`
	OvertimeCents    int64 `json:"overtime_cents"`
	BonusCents       int64 `json:"bonus_cents"`
	EPFEmployerCents int64 `json:"epf_employer_cents"`
	ETFCents         int64 `json:"etf_cents"`
}

// ReportRangeQuery bounds the window reports that take one.
type ReportRangeQuery struct {
	From string `query:"from" validate:"omitempty,datetime=2006-01-02"`
	To   string `query:"to" validate:"omitempty,datetime=2006-01-02"`
}

// PayrollReportQuery selects the year a payroll report covers; absent, the
// server uses the current one.
type PayrollReportQuery struct {
	Year int `query:"year" validate:"omitempty,min=2000,max=2200"`
}

/*
HRMSettingsResponse and its request are the same shape: the settings screen reads
all three groups and writes all three back, so a separate request type would be
the identical fields with different tags.

Percentages are 0-100 (8 = 8%), not fractions — unlike a product's tax_rate. A
payroll officer enters "8" for EPF, and the conversion happens once, server-side.
*/
type HRMSettingsResponse struct {
	Attendance AttendanceRulesDTO `json:"attendance"`
	Leave      LeaveRulesDTO      `json:"leave"`
	Payroll    PayrollRulesDTO    `json:"payroll"`
}

type AttendanceRulesDTO struct {
	FullDayMinutes     int  `json:"full_day_minutes" validate:"min=1,max=1440"`
	HalfDayMinutes     int  `json:"half_day_minutes" validate:"min=1,max=1440"`
	OvertimeEnabled    bool `json:"overtime_enabled"`
	MinOvertimeMinutes int  `json:"min_overtime_minutes" validate:"min=0,max=480"`
	AllowFutureEntry   bool `json:"allow_future_entry"`
}

type LeaveRulesDTO struct {
	MaxConsecutiveDays   int  `json:"max_consecutive_days" validate:"min=0,max=365"`
	MinNoticeDays        int  `json:"min_notice_days" validate:"min=0,max=90"`
	AllowNegativeBalance bool `json:"allow_negative_balance"`
	ExcludeWeeklyOff     bool `json:"exclude_weekly_off"`
}

type PayrollRulesDTO struct {
	// Required and positive: it is the divisor that turns a monthly salary into
	// a daily rate, so a zero would make every payroll run fail.
	WorkingDaysPerMonth float64 `json:"working_days_per_month" validate:"required,gt=0,max=31"`
	WorkingHoursPerDay  float64 `json:"working_hours_per_day" validate:"required,gt=0,max=24"`
	OvertimeMultiplier  float64 `json:"overtime_multiplier" validate:"min=0,max=5"`

	EPFEmployeePercent float64 `json:"epf_employee_percent" validate:"min=0,max=100"`
	EPFEmployerPercent float64 `json:"epf_employer_percent" validate:"min=0,max=100"`
	ETFPercent         float64 `json:"etf_percent" validate:"min=0,max=100"`
	TaxPercent         float64 `json:"tax_percent" validate:"min=0,max=100"`

	DeductAbsentDays  bool `json:"deduct_absent_days"`
	DeductUnpaidLeave bool `json:"deduct_unpaid_leave"`
}
