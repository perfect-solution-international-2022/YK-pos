package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
)

/*
PerformanceService owns monthly evaluations.
*/
type PerformanceService interface {
	List(ctx context.Context, businessID uuid.UUID, query PerformanceQuery) ([]entity.PerformanceReview, int64, error)
	Get(ctx context.Context, businessID, id uuid.UUID) (*entity.PerformanceReview, error)
	Create(ctx context.Context, businessID uuid.UUID, r *entity.PerformanceReview) (*entity.PerformanceReview, error)
	Update(ctx context.Context, businessID, id uuid.UUID, apply func(*entity.PerformanceReview)) (*entity.PerformanceReview, error)
	Delete(ctx context.Context, businessID, id uuid.UUID) error
}

type performanceService struct {
	reviews   repository.PerformanceRepository
	employees repository.EmployeeRepository
}

func NewPerformanceService(
	reviews repository.PerformanceRepository, employees repository.EmployeeRepository,
) PerformanceService {
	return &performanceService{reviews: reviews, employees: employees}
}

func (s *performanceService) List(
	ctx context.Context, businessID uuid.UUID, query PerformanceQuery,
) ([]entity.PerformanceReview, int64, error) {
	rows, total, err := s.reviews.List(ctx, businessID, query.toParams())
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "failed to list performance reviews", err)
	}
	return rows, total, nil
}

func (s *performanceService) Get(ctx context.Context, businessID, id uuid.UUID) (*entity.PerformanceReview, error) {
	row, err := s.reviews.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "performance review not found", "failed to load performance review")
	}
	return row, nil
}

/*
Create files an evaluation.

One per employee per month: a second submission for a period already reviewed is
a conflict pointing at the existing one, not a new row. Without that, an
employee's average rating would depend on how many times their manager pressed
save.
*/
func (s *performanceService) Create(
	ctx context.Context, businessID uuid.UUID, review *entity.PerformanceReview,
) (*entity.PerformanceReview, error) {
	review.BusinessID = businessID

	if _, err := s.employees.FindByID(ctx, businessID, review.EmployeeID); err != nil {
		return nil, mapNotFound(err, "employee not found", "failed to load employee")
	}

	_, err := s.reviews.FindByPeriod(ctx, businessID, review.EmployeeID, review.PeriodYear, review.PeriodMonth)
	switch {
	case err == nil:
		return nil, apperror.New(
			apperror.CodeConflict,
			"this employee has already been reviewed for that month",
		)
	case !errors.Is(err, repository.ErrNotFound):
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to check existing review", err)
	}

	if err := s.reviews.Create(ctx, review); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to create performance review", err)
	}
	return s.Get(ctx, businessID, review.ID)
}

/*
Update edits an existing evaluation. The period and the employee are not
editable: moving a review to another month is filing a different review, and
would collide with whatever already sits there.
*/
func (s *performanceService) Update(
	ctx context.Context, businessID, id uuid.UUID, apply func(*entity.PerformanceReview),
) (*entity.PerformanceReview, error) {
	row, err := s.reviews.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "performance review not found", "failed to load performance review")
	}

	apply(row)

	if err := s.reviews.Update(ctx, row); err != nil {
		return nil, mapNotFound(err, "performance review not found", "failed to update performance review")
	}
	return s.Get(ctx, businessID, id)
}

func (s *performanceService) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	if err := s.reviews.Delete(ctx, businessID, id); err != nil {
		return mapNotFound(err, "performance review not found", "failed to delete performance review")
	}
	return nil
}

/*
AnnouncementService owns staff notices.
*/
type AnnouncementService interface {
	List(ctx context.Context, businessID uuid.UUID, query AnnouncementQuery) ([]entity.Announcement, int64, error)
	Get(ctx context.Context, businessID, id uuid.UUID) (*entity.Announcement, error)
	Create(ctx context.Context, businessID uuid.UUID, a *entity.Announcement) (*entity.Announcement, error)
	Update(ctx context.Context, businessID, id uuid.UUID, apply func(*entity.Announcement)) (*entity.Announcement, error)
	Delete(ctx context.Context, businessID, id uuid.UUID) error
}

type announcementService struct {
	announcements repository.AnnouncementRepository
}

func NewAnnouncementService(announcements repository.AnnouncementRepository) AnnouncementService {
	return &announcementService{announcements: announcements}
}

func (s *announcementService) List(
	ctx context.Context, businessID uuid.UUID, query AnnouncementQuery,
) ([]entity.Announcement, int64, error) {
	rows, total, err := s.announcements.List(ctx, businessID, query.toParams())
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "failed to list announcements", err)
	}
	return rows, total, nil
}

func (s *announcementService) Get(ctx context.Context, businessID, id uuid.UUID) (*entity.Announcement, error) {
	row, err := s.announcements.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "announcement not found", "failed to load announcement")
	}
	return row, nil
}

func (s *announcementService) Create(
	ctx context.Context, businessID uuid.UUID, a *entity.Announcement,
) (*entity.Announcement, error) {
	a.BusinessID = businessID
	if a.Status == "" {
		a.Status = entity.AnnouncementStatusDraft
	}
	if a.ExpiresOn != nil && a.ExpiresOn.Before(a.PublishDate) {
		return nil, apperror.New(apperror.CodeValidationError, "the expiry date is before the publish date")
	}

	if err := s.announcements.Create(ctx, a); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to create announcement", err)
	}
	return a, nil
}

func (s *announcementService) Update(
	ctx context.Context, businessID, id uuid.UUID, apply func(*entity.Announcement),
) (*entity.Announcement, error) {
	row, err := s.announcements.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "announcement not found", "failed to load announcement")
	}

	apply(row)

	if row.ExpiresOn != nil && row.ExpiresOn.Before(row.PublishDate) {
		return nil, apperror.New(apperror.CodeValidationError, "the expiry date is before the publish date")
	}
	if err := s.announcements.Update(ctx, row); err != nil {
		return nil, mapNotFound(err, "announcement not found", "failed to update announcement")
	}
	return row, nil
}

func (s *announcementService) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	if err := s.announcements.Delete(ctx, businessID, id); err != nil {
		return mapNotFound(err, "announcement not found", "failed to delete announcement")
	}
	return nil
}
