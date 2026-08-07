package dto

// Sort whitelists for the payroll lists; the first entry is the default.
var (
	PayrollRunSortFields = []string{"period_year", "created_at", "status"}
	PayslipSortFields    = []string{"created_at", "net_cents", "period_year"}
)

/*
PayrollRunResponse is one month's payroll.

rules is the settings snapshot the run was computed under, echoed back so a
payslip can be explained months later without depending on what the settings
screen says today.
*/
type PayrollRunResponse struct {
	ID          string `json:"id"`
	PeriodYear  int    `json:"period_year"`
	PeriodMonth int    `json:"period_month"`
	Status      string `json:"status"`

	Rules map[string]any `json:"rules,omitempty"`
	Notes *string        `json:"notes,omitempty"`

	FinalizedAt *int64 `json:"finalized_at,omitempty"`
	PaidAt      *int64 `json:"paid_at,omitempty"`

	// Totals across the run's payslips, so a list of months does not need one
	// request per month to show what each cost.
	EmployeeCount  int   `json:"employee_count"`
	GrossCents     int64 `json:"gross_cents"`
	DeductionCents int64 `json:"deduction_cents"`
	NetCents       int64 `json:"net_cents"`

	// Present only on the detail read.
	Payslips []PayslipResponse `json:"payslips,omitempty"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

/*
PayslipResponse is one employee's pay for one period.

Every money field is integer cents. The employer's EPF share and the ETF
contribution are reported for the employer's own returns — neither is deducted
from the employee, so neither is inside deduction_cents.
*/
type PayslipResponse struct {
	ID           string  `json:"id"`
	PayrollRunID string  `json:"payroll_run_id"`
	EmployeeID   string  `json:"employee_id"`
	EmployeeName *string `json:"employee_name,omitempty"`
	EmployeeCode *string `json:"employee_code,omitempty"`

	PeriodYear  int `json:"period_year"`
	PeriodMonth int `json:"period_month"`

	BasicSalaryCents int64   `json:"basic_salary_cents"`
	PayableDays      float64 `json:"payable_days"`
	PresentDays      float64 `json:"present_days"`
	AbsentDays       float64 `json:"absent_days"`
	PaidLeaveDays    float64 `json:"paid_leave_days"`
	UnpaidLeaveDays  float64 `json:"unpaid_leave_days"`

	OvertimeMinutes int   `json:"overtime_minutes"`
	OvertimeCents   int64 `json:"overtime_cents"`
	BonusCents      int64 `json:"bonus_cents"`
	AllowanceCents  int64 `json:"allowance_cents"`

	AbsenceDeductionCents int64 `json:"absence_deduction_cents"`
	EPFEmployeeCents      int64 `json:"epf_employee_cents"`
	EPFEmployerCents      int64 `json:"epf_employer_cents"`
	ETFCents              int64 `json:"etf_cents"`
	TaxCents              int64 `json:"tax_cents"`
	OtherDeductionCents   int64 `json:"other_deduction_cents"`

	GrossCents     int64 `json:"gross_cents"`
	DeductionCents int64 `json:"deduction_cents"`
	NetCents       int64 `json:"net_cents"`

	Status string                `json:"status"`
	Items  []PayslipItemResponse `json:"items,omitempty"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// PayslipItemResponse is one ad-hoc line: a festival bonus, a uniform charge.
type PayslipItemResponse struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	AmountCents int64  `json:"amount_cents"`
}

/*
GeneratePayrollRequest asks for a month.

employee_ids narrows the run — a shop paying contract staff on a different date
than permanent ones — and an empty list means everybody active. Re-posting the
same period recomputes a draft in place; a finalised period is refused.
*/
type GeneratePayrollRequest struct {
	Year        int      `json:"period_year" validate:"required,min=2000,max=2200"`
	Month       int      `json:"period_month" validate:"required,min=1,max=12"`
	EmployeeIDs []string `json:"employee_ids" validate:"omitempty,dive,uuid"`
	Notes       *string  `json:"notes" validate:"omitempty,max=1000"`
}

/*
PayslipAdjustRequest replaces a draft payslip's ad-hoc lines.

Replacement rather than append, so removing a bonus is expressible. The computed
components — basic, overtime, statutory deductions — are not editable: those come
from attendance and the run's rules snapshot, and letting a form overwrite them
would make the payslip unexplainable.
*/
type PayslipAdjustRequest struct {
	Items []PayslipItemRequest `json:"items" validate:"omitempty,max=20,dive"`
}

type PayslipItemRequest struct {
	Kind        string `json:"kind" validate:"required,oneof=earning deduction"`
	Label       string `json:"label" validate:"required,min=1,max=120"`
	AmountCents Cents  `json:"amount_cents" validate:"min=0"`
}

// PayslipListQuery is the GET /hrm/payroll/payslips query string.
type PayslipListQuery struct {
	PaginationQuery

	EmployeeID string `query:"employee_id" validate:"omitempty,uuid"`
	Year       int    `query:"period_year" validate:"omitempty,min=2000,max=2200"`
	Month      int    `query:"period_month" validate:"omitempty,min=1,max=12"`
	Status     string `query:"status" validate:"omitempty,oneof=draft finalized paid"`
}
