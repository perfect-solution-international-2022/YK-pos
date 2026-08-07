package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

//go:generate go run go.uber.org/mock/mockgen -source=hrm_leave_repository.go -destination=mocks/hrm_leave_repository_mock.go -package=mocks

/*
LeaveRequestListParams narrows the leave history. Every field is optional; the
zero value lists the whole business, which is what the approvals screen shows
once it filters to status=pending.
*/
type LeaveRequestListParams struct {
	ListParams

	EmployeeID  *uuid.UUID
	LeaveTypeID *uuid.UUID
	Status      string
	// Bounds on start_date — a request that begins inside the window.
	Range DateRange
}

/*
LeaveTaken is the per-type total a balance is derived from: approved days only,
within one calendar year.
*/
type LeaveTaken struct {
	LeaveTypeID uuid.UUID `gorm:"column:leave_type_id"`
	Days        float64   `gorm:"column:days"`
}

/*
LeaveDaysByPayType splits one employee's approved leave in a window into paid
and unpaid, which is the only distinction payroll cares about.
*/
type LeaveDaysByPayType struct {
	EmployeeID uuid.UUID `gorm:"column:employee_id"`
	PaidDays   float64   `gorm:"column:paid_days"`
	UnpaidDays float64   `gorm:"column:unpaid_days"`
}

/*
LeaveTypeRepository persists entitlement categories.

Reads return the business's own types alongside the system ones (business_id
NULL), the same way RoleRepository.ListForBusiness returns global system roles —
so a shop that has configured nothing still has somewhere to file sick leave.
*/
type LeaveTypeRepository interface {
	ListForBusiness(ctx context.Context, businessID uuid.UUID, includeInactive bool) ([]entity.LeaveType, error)
	FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.LeaveType, error)
	FindByCode(ctx context.Context, businessID uuid.UUID, code string) (*entity.LeaveType, error)
	Create(ctx context.Context, t *entity.LeaveType) error
	Update(ctx context.Context, businessID uuid.UUID, t *entity.LeaveType) error
	Delete(ctx context.Context, businessID, id uuid.UUID) error
}

type leaveTypeRepository struct {
	db *gorm.DB
}

func NewLeaveTypeRepository(db *gorm.DB) LeaveTypeRepository {
	return &leaveTypeRepository{db: db}
}

func (r *leaveTypeRepository) ListForBusiness(
	ctx context.Context, businessID uuid.UUID, includeInactive bool,
) ([]entity.LeaveType, error) {
	query := r.db.WithContext(ctx).
		Where("business_id = ? OR business_id IS NULL", businessID)
	if !includeInactive {
		query = query.Where("status = ?", entity.HRMStatusActive)
	}

	var rows []entity.LeaveType
	// System types first, so the familiar categories head the list.
	err := query.Order("is_system DESC, name").Find(&rows).Error
	return rows, err
}

func (r *leaveTypeRepository) FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.LeaveType, error) {
	var row entity.LeaveType
	err := r.db.WithContext(ctx).
		Where("id = ? AND (business_id = ? OR business_id IS NULL)", id, businessID).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *leaveTypeRepository) FindByCode(ctx context.Context, businessID uuid.UUID, code string) (*entity.LeaveType, error) {
	var row entity.LeaveType
	err := r.db.WithContext(ctx).
		Where("upper(code) = upper(?) AND (business_id = ? OR business_id IS NULL)", code, businessID).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *leaveTypeRepository) Create(ctx context.Context, t *entity.LeaveType) error {
	return r.db.WithContext(ctx).Create(t).Error
}

/*
Update and Delete match on business_id rather than allowing NULL: a shop may
configure its own types but must not edit or remove the system ones, which every
other tenant shares. A system id therefore reads as not found here, which is the
same answer another tenant's id gets.
*/
func (r *leaveTypeRepository) Update(ctx context.Context, businessID uuid.UUID, t *entity.LeaveType) error {
	res := r.db.WithContext(ctx).
		Model(&entity.LeaveType{}).
		Where("business_id = ? AND id = ?", businessID, t.ID).
		Updates(map[string]any{
			"name":              t.Name,
			"code":              t.Code,
			"annual_quota_days": t.AnnualQuotaDays,
			"is_paid":           t.IsPaid,
			"status":            t.Status,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *leaveTypeRepository) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		Delete(&entity.LeaveType{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

/*
LeaveRequestRepository persists applications and the decisions on them.
*/
type LeaveRequestRepository interface {
	List(ctx context.Context, businessID uuid.UUID, params LeaveRequestListParams) ([]entity.LeaveRequest, int64, error)
	FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.LeaveRequest, error)
	Create(ctx context.Context, req *entity.LeaveRequest) error
	Update(ctx context.Context, req *entity.LeaveRequest) error

	/*
		FindOverlapping returns the employee's pending or approved requests that
		intersect a date range, so a second application for days already booked
		is rejected rather than double-counted against the balance.
	*/
	FindOverlapping(
		ctx context.Context, businessID, employeeID uuid.UUID, start, end time.Time, excludeID *uuid.UUID,
	) ([]entity.LeaveRequest, error)

	/*
		SumApprovedByType totals approved days per leave type for one employee
		inside a window — the taken half of a leave balance.
	*/
	SumApprovedByType(
		ctx context.Context, businessID, employeeID uuid.UUID, r DateRange,
	) ([]LeaveTaken, error)

	/*
		SumApprovedByPayType splits approved days into paid and unpaid for a set
		of employees, in one query. Payroll runs this once per run rather than
		once per employee.
	*/
	SumApprovedByPayType(
		ctx context.Context, businessID uuid.UUID, r DateRange, employeeIDs []uuid.UUID,
	) ([]LeaveDaysByPayType, error)
}

type leaveRequestRepository struct {
	db *gorm.DB
}

func NewLeaveRequestRepository(db *gorm.DB) LeaveRequestRepository {
	return &leaveRequestRepository{db: db}
}

func (r *leaveRequestRepository) List(
	ctx context.Context, businessID uuid.UUID, params LeaveRequestListParams,
) ([]entity.LeaveRequest, int64, error) {
	var total int64
	if err := r.scoped(ctx, businessID, params).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entity.LeaveRequest{}, 0, nil
	}

	var rows []entity.LeaveRequest
	err := paginate(r.scoped(ctx, businessID, params), params.ListParams).
		Preload("Employee").
		Preload("LeaveType").
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *leaveRequestRepository) scoped(
	ctx context.Context, businessID uuid.UUID, params LeaveRequestListParams,
) *gorm.DB {
	query := r.db.WithContext(ctx).
		Model(&entity.LeaveRequest{}).
		Where("hrm_leave_requests.business_id = ?", businessID)

	if params.EmployeeID != nil {
		query = query.Where("hrm_leave_requests.employee_id = ?", *params.EmployeeID)
	}
	if params.LeaveTypeID != nil {
		query = query.Where("hrm_leave_requests.leave_type_id = ?", *params.LeaveTypeID)
	}
	if params.Status != "" {
		query = query.Where("hrm_leave_requests.status = ?", params.Status)
	}
	query = applyDateRange(query, "hrm_leave_requests.start_date", params.Range)

	if like := likePattern(params.Search); like != "" {
		query = query.Where(
			`EXISTS (
				SELECT 1 FROM hrm_employees e
				WHERE e.id = hrm_leave_requests.employee_id
				  AND (e.full_name ILIKE ? OR e.employee_code ILIKE ?)
			)`,
			like, like,
		)
	}

	return query
}

func (r *leaveRequestRepository) FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.LeaveRequest, error) {
	var row entity.LeaveRequest
	err := r.db.WithContext(ctx).
		Preload("Employee").
		Preload("LeaveType").
		Where("business_id = ? AND id = ?", businessID, id).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *leaveRequestRepository) Create(ctx context.Context, req *entity.LeaveRequest) error {
	return r.db.WithContext(ctx).Omit("Employee", "LeaveType").Create(req).Error
}

func (r *leaveRequestRepository) Update(ctx context.Context, req *entity.LeaveRequest) error {
	res := r.db.WithContext(ctx).
		Model(&entity.LeaveRequest{}).
		Where("business_id = ? AND id = ?", req.BusinessID, req.ID).
		Updates(map[string]any{
			"leave_type_id": req.LeaveTypeID,
			"start_date":    req.StartDate,
			"end_date":      req.EndDate,
			"days":          req.Days,
			"half_day":      req.HalfDay,
			"reason":        req.Reason,
			"status":        req.Status,
			"reviewed_by":   req.ReviewedBy,
			"reviewed_at":   req.ReviewedAt,
			"review_note":   req.ReviewNote,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *leaveRequestRepository) FindOverlapping(
	ctx context.Context, businessID, employeeID uuid.UUID, start, end time.Time, excludeID *uuid.UUID,
) ([]entity.LeaveRequest, error) {
	// Two ranges overlap when each starts before the other ends. Rejected and
	// cancelled requests are excluded: those days were handed back.
	query := r.db.WithContext(ctx).
		Where("business_id = ? AND employee_id = ?", businessID, employeeID).
		Where("status IN ?", []string{entity.LeaveStatusPending, entity.LeaveStatusApproved}).
		Where("start_date <= ? AND end_date >= ?", end, start)
	if excludeID != nil {
		query = query.Where("id <> ?", *excludeID)
	}

	var rows []entity.LeaveRequest
	err := query.Find(&rows).Error
	return rows, err
}

func (r *leaveRequestRepository) SumApprovedByType(
	ctx context.Context, businessID, employeeID uuid.UUID, dr DateRange,
) ([]LeaveTaken, error) {
	query := r.db.WithContext(ctx).
		Model(&entity.LeaveRequest{}).
		Select("leave_type_id, COALESCE(SUM(days), 0) AS days").
		Where("business_id = ? AND employee_id = ? AND status = ?",
			businessID, employeeID, entity.LeaveStatusApproved).
		Group("leave_type_id")
	query = applyDateRange(query, "start_date", dr)

	var rows []LeaveTaken
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *leaveRequestRepository) SumApprovedByPayType(
	ctx context.Context, businessID uuid.UUID, dr DateRange, employeeIDs []uuid.UUID,
) ([]LeaveDaysByPayType, error) {
	query := r.db.WithContext(ctx).
		Model(&entity.LeaveRequest{}).
		Select(`hrm_leave_requests.employee_id,
			COALESCE(SUM(CASE WHEN t.is_paid THEN hrm_leave_requests.days ELSE 0 END), 0) AS paid_days,
			COALESCE(SUM(CASE WHEN t.is_paid THEN 0 ELSE hrm_leave_requests.days END), 0) AS unpaid_days`).
		Joins("JOIN hrm_leave_types t ON t.id = hrm_leave_requests.leave_type_id").
		Where("hrm_leave_requests.business_id = ? AND hrm_leave_requests.status = ?",
			businessID, entity.LeaveStatusApproved).
		Group("hrm_leave_requests.employee_id")

	query = applyDateRange(query, "hrm_leave_requests.start_date", dr)
	if len(employeeIDs) > 0 {
		query = query.Where("hrm_leave_requests.employee_id IN ?", employeeIDs)
	}

	var rows []LeaveDaysByPayType
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
