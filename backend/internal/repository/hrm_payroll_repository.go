package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

//go:generate go run go.uber.org/mock/mockgen -source=hrm_payroll_repository.go -destination=mocks/hrm_payroll_repository_mock.go -package=mocks

/*
PayslipListParams narrows payslip history. Year and month are pointers rather
than ints because 0 is not distinguishable from "not filtered" for a month.
*/
type PayslipListParams struct {
	ListParams

	EmployeeID  *uuid.UUID
	PeriodYear  *int
	PeriodMonth *int
	Status      string
}

/*
PayrollRepository persists monthly runs and the payslips inside them.

A run and its payslips are written together: generating a month is one
transaction, so a half-written run — some staff paid, some not — is never
visible to a report.
*/
type PayrollRepository interface {
	ListRuns(ctx context.Context, businessID uuid.UUID, params ListParams) ([]entity.PayrollRun, int64, error)
	FindRunByID(ctx context.Context, businessID, id uuid.UUID) (*entity.PayrollRun, error)
	FindRunByPeriod(ctx context.Context, businessID uuid.UUID, year, month int) (*entity.PayrollRun, error)
	CreateRun(ctx context.Context, tx *gorm.DB, run *entity.PayrollRun) error
	UpdateRunStatus(ctx context.Context, businessID, id uuid.UUID, run *entity.PayrollRun) error
	DeleteRun(ctx context.Context, businessID, id uuid.UUID) error

	// ReplacePayslips clears a draft run's payslips and writes the new set, so
	// regenerating a month cannot leave a payslip behind for someone who has
	// since left.
	ReplacePayslips(ctx context.Context, tx *gorm.DB, runID uuid.UUID, payslips []entity.Payslip) error
	ListPayslips(ctx context.Context, businessID uuid.UUID, params PayslipListParams) ([]entity.Payslip, int64, error)
	FindPayslipByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Payslip, error)
	UpdatePayslip(ctx context.Context, tx *gorm.DB, p *entity.Payslip) error
	ReplacePayslipItems(ctx context.Context, tx *gorm.DB, payslipID uuid.UUID, items []entity.PayslipItem) error
	// SetPayslipStatusForRun moves every payslip in a run at once, so finalising
	// cannot leave individual slips in draft.
	SetPayslipStatusForRun(ctx context.Context, tx *gorm.DB, runID uuid.UUID, status string) error
}

type payrollRepository struct {
	db *gorm.DB
}

func NewPayrollRepository(db *gorm.DB) PayrollRepository {
	return &payrollRepository{db: db}
}

func (r *payrollRepository) ListRuns(
	ctx context.Context, businessID uuid.UUID, params ListParams,
) ([]entity.PayrollRun, int64, error) {
	scoped := func() *gorm.DB {
		return r.db.WithContext(ctx).
			Model(&entity.PayrollRun{}).
			Where("business_id = ?", businessID)
	}

	var total int64
	if err := scoped().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entity.PayrollRun{}, 0, nil
	}

	// Payslips are not preloaded: the runs list shows periods and totals, and a
	// year of runs would otherwise drag every payslip of every month with it.
	var rows []entity.PayrollRun
	if err := paginate(scoped(), params).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *payrollRepository) FindRunByID(ctx context.Context, businessID, id uuid.UUID) (*entity.PayrollRun, error) {
	var row entity.PayrollRun
	err := r.db.WithContext(ctx).
		Preload("Payslips", func(db *gorm.DB) *gorm.DB {
			return db.Order("hrm_payslips.created_at")
		}).
		Preload("Payslips.Employee").
		Preload("Payslips.Items").
		Where("business_id = ? AND id = ?", businessID, id).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *payrollRepository) FindRunByPeriod(
	ctx context.Context, businessID uuid.UUID, year, month int,
) (*entity.PayrollRun, error) {
	var row entity.PayrollRun
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND period_year = ? AND period_month = ?", businessID, year, month).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *payrollRepository) CreateRun(ctx context.Context, tx *gorm.DB, run *entity.PayrollRun) error {
	return r.conn(ctx, tx).Omit("Payslips").Create(run).Error
}

func (r *payrollRepository) UpdateRunStatus(
	ctx context.Context, businessID, id uuid.UUID, run *entity.PayrollRun,
) error {
	res := r.db.WithContext(ctx).
		Model(&entity.PayrollRun{}).
		Where("business_id = ? AND id = ?", businessID, id).
		Updates(map[string]any{
			"status":       run.Status,
			"notes":        run.Notes,
			"finalized_at": run.FinalizedAt,
			"paid_at":      run.PaidAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *payrollRepository) DeleteRun(ctx context.Context, businessID, id uuid.UUID) error {
	// Hard delete, unlike the master-data tables: a draft run holds no history
	// anyone has been shown, and its payslips cascade away with it.
	res := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		Delete(&entity.PayrollRun{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *payrollRepository) ReplacePayslips(
	ctx context.Context, tx *gorm.DB, runID uuid.UUID, payslips []entity.Payslip,
) error {
	db := r.conn(ctx, tx)
	if err := db.Where("payroll_run_id = ?", runID).Delete(&entity.Payslip{}).Error; err != nil {
		return err
	}
	if len(payslips) == 0 {
		return nil
	}
	for i := range payslips {
		payslips[i].PayrollRunID = runID
	}
	return db.Omit("Employee", "Items").Create(&payslips).Error
}

func (r *payrollRepository) ListPayslips(
	ctx context.Context, businessID uuid.UUID, params PayslipListParams,
) ([]entity.Payslip, int64, error) {
	scoped := func() *gorm.DB {
		q := r.db.WithContext(ctx).
			Model(&entity.Payslip{}).
			Where("hrm_payslips.business_id = ?", businessID)
		if params.EmployeeID != nil {
			q = q.Where("hrm_payslips.employee_id = ?", *params.EmployeeID)
		}
		if params.PeriodYear != nil {
			q = q.Where("hrm_payslips.period_year = ?", *params.PeriodYear)
		}
		if params.PeriodMonth != nil {
			q = q.Where("hrm_payslips.period_month = ?", *params.PeriodMonth)
		}
		if params.Status != "" {
			q = q.Where("hrm_payslips.status = ?", params.Status)
		}
		if like := likePattern(params.Search); like != "" {
			q = q.Where(
				`EXISTS (
					SELECT 1 FROM hrm_employees e
					WHERE e.id = hrm_payslips.employee_id
					  AND (e.full_name ILIKE ? OR e.employee_code ILIKE ?)
				)`,
				like, like,
			)
		}
		return q
	}

	var total int64
	if err := scoped().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entity.Payslip{}, 0, nil
	}

	var rows []entity.Payslip
	err := paginate(scoped(), params.ListParams).
		Preload("Employee").
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *payrollRepository) FindPayslipByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Payslip, error) {
	var row entity.Payslip
	err := r.db.WithContext(ctx).
		Preload("Employee").
		Preload("Items").
		Where("business_id = ? AND id = ?", businessID, id).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *payrollRepository) UpdatePayslip(ctx context.Context, tx *gorm.DB, p *entity.Payslip) error {
	res := r.conn(ctx, tx).
		Model(&entity.Payslip{}).
		Where("business_id = ? AND id = ?", p.BusinessID, p.ID).
		Updates(map[string]any{
			"bonus_cents":           p.BonusCents,
			"allowance_cents":       p.AllowanceCents,
			"other_deduction_cents": p.OtherDeductionCents,
			"gross_cents":           p.GrossCents,
			"deduction_cents":       p.DeductionCents,
			"net_cents":             p.NetCents,
			"status":                p.Status,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *payrollRepository) ReplacePayslipItems(
	ctx context.Context, tx *gorm.DB, payslipID uuid.UUID, items []entity.PayslipItem,
) error {
	db := r.conn(ctx, tx)
	if err := db.Where("payslip_id = ?", payslipID).Delete(&entity.PayslipItem{}).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		items[i].PayslipID = payslipID
	}
	return db.Create(&items).Error
}

func (r *payrollRepository) SetPayslipStatusForRun(
	ctx context.Context, tx *gorm.DB, runID uuid.UUID, status string,
) error {
	return r.conn(ctx, tx).
		Model(&entity.Payslip{}).
		Where("payroll_run_id = ?", runID).
		Update("status", status).Error
}

func (r *payrollRepository) conn(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
