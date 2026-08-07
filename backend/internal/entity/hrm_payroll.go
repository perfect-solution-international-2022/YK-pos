package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Payroll lifecycle. A draft can be recomputed and edited; finalising freezes
// it, because a payslip a member of staff has already been shown must not
// change under them. "paid" records that the money actually left.
const (
	PayrollStatusDraft     = "draft"
	PayrollStatusFinalized = "finalized"
	PayrollStatusPaid      = "paid"
)

// Ad-hoc payslip line kinds.
const (
	PayslipItemEarning   = "earning"
	PayslipItemDeduction = "deduction"
)

// PayrollRun is one month's payroll for one business, unique on
// (business_id, period_year, period_month).
//
// Rules is a snapshot of the payroll settings the run was computed under. It is
// stored rather than read live at print time so that changing the EPF rate next
// year cannot silently reinterpret a payslip issued this year.
type PayrollRun struct {
	IDMixin
	Timestamps

	BusinessID  uuid.UUID      `gorm:"column:business_id;not null"`
	PeriodYear  int            `gorm:"column:period_year;not null"`
	PeriodMonth int            `gorm:"column:period_month;not null"`
	Status      string         `gorm:"column:status;not null"`
	Rules       datatypes.JSON `gorm:"column:rules;not null;default:'{}'"`
	Notes       *string        `gorm:"column:notes"`
	GeneratedBy *uuid.UUID     `gorm:"column:generated_by"`
	FinalizedAt *time.Time     `gorm:"column:finalized_at"`
	PaidAt      *time.Time     `gorm:"column:paid_at"`

	Payslips []Payslip `gorm:"foreignKey:PayrollRunID;references:ID"`
}

func (PayrollRun) TableName() string { return "hrm_payroll_runs" }

// IsEditable reports whether the run may still be recomputed or adjusted.
func (r PayrollRun) IsEditable() bool { return r.Status == PayrollStatusDraft }

// Payslip is one employee's pay for one run. Every money field is integer
// cents; the day counts are fractional because a half-day is 0.5.
//
// The period is denormalised from the run so payslip history can be filtered by
// month without a join — which is what every payroll report does.
type Payslip struct {
	IDMixin
	Timestamps

	PayrollRunID uuid.UUID `gorm:"column:payroll_run_id;not null"`
	BusinessID   uuid.UUID `gorm:"column:business_id;not null"`
	EmployeeID   uuid.UUID `gorm:"column:employee_id;not null"`
	PeriodYear   int       `gorm:"column:period_year;not null"`
	PeriodMonth  int       `gorm:"column:period_month;not null"`

	BasicSalaryCents int64   `gorm:"column:basic_salary_cents;not null"`
	PayableDays      float64 `gorm:"column:payable_days;not null"`
	PresentDays      float64 `gorm:"column:present_days;not null"`
	AbsentDays       float64 `gorm:"column:absent_days;not null"`
	PaidLeaveDays    float64 `gorm:"column:paid_leave_days;not null"`
	UnpaidLeaveDays  float64 `gorm:"column:unpaid_leave_days;not null"`

	OvertimeMinutes int   `gorm:"column:overtime_minutes;not null"`
	OvertimeCents   int64 `gorm:"column:overtime_cents;not null"`
	BonusCents      int64 `gorm:"column:bonus_cents;not null"`
	AllowanceCents  int64 `gorm:"column:allowance_cents;not null"`

	AbsenceDeductionCents int64 `gorm:"column:absence_deduction_cents;not null"`
	EPFEmployeeCents      int64 `gorm:"column:epf_employee_cents;not null"`
	EPFEmployerCents      int64 `gorm:"column:epf_employer_cents;not null"`
	ETFCents              int64 `gorm:"column:etf_cents;not null"`
	TaxCents              int64 `gorm:"column:tax_cents;not null"`
	OtherDeductionCents   int64 `gorm:"column:other_deduction_cents;not null"`

	GrossCents     int64 `gorm:"column:gross_cents;not null"`
	DeductionCents int64 `gorm:"column:deduction_cents;not null"`
	NetCents       int64 `gorm:"column:net_cents;not null"`

	Status string `gorm:"column:status;not null"`

	Items    []PayslipItem `gorm:"foreignKey:PayslipID;references:ID"`
	Employee *Employee     `gorm:"foreignKey:EmployeeID;references:ID"`
}

func (Payslip) TableName() string { return "hrm_payslips" }

// PayslipItem is an ad-hoc earning or deduction a manager added to a draft:
// a festival bonus, a uniform charge, a salary advance being recovered.
// Relational rather than JSON so a payslip can print its own breakdown.
type PayslipItem struct {
	IDMixin
	Timestamps

	PayslipID   uuid.UUID `gorm:"column:payslip_id;not null"`
	Kind        string    `gorm:"column:kind;not null"`
	Label       string    `gorm:"column:label;not null"`
	AmountCents int64     `gorm:"column:amount_cents;not null"`
}

func (PayslipItem) TableName() string { return "hrm_payslip_items" }
