package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/mapper"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/middleware"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/pagination"
)

/*
DesignationHandler serves the job-title endpoints.
*/
type DesignationHandler struct {
	designations service.DesignationService
}

func NewDesignationHandler(designations service.DesignationService) *DesignationHandler {
	return &DesignationHandler{designations: designations}
}

// List serves GET /hrm/designations: one page, filterable by status.
func (h *DesignationHandler) List(c *fiber.Ctx) error {
	var query struct {
		dto.PaginationQuery
		Status string `query:"status" validate:"omitempty,oneof=active inactive"`
	}
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	params, err := listParams(query.PaginationQuery, dto.DesignationSortFields)
	if err != nil {
		return err
	}

	rows, total, err := h.designations.List(c.UserContext(), middleware.BusinessID(c), service.DesignationQuery{
		ListQuery: toListQuery(params),
		Status:    query.Status,
	})
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, dto.NewListResponse(
		mapper.ToDesignationResponseList(rows), pagination.NewMeta(params, total),
	))
}

func (h *DesignationHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	row, err := h.designations.Get(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToDesignationResponse(*row))
}

func (h *DesignationHandler) Create(c *fiber.Ctx) error {
	var req dto.DesignationRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	var row entity.Designation
	mapper.ApplyDesignationRequest(&row, req)

	created, err := h.designations.Create(c.UserContext(), middleware.BusinessID(c), &row)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusCreated, mapper.ToDesignationResponse(*created))
}

/*
Update serves PUT /hrm/designations/{id}: a full replacement, so an omitted
description clears it rather than being left alone.
*/
func (h *DesignationHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	var req dto.DesignationRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	row, err := h.designations.Update(
		c.UserContext(), middleware.BusinessID(c), id,
		func(d *entity.Designation) { mapper.ApplyDesignationRequest(d, req) },
	)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToDesignationResponse(*row))
}

// Delete serves DELETE /hrm/designations/{id} → 204. Refused with 409 while
// employees still hold the title.
func (h *DesignationHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	if err := h.designations.Delete(c.UserContext(), middleware.BusinessID(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

/*
ShiftHandler serves the working-pattern endpoints.
*/
type ShiftHandler struct {
	shifts service.ShiftService
}

func NewShiftHandler(shifts service.ShiftService) *ShiftHandler {
	return &ShiftHandler{shifts: shifts}
}

func (h *ShiftHandler) List(c *fiber.Ctx) error {
	var query struct {
		dto.PaginationQuery
		Status string `query:"status" validate:"omitempty,oneof=active inactive"`
	}
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	params, err := listParams(query.PaginationQuery, dto.ShiftSortFields)
	if err != nil {
		return err
	}

	rows, total, err := h.shifts.List(c.UserContext(), middleware.BusinessID(c), service.ShiftQuery{
		ListQuery: toListQuery(params),
		Status:    query.Status,
	})
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, dto.NewListResponse(
		mapper.ToShiftResponseList(rows), pagination.NewMeta(params, total),
	))
}

func (h *ShiftHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	row, err := h.shifts.Get(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToShiftResponse(*row))
}

func (h *ShiftHandler) Create(c *fiber.Ctx) error {
	var req dto.ShiftRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	var row entity.Shift
	mapper.ApplyShiftRequest(&row, req)

	created, err := h.shifts.Create(c.UserContext(), middleware.BusinessID(c), &row)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusCreated, mapper.ToShiftResponse(*created))
}

func (h *ShiftHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	var req dto.ShiftRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	row, err := h.shifts.Update(
		c.UserContext(), middleware.BusinessID(c), id,
		func(s *entity.Shift) { mapper.ApplyShiftRequest(s, req) },
	)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToShiftResponse(*row))
}

func (h *ShiftHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	if err := h.shifts.Delete(c.UserContext(), middleware.BusinessID(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
