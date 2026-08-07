package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

//go:generate go run go.uber.org/mock/mockgen -source=hrm_performance_repository.go -destination=mocks/hrm_performance_repository_mock.go -package=mocks

/*
PerformanceListParams narrows the evaluation history.
*/
type PerformanceListParams struct {
	ListParams

	EmployeeID  *uuid.UUID
	PeriodYear  *int
	PeriodMonth *int
}

/*
AnnouncementListParams narrows the noticeboard. PublishedOn filters to notices
current on a given day — published, on or before it, and not yet expired —
which is what a staff-facing list asks for.
*/
type AnnouncementListParams struct {
	ListParams

	Status      string
	PublishedOn *time.Time
}

/*
PerformanceRepository persists monthly evaluations.
*/
type PerformanceRepository interface {
	List(ctx context.Context, businessID uuid.UUID, params PerformanceListParams) ([]entity.PerformanceReview, int64, error)
	FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.PerformanceReview, error)
	// FindByPeriod backs the "one review per employee per month" rule, so a
	// second save is reported as a conflict rather than rejected by the unique
	// index with a driver error nothing above can interpret.
	FindByPeriod(ctx context.Context, businessID, employeeID uuid.UUID, year, month int) (*entity.PerformanceReview, error)
	Create(ctx context.Context, r *entity.PerformanceReview) error
	Update(ctx context.Context, r *entity.PerformanceReview) error
	Delete(ctx context.Context, businessID, id uuid.UUID) error
}

type performanceRepository struct {
	db *gorm.DB
}

func NewPerformanceRepository(db *gorm.DB) PerformanceRepository {
	return &performanceRepository{db: db}
}

func (r *performanceRepository) List(
	ctx context.Context, businessID uuid.UUID, params PerformanceListParams,
) ([]entity.PerformanceReview, int64, error) {
	scoped := func() *gorm.DB {
		q := r.db.WithContext(ctx).
			Model(&entity.PerformanceReview{}).
			Where("hrm_performance_reviews.business_id = ?", businessID)
		if params.EmployeeID != nil {
			q = q.Where("hrm_performance_reviews.employee_id = ?", *params.EmployeeID)
		}
		if params.PeriodYear != nil {
			q = q.Where("hrm_performance_reviews.period_year = ?", *params.PeriodYear)
		}
		if params.PeriodMonth != nil {
			q = q.Where("hrm_performance_reviews.period_month = ?", *params.PeriodMonth)
		}
		if like := likePattern(params.Search); like != "" {
			q = q.Where(
				`EXISTS (
					SELECT 1 FROM hrm_employees e
					WHERE e.id = hrm_performance_reviews.employee_id
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
		return []entity.PerformanceReview{}, 0, nil
	}

	var rows []entity.PerformanceReview
	if err := paginate(scoped(), params.ListParams).Preload("Employee").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *performanceRepository) FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.PerformanceReview, error) {
	var row entity.PerformanceReview
	err := r.db.WithContext(ctx).
		Preload("Employee").
		Where("business_id = ? AND id = ?", businessID, id).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *performanceRepository) FindByPeriod(
	ctx context.Context, businessID, employeeID uuid.UUID, year, month int,
) (*entity.PerformanceReview, error) {
	var row entity.PerformanceReview
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND employee_id = ? AND period_year = ? AND period_month = ?",
			businessID, employeeID, year, month).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *performanceRepository) Create(ctx context.Context, review *entity.PerformanceReview) error {
	return r.db.WithContext(ctx).Omit("Employee").Create(review).Error
}

func (r *performanceRepository) Update(ctx context.Context, review *entity.PerformanceReview) error {
	res := r.db.WithContext(ctx).
		Model(&entity.PerformanceReview{}).
		Where("business_id = ? AND id = ?", review.BusinessID, review.ID).
		Updates(map[string]any{
			"rating":        review.Rating,
			"punctuality":   review.Punctuality,
			"teamwork":      review.Teamwork,
			"productivity":  review.Productivity,
			"manager_notes": review.ManagerNotes,
			"reviewer_id":   review.ReviewerID,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *performanceRepository) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		Delete(&entity.PerformanceReview{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

/*
AnnouncementRepository persists staff notices.
*/
type AnnouncementRepository interface {
	List(ctx context.Context, businessID uuid.UUID, params AnnouncementListParams) ([]entity.Announcement, int64, error)
	FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Announcement, error)
	Create(ctx context.Context, a *entity.Announcement) error
	Update(ctx context.Context, a *entity.Announcement) error
	Delete(ctx context.Context, businessID, id uuid.UUID) error
}

type announcementRepository struct {
	db *gorm.DB
}

func NewAnnouncementRepository(db *gorm.DB) AnnouncementRepository {
	return &announcementRepository{db: db}
}

func (r *announcementRepository) List(
	ctx context.Context, businessID uuid.UUID, params AnnouncementListParams,
) ([]entity.Announcement, int64, error) {
	scoped := func() *gorm.DB {
		q := r.db.WithContext(ctx).
			Model(&entity.Announcement{}).
			Where("business_id = ?", businessID)
		if params.Status != "" {
			q = q.Where("status = ?", params.Status)
		}
		if params.PublishedOn != nil {
			q = q.Where("status = ?", entity.AnnouncementStatusPublished).
				Where("publish_date <= ?", *params.PublishedOn).
				Where("(expires_on IS NULL OR expires_on >= ?)", *params.PublishedOn)
		}
		if like := likePattern(params.Search); like != "" {
			q = q.Where("(title ILIKE ? OR description ILIKE ?)", like, like)
		}
		return q
	}

	var total int64
	if err := scoped().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entity.Announcement{}, 0, nil
	}

	var rows []entity.Announcement
	if err := paginate(scoped(), params.ListParams).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *announcementRepository) FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Announcement, error) {
	var row entity.Announcement
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *announcementRepository) Create(ctx context.Context, a *entity.Announcement) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *announcementRepository) Update(ctx context.Context, a *entity.Announcement) error {
	res := r.db.WithContext(ctx).
		Model(&entity.Announcement{}).
		Where("business_id = ? AND id = ?", a.BusinessID, a.ID).
		Updates(map[string]any{
			"title":        a.Title,
			"description":  a.Description,
			"publish_date": a.PublishDate,
			"expires_on":   a.ExpiresOn,
			"status":       a.Status,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *announcementRepository) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		Delete(&entity.Announcement{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
