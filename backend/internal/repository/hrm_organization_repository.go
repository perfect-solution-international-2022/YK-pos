package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

//go:generate go run go.uber.org/mock/mockgen -source=hrm_organization_repository.go -destination=mocks/hrm_organization_repository_mock.go -package=mocks

/*
ErrInUse means the row is still referenced by another record and cannot be
removed. Deleting a designation out from under the people holding it would
leave payroll unable to say what anyone's job was, so the FK is RESTRICT and
this is what the caller reports as a 409.
*/
var ErrInUse = errors.New("repository: record is still referenced")

/*
DesignationRepository persists job titles.

Every read is scoped by business_id in the same WHERE clause as the id, so a
cross-tenant lookup returns ErrNotFound rather than another shop's row.
*/
type DesignationRepository interface {
	List(ctx context.Context, businessID uuid.UUID, params ListParams, status string) ([]entity.Designation, int64, error)
	// ListActive returns every active title unpaginated, for the employee
	// form's dropdown — a shop has tens of these, not thousands.
	ListActive(ctx context.Context, businessID uuid.UUID) ([]entity.Designation, error)
	FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Designation, error)
	// FindByName resolves the uniqueness check before the partial unique index
	// rejects the insert with a driver error nothing above can interpret.
	FindByName(ctx context.Context, businessID uuid.UUID, name string) (*entity.Designation, error)
	Create(ctx context.Context, d *entity.Designation) error
	Update(ctx context.Context, d *entity.Designation) error
	// Delete soft-deletes, and reports ErrInUse when employees still hold the
	// title.
	Delete(ctx context.Context, businessID, id uuid.UUID) error
	CountEmployees(ctx context.Context, businessID, designationID uuid.UUID) (int64, error)
}

type designationRepository struct {
	db *gorm.DB
}

func NewDesignationRepository(db *gorm.DB) DesignationRepository {
	return &designationRepository{db: db}
}

func (r *designationRepository) List(
	ctx context.Context, businessID uuid.UUID, params ListParams, status string,
) ([]entity.Designation, int64, error) {
	scoped := func() *gorm.DB {
		q := r.db.WithContext(ctx).
			Model(&entity.Designation{}).
			Where("business_id = ?", businessID)
		if status != "" {
			q = q.Where("status = ?", status)
		}
		if like := likePattern(params.Search); like != "" {
			q = q.Where("(name ILIKE ? OR description ILIKE ?)", like, like)
		}
		return q
	}

	var total int64
	if err := scoped().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entity.Designation{}, 0, nil
	}

	var rows []entity.Designation
	if err := paginate(scoped(), params).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *designationRepository) ListActive(ctx context.Context, businessID uuid.UUID) ([]entity.Designation, error) {
	var rows []entity.Designation
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND status = ?", businessID, entity.HRMStatusActive).
		Order("name").
		Find(&rows).Error
	return rows, err
}

func (r *designationRepository) FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Designation, error) {
	var row entity.Designation
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *designationRepository) FindByName(ctx context.Context, businessID uuid.UUID, name string) (*entity.Designation, error) {
	var row entity.Designation
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND lower(name) = lower(?)", businessID, name).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *designationRepository) Create(ctx context.Context, d *entity.Designation) error {
	return r.db.WithContext(ctx).Create(d).Error
}

/*
Update writes an explicit column map rather than saving the struct: a struct
update skips zero values, so clearing a description would be silently dropped.
updated_at is left to the table's trigger.
*/
func (r *designationRepository) Update(ctx context.Context, d *entity.Designation) error {
	res := r.db.WithContext(ctx).
		Model(&entity.Designation{}).
		Where("business_id = ? AND id = ?", d.BusinessID, d.ID).
		Updates(map[string]any{
			"name":        d.Name,
			"description": d.Description,
			"status":      d.Status,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *designationRepository) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		Delete(&entity.Designation{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *designationRepository) CountEmployees(ctx context.Context, businessID, designationID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.Employee{}).
		Where("business_id = ? AND designation_id = ?", businessID, designationID).
		Count(&count).Error
	return count, err
}

/*
ShiftRepository persists working patterns.
*/
type ShiftRepository interface {
	List(ctx context.Context, businessID uuid.UUID, params ListParams, status string) ([]entity.Shift, int64, error)
	ListActive(ctx context.Context, businessID uuid.UUID) ([]entity.Shift, error)
	FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Shift, error)
	FindByName(ctx context.Context, businessID uuid.UUID, name string) (*entity.Shift, error)
	Create(ctx context.Context, s *entity.Shift) error
	Update(ctx context.Context, s *entity.Shift) error
	Delete(ctx context.Context, businessID, id uuid.UUID) error
	CountEmployees(ctx context.Context, businessID, shiftID uuid.UUID) (int64, error)
}

type shiftRepository struct {
	db *gorm.DB
}

func NewShiftRepository(db *gorm.DB) ShiftRepository {
	return &shiftRepository{db: db}
}

func (r *shiftRepository) List(
	ctx context.Context, businessID uuid.UUID, params ListParams, status string,
) ([]entity.Shift, int64, error) {
	scoped := func() *gorm.DB {
		q := r.db.WithContext(ctx).
			Model(&entity.Shift{}).
			Where("business_id = ?", businessID)
		if status != "" {
			q = q.Where("status = ?", status)
		}
		if like := likePattern(params.Search); like != "" {
			q = q.Where("name ILIKE ?", like)
		}
		return q
	}

	var total int64
	if err := scoped().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entity.Shift{}, 0, nil
	}

	var rows []entity.Shift
	if err := paginate(scoped(), params).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *shiftRepository) ListActive(ctx context.Context, businessID uuid.UUID) ([]entity.Shift, error) {
	var rows []entity.Shift
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND status = ?", businessID, entity.HRMStatusActive).
		Order("start_time, name").
		Find(&rows).Error
	return rows, err
}

func (r *shiftRepository) FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Shift, error) {
	var row entity.Shift
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *shiftRepository) FindByName(ctx context.Context, businessID uuid.UUID, name string) (*entity.Shift, error) {
	var row entity.Shift
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND lower(name) = lower(?)", businessID, name).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *shiftRepository) Create(ctx context.Context, s *entity.Shift) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *shiftRepository) Update(ctx context.Context, s *entity.Shift) error {
	res := r.db.WithContext(ctx).
		Model(&entity.Shift{}).
		Where("business_id = ? AND id = ?", s.BusinessID, s.ID).
		Updates(map[string]any{
			"name":          s.Name,
			"start_time":    s.StartTime,
			"end_time":      s.EndTime,
			"break_minutes": s.BreakMinutes,
			"grace_minutes": s.GraceMinutes,
			"weekly_off":    s.WeeklyOff,
			"status":        s.Status,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *shiftRepository) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		Delete(&entity.Shift{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *shiftRepository) CountEmployees(ctx context.Context, businessID, shiftID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.Employee{}).
		Where("business_id = ? AND shift_id = ?", businessID, shiftID).
		Count(&count).Error
	return count, err
}
