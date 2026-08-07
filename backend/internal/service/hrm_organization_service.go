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
DesignationService owns job titles.

A designation is referenced by every employee holding it, so the two rules worth
stating are here rather than in the handler: a name is unique within a business
(case-insensitively, because "Cashier" and "cashier" are the same job), and a
title still held by somebody cannot be deleted.
*/
type DesignationService interface {
	List(ctx context.Context, businessID uuid.UUID, query DesignationQuery) ([]entity.Designation, int64, error)
	Get(ctx context.Context, businessID, id uuid.UUID) (*entity.Designation, error)
	Create(ctx context.Context, businessID uuid.UUID, d *entity.Designation) (*entity.Designation, error)
	Update(ctx context.Context, businessID, id uuid.UUID, apply func(*entity.Designation)) (*entity.Designation, error)
	Delete(ctx context.Context, businessID, id uuid.UUID) error
}

type designationService struct {
	designations repository.DesignationRepository
}

func NewDesignationService(designations repository.DesignationRepository) DesignationService {
	return &designationService{designations: designations}
}

func (s *designationService) List(
	ctx context.Context, businessID uuid.UUID, query DesignationQuery,
) ([]entity.Designation, int64, error) {
	rows, total, err := s.designations.List(ctx, businessID, query.toParams(), query.Status)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "failed to list designations", err)
	}
	return rows, total, nil
}

func (s *designationService) Get(ctx context.Context, businessID, id uuid.UUID) (*entity.Designation, error) {
	row, err := s.designations.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "designation not found", "failed to load designation")
	}
	return row, nil
}

func (s *designationService) Create(
	ctx context.Context, businessID uuid.UUID, d *entity.Designation,
) (*entity.Designation, error) {
	d.BusinessID = businessID
	if d.Status == "" {
		d.Status = entity.HRMStatusActive
	}

	if err := s.assertNameFree(ctx, businessID, d.Name, uuid.Nil); err != nil {
		return nil, err
	}
	if err := s.designations.Create(ctx, d); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to create designation", err)
	}
	return d, nil
}

/*
Update loads the stored row first and applies the edit to it, the same way
ProductService.Update does — so the response is the server's copy rather than
whatever the client believed it held, and columns the request does not carry are
never overwritten from a stale form.
*/
func (s *designationService) Update(
	ctx context.Context, businessID, id uuid.UUID, apply func(*entity.Designation),
) (*entity.Designation, error) {
	row, err := s.designations.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "designation not found", "failed to load designation")
	}

	apply(row)

	if err := s.assertNameFree(ctx, businessID, row.Name, row.ID); err != nil {
		return nil, err
	}
	if err := s.designations.Update(ctx, row); err != nil {
		return nil, mapNotFound(err, "designation not found", "failed to update designation")
	}
	return row, nil
}

/*
Delete refuses while employees still hold the title.

Checked here rather than left to the FK: hrm_employees.designation_id is ON
DELETE RESTRICT, so the database would reject it anyway — but as a driver error
carrying a constraint name, which no layer above could turn into a sentence a
manager can act on.
*/
func (s *designationService) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	count, err := s.designations.CountEmployees(ctx, businessID, id)
	if err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "failed to check designation usage", err)
	}
	if count > 0 {
		return apperror.New(
			apperror.CodeConflict,
			"this designation is still assigned to employees; reassign them first",
		)
	}

	if err := s.designations.Delete(ctx, businessID, id); err != nil {
		return mapNotFound(err, "designation not found", "failed to delete designation")
	}
	return nil
}

func (s *designationService) assertNameFree(
	ctx context.Context, businessID uuid.UUID, name string, selfID uuid.UUID,
) error {
	existing, err := s.designations.FindByName(ctx, businessID, name)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return nil
	case err != nil:
		return apperror.Wrap(apperror.CodeDatabase, "failed to check existing designation", err)
	case existing.ID == selfID:
		// An edit that leaves the name alone finds itself.
		return nil
	default:
		return apperror.New(apperror.CodeConflict, "a designation with this name already exists")
	}
}

/*
ShiftService owns working patterns.
*/
type ShiftService interface {
	List(ctx context.Context, businessID uuid.UUID, query ShiftQuery) ([]entity.Shift, int64, error)
	Get(ctx context.Context, businessID, id uuid.UUID) (*entity.Shift, error)
	Create(ctx context.Context, businessID uuid.UUID, sh *entity.Shift) (*entity.Shift, error)
	Update(ctx context.Context, businessID, id uuid.UUID, apply func(*entity.Shift)) (*entity.Shift, error)
	Delete(ctx context.Context, businessID, id uuid.UUID) error
}

type shiftService struct {
	shifts repository.ShiftRepository
}

func NewShiftService(shifts repository.ShiftRepository) ShiftService {
	return &shiftService{shifts: shifts}
}

func (s *shiftService) List(
	ctx context.Context, businessID uuid.UUID, query ShiftQuery,
) ([]entity.Shift, int64, error) {
	rows, total, err := s.shifts.List(ctx, businessID, query.toParams(), query.Status)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "failed to list shifts", err)
	}
	return rows, total, nil
}

func (s *shiftService) Get(ctx context.Context, businessID, id uuid.UUID) (*entity.Shift, error) {
	row, err := s.shifts.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "shift not found", "failed to load shift")
	}
	return row, nil
}

func (s *shiftService) Create(
	ctx context.Context, businessID uuid.UUID, sh *entity.Shift,
) (*entity.Shift, error) {
	sh.BusinessID = businessID
	if sh.Status == "" {
		sh.Status = entity.HRMStatusActive
	}

	if err := s.validate(sh); err != nil {
		return nil, err
	}
	if err := s.assertNameFree(ctx, businessID, sh.Name, uuid.Nil); err != nil {
		return nil, err
	}
	if err := s.shifts.Create(ctx, sh); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to create shift", err)
	}
	return sh, nil
}

func (s *shiftService) Update(
	ctx context.Context, businessID, id uuid.UUID, apply func(*entity.Shift),
) (*entity.Shift, error) {
	row, err := s.shifts.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "shift not found", "failed to load shift")
	}

	apply(row)

	if err := s.validate(row); err != nil {
		return nil, err
	}
	if err := s.assertNameFree(ctx, businessID, row.Name, row.ID); err != nil {
		return nil, err
	}
	if err := s.shifts.Update(ctx, row); err != nil {
		return nil, mapNotFound(err, "shift not found", "failed to update shift")
	}
	return row, nil
}

func (s *shiftService) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	count, err := s.shifts.CountEmployees(ctx, businessID, id)
	if err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "failed to check shift usage", err)
	}
	if count > 0 {
		return apperror.New(
			apperror.CodeConflict,
			"this shift is still assigned to employees; move them to another shift first",
		)
	}

	if err := s.shifts.Delete(ctx, businessID, id); err != nil {
		return mapNotFound(err, "shift not found", "failed to delete shift")
	}
	return nil
}

/*
validate rejects a shift whose break swallows the whole span. The times
themselves are not compared — an end at or before the start is a night shift,
not an error — so the only impossible combination is one that leaves no paid
minutes, which would make every attendance row against it read as overtime.
*/
func (s *shiftService) validate(sh *entity.Shift) error {
	if sh.ScheduledMinutes() <= 0 {
		return apperror.New(
			apperror.CodeValidationError,
			"the break time is longer than the shift itself",
		)
	}
	return nil
}

func (s *shiftService) assertNameFree(
	ctx context.Context, businessID uuid.UUID, name string, selfID uuid.UUID,
) error {
	existing, err := s.shifts.FindByName(ctx, businessID, name)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return nil
	case err != nil:
		return apperror.Wrap(apperror.CodeDatabase, "failed to check existing shift", err)
	case existing.ID == selfID:
		return nil
	default:
		return apperror.New(apperror.CodeConflict, "a shift with this name already exists")
	}
}

/*
mapNotFound turns a repository error into the AppError the handler layer
returns, so every HRM service reports a missing row the same way.

A cross-tenant id lands on repository.ErrNotFound exactly as a nonexistent one
does — both must map to 404, never 403, or the response leaks that the record
exists in another shop.
*/
func mapNotFound(err error, notFoundMessage, wrapMessage string) error {
	if errors.Is(err, repository.ErrNotFound) {
		return apperror.New(apperror.CodeNotFound, notFoundMessage)
	}
	return apperror.Wrap(apperror.CodeDatabase, wrapMessage, err)
}
