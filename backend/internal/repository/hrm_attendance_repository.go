package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

//go:generate go run go.uber.org/mock/mockgen -source=hrm_attendance_repository.go -destination=mocks/hrm_attendance_repository_mock.go -package=mocks

/*
AttendanceListParams narrows the attendance history. A zero EmployeeID lists the
whole business, which is what the daily register screen shows.
*/
type AttendanceListParams struct {
	ListParams

	EmployeeID *uuid.UUID
	Status     string
	Range      DateRange
}

/*
AttendanceSummary is the aggregate behind the summary endpoint and the payroll
run: one employee's totals over a window. Computed in SQL rather than by
summing rows in Go, because payroll needs it for every employee at once and
pulling a month of rows per person would be an N+1 over the largest table in the
module.
*/
type AttendanceSummary struct {
	EmployeeID      uuid.UUID `gorm:"column:employee_id"`
	PresentDays     float64   `gorm:"column:present_days"`
	LateDays        float64   `gorm:"column:late_days"`
	HalfDays        float64   `gorm:"column:half_days"`
	AbsentDays      float64   `gorm:"column:absent_days"`
	LeaveDays       float64   `gorm:"column:leave_days"`
	WorkedMinutes   int64     `gorm:"column:worked_minutes"`
	OvertimeMinutes int64     `gorm:"column:overtime_minutes"`
	LateMinutes     int64     `gorm:"column:late_minutes"`
}

/*
AttendanceRepository persists the daily register.
*/
type AttendanceRepository interface {
	List(ctx context.Context, businessID uuid.UUID, params AttendanceListParams) ([]entity.Attendance, int64, error)
	FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Attendance, error)
	// FindByEmployeeDate is the idempotency check behind clock-in and manual
	// entry: the (employee_id, work_date) unique index means there is at most
	// one row to find.
	FindByEmployeeDate(ctx context.Context, businessID, employeeID uuid.UUID, workDate time.Time) (*entity.Attendance, error)
	Create(ctx context.Context, a *entity.Attendance) error
	Update(ctx context.Context, a *entity.Attendance) error
	Delete(ctx context.Context, businessID, id uuid.UUID) error

	/*
		SummarizeByEmployee aggregates a window per employee. Passing no ids
		covers the whole business, which is what the monthly report needs.
	*/
	SummarizeByEmployee(
		ctx context.Context, businessID uuid.UUID, r DateRange, employeeIDs []uuid.UUID,
	) ([]AttendanceSummary, error)
}

type attendanceRepository struct {
	db *gorm.DB
}

func NewAttendanceRepository(db *gorm.DB) AttendanceRepository {
	return &attendanceRepository{db: db}
}

func (r *attendanceRepository) List(
	ctx context.Context, businessID uuid.UUID, params AttendanceListParams,
) ([]entity.Attendance, int64, error) {
	var total int64
	if err := r.scoped(ctx, businessID, params).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entity.Attendance{}, 0, nil
	}

	var rows []entity.Attendance
	err := paginate(r.scoped(ctx, businessID, params), params.ListParams).
		Preload("Employee").
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *attendanceRepository) scoped(
	ctx context.Context, businessID uuid.UUID, params AttendanceListParams,
) *gorm.DB {
	query := r.db.WithContext(ctx).
		Model(&entity.Attendance{}).
		Where("hrm_attendance.business_id = ?", businessID)

	if params.EmployeeID != nil {
		query = query.Where("hrm_attendance.employee_id = ?", *params.EmployeeID)
	}
	if params.Status != "" {
		query = query.Where("hrm_attendance.status = ?", params.Status)
	}
	query = applyDateRange(query, "hrm_attendance.work_date", params.Range)

	if like := likePattern(params.Search); like != "" {
		/*
			Searching the register means searching for a person, so the match is
			against the employee rather than the attendance row. EXISTS rather
			than a join: a join would multiply rows and make the count wrong.
		*/
		query = query.Where(
			`EXISTS (
				SELECT 1 FROM hrm_employees e
				WHERE e.id = hrm_attendance.employee_id
				  AND (e.full_name ILIKE ? OR e.employee_code ILIKE ?)
			)`,
			like, like,
		)
	}

	return query
}

func (r *attendanceRepository) FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Attendance, error) {
	var row entity.Attendance
	err := r.db.WithContext(ctx).
		Preload("Employee").
		Where("business_id = ? AND id = ?", businessID, id).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *attendanceRepository) FindByEmployeeDate(
	ctx context.Context, businessID, employeeID uuid.UUID, workDate time.Time,
) (*entity.Attendance, error) {
	var row entity.Attendance
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND employee_id = ? AND work_date = ?", businessID, employeeID, workDate).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *attendanceRepository) Create(ctx context.Context, a *entity.Attendance) error {
	return r.db.WithContext(ctx).Omit("Employee").Create(a).Error
}

func (r *attendanceRepository) Update(ctx context.Context, a *entity.Attendance) error {
	res := r.db.WithContext(ctx).
		Model(&entity.Attendance{}).
		Where("business_id = ? AND id = ?", a.BusinessID, a.ID).
		Updates(map[string]any{
			"shift_id":            a.ShiftID,
			"clock_in_at":         a.ClockInAt,
			"clock_out_at":        a.ClockOutAt,
			"status":              a.Status,
			"source":              a.Source,
			"worked_minutes":      a.WorkedMinutes,
			"late_minutes":        a.LateMinutes,
			"early_leave_minutes": a.EarlyLeaveMinutes,
			"overtime_minutes":    a.OvertimeMinutes,
			"note":                a.Note,
			"recorded_by":         a.RecordedBy,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *attendanceRepository) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		Delete(&entity.Attendance{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

/*
SummarizeByEmployee counts days by status and sums the minute columns.

A half day counts as 0.5 of a present day, which is why the present count is a
SUM of weights rather than a COUNT — payroll multiplies it by the daily rate, so
counting a half day as a whole one would overpay.
*/
func (r *attendanceRepository) SummarizeByEmployee(
	ctx context.Context, businessID uuid.UUID, dr DateRange, employeeIDs []uuid.UUID,
) ([]AttendanceSummary, error) {
	query := r.db.WithContext(ctx).
		Model(&entity.Attendance{}).
		Select(`employee_id,
			COALESCE(SUM(CASE WHEN status IN ('present', 'late') THEN 1
			                  WHEN status = 'half_day' THEN 0.5 ELSE 0 END), 0) AS present_days,
			COALESCE(SUM(CASE WHEN status = 'late' THEN 1 ELSE 0 END), 0) AS late_days,
			COALESCE(SUM(CASE WHEN status = 'half_day' THEN 1 ELSE 0 END), 0) AS half_days,
			COALESCE(SUM(CASE WHEN status = 'absent' THEN 1 ELSE 0 END), 0) AS absent_days,
			COALESCE(SUM(CASE WHEN status = 'on_leave' THEN 1 ELSE 0 END), 0) AS leave_days,
			COALESCE(SUM(worked_minutes), 0) AS worked_minutes,
			COALESCE(SUM(overtime_minutes), 0) AS overtime_minutes,
			COALESCE(SUM(late_minutes), 0) AS late_minutes`).
		Where("business_id = ?", businessID).
		Group("employee_id")

	query = applyDateRange(query, "work_date", dr)
	if len(employeeIDs) > 0 {
		query = query.Where("employee_id IN ?", employeeIDs)
	}

	var rows []AttendanceSummary
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
