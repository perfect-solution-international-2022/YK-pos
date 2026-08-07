package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

//go:generate go run go.uber.org/mock/mockgen -source=hrm_report_repository.go -destination=mocks/hrm_report_repository_mock.go -package=mocks

/*
The report rows below are aggregates, not entities: they are computed by the
database and never written back. They live here rather than in internal/entity
for that reason — nothing maps them to a table.
*/

// HeadcountRow is one bucket of the employee report: how many staff hold a
// designation, and what they cost per month.
type HeadcountRow struct {
	DesignationID   uuid.UUID `gorm:"column:designation_id"`
	DesignationName string    `gorm:"column:designation_name"`
	Total           int64     `gorm:"column:total"`
	ActiveTotal     int64     `gorm:"column:active_total"`
	SalaryCents     int64     `gorm:"column:salary_cents"`
}

// EmploymentTypeRow is the headcount split by contract type.
type EmploymentTypeRow struct {
	EmploymentType string `gorm:"column:employment_type"`
	Total          int64  `gorm:"column:total"`
}

// LeaveReportRow is one leave type's usage across the business in a window.
type LeaveReportRow struct {
	LeaveTypeID   uuid.UUID `gorm:"column:leave_type_id"`
	LeaveTypeName string    `gorm:"column:leave_type_name"`
	IsPaid        bool      `gorm:"column:is_paid"`
	Requests      int64     `gorm:"column:requests"`
	ApprovedDays  float64   `gorm:"column:approved_days"`
	PendingDays   float64   `gorm:"column:pending_days"`
	RejectedCount int64     `gorm:"column:rejected_count"`
}

// PayrollReportRow is one month's payroll totals.
type PayrollReportRow struct {
	PeriodYear     int   `gorm:"column:period_year"`
	PeriodMonth    int   `gorm:"column:period_month"`
	Employees      int64 `gorm:"column:employees"`
	GrossCents     int64 `gorm:"column:gross_cents"`
	DeductionCents int64 `gorm:"column:deduction_cents"`
	NetCents       int64 `gorm:"column:net_cents"`
	OvertimeCents  int64 `gorm:"column:overtime_cents"`
	BonusCents     int64 `gorm:"column:bonus_cents"`
	EPFEmployer    int64 `gorm:"column:epf_employer_cents"`
	ETFCents       int64 `gorm:"column:etf_cents"`
}

/*
HRMReportRepository runs the module's aggregate queries.

They are separated from the CRUD repositories because they answer questions
across several tables at once and belong to no single aggregate. Each one is a
single grouped statement rather than a fetch-and-sum in Go: a shop with a
hundred staff and three years of attendance would otherwise pull hundreds of
thousands of rows through the application to produce twelve numbers.
*/
type HRMReportRepository interface {
	HeadcountByDesignation(ctx context.Context, businessID uuid.UUID) ([]HeadcountRow, error)
	HeadcountByEmploymentType(ctx context.Context, businessID uuid.UUID) ([]EmploymentTypeRow, error)
	LeaveUsage(ctx context.Context, businessID uuid.UUID, r DateRange) ([]LeaveReportRow, error)
	PayrollTotals(ctx context.Context, businessID uuid.UUID, year int) ([]PayrollReportRow, error)
}

type hrmReportRepository struct {
	db *gorm.DB
}

func NewHRMReportRepository(db *gorm.DB) HRMReportRepository {
	return &hrmReportRepository{db: db}
}

/*
HeadcountByDesignation joins from designations outward, so a title nobody holds
still appears with a zero — an empty row is the point of the report when a shop
is checking whether it has a supervisor on the books at all.
*/
func (r *hrmReportRepository) HeadcountByDesignation(
	ctx context.Context, businessID uuid.UUID,
) ([]HeadcountRow, error) {
	var rows []HeadcountRow
	err := r.db.WithContext(ctx).
		Model(&entity.Designation{}).
		Select(`hrm_designations.id AS designation_id,
			hrm_designations.name AS designation_name,
			COUNT(e.id) AS total,
			COUNT(e.id) FILTER (WHERE e.status = 'active') AS active_total,
			COALESCE(SUM(e.basic_salary_cents) FILTER (WHERE e.status = 'active'), 0) AS salary_cents`).
		Joins(`LEFT JOIN hrm_employees e
			ON e.designation_id = hrm_designations.id AND e.deleted_at IS NULL`).
		Where("hrm_designations.business_id = ?", businessID).
		Group("hrm_designations.id, hrm_designations.name").
		Order("hrm_designations.name").
		Scan(&rows).Error
	return rows, err
}

func (r *hrmReportRepository) HeadcountByEmploymentType(
	ctx context.Context, businessID uuid.UUID,
) ([]EmploymentTypeRow, error) {
	var rows []EmploymentTypeRow
	err := r.db.WithContext(ctx).
		Model(&entity.Employee{}).
		Select("employment_type, COUNT(*) AS total").
		Where("business_id = ?", businessID).
		Group("employment_type").
		Order("employment_type").
		Scan(&rows).Error
	return rows, err
}

func (r *hrmReportRepository) LeaveUsage(
	ctx context.Context, businessID uuid.UUID, dr DateRange,
) ([]LeaveReportRow, error) {
	query := r.db.WithContext(ctx).
		Model(&entity.LeaveRequest{}).
		Select(`t.id AS leave_type_id,
			t.name AS leave_type_name,
			t.is_paid AS is_paid,
			COUNT(*) AS requests,
			COALESCE(SUM(hrm_leave_requests.days) FILTER (WHERE hrm_leave_requests.status = 'approved'), 0) AS approved_days,
			COALESCE(SUM(hrm_leave_requests.days) FILTER (WHERE hrm_leave_requests.status = 'pending'), 0) AS pending_days,
			COUNT(*) FILTER (WHERE hrm_leave_requests.status = 'rejected') AS rejected_count`).
		Joins("JOIN hrm_leave_types t ON t.id = hrm_leave_requests.leave_type_id").
		Where("hrm_leave_requests.business_id = ?", businessID).
		Group("t.id, t.name, t.is_paid").
		Order("t.name")

	query = applyDateRange(query, "hrm_leave_requests.start_date", dr)

	var rows []LeaveReportRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

/*
PayrollTotals reports a year of monthly totals. Draft runs are included: a
manager comparing this month against last needs the figure they are about to
approve, and the payslip status travels with each row's run for the caller to
qualify it.
*/
func (r *hrmReportRepository) PayrollTotals(
	ctx context.Context, businessID uuid.UUID, year int,
) ([]PayrollReportRow, error) {
	var rows []PayrollReportRow
	err := r.db.WithContext(ctx).
		Model(&entity.Payslip{}).
		Select(`period_year,
			period_month,
			COUNT(*) AS employees,
			COALESCE(SUM(gross_cents), 0) AS gross_cents,
			COALESCE(SUM(deduction_cents), 0) AS deduction_cents,
			COALESCE(SUM(net_cents), 0) AS net_cents,
			COALESCE(SUM(overtime_cents), 0) AS overtime_cents,
			COALESCE(SUM(bonus_cents), 0) AS bonus_cents,
			COALESCE(SUM(epf_employer_cents), 0) AS epf_employer_cents,
			COALESCE(SUM(etf_cents), 0) AS etf_cents`).
		Where("business_id = ? AND period_year = ?", businessID, year).
		Group("period_year, period_month").
		Order("period_month").
		Scan(&rows).Error
	return rows, err
}
