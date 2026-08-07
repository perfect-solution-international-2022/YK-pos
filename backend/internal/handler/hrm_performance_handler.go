package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/mapper"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/middleware"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/pagination"
)

/*
PerformanceHandler serves the monthly-evaluation endpoints.
*/
type PerformanceHandler struct {
	reviews service.PerformanceService
}

func NewPerformanceHandler(reviews service.PerformanceService) *PerformanceHandler {
	return &PerformanceHandler{reviews: reviews}
}

func (h *PerformanceHandler) List(c *fiber.Ctx) error {
	var query dto.PerformanceListQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	params, err := listParams(query.PaginationQuery, dto.PerformanceSortFields)
	if err != nil {
		return err
	}
	employeeID, err := optionalID(query.EmployeeID, "employee_id")
	if err != nil {
		return err
	}

	rows, total, err := h.reviews.List(c.UserContext(), middleware.BusinessID(c), service.PerformanceQuery{
		ListQuery:   toListQuery(params),
		EmployeeID:  employeeID,
		PeriodYear:  optionalInt(query.Year),
		PeriodMonth: optionalInt(query.Month),
	})
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, dto.NewListResponse(
		mapper.ToPerformanceReviewResponseList(rows), pagination.NewMeta(params, total),
	))
}

func (h *PerformanceHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	row, err := h.reviews.Get(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToPerformanceReviewResponse(*row))
}

/*
Create serves POST /hrm/performance → 201.

One evaluation per employee per month: a second for a period already reviewed is
a 409 pointing at the existing one, because an average rating that depended on
how many times a manager pressed save would mean nothing.
*/
func (h *PerformanceHandler) Create(c *fiber.Ctx) error {
	var req dto.PerformanceReviewCreateRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	row := mapper.ToPerformanceReviewEntity(req, actorID(c))
	created, err := h.reviews.Create(c.UserContext(), middleware.BusinessID(c), &row)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusCreated, mapper.ToPerformanceReviewResponse(*created))
}

// Update serves PUT /hrm/performance/{id}: the scores and notes only. The
// employee and the period are fixed — moving a review to another month is
// filing a different review.
func (h *PerformanceHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	var req dto.PerformanceReviewUpdateRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	reviewerID := actorID(c)
	row, err := h.reviews.Update(
		c.UserContext(), middleware.BusinessID(c), id,
		func(r *entity.PerformanceReview) { mapper.ApplyPerformanceReviewUpdate(r, req, reviewerID) },
	)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToPerformanceReviewResponse(*row))
}

func (h *PerformanceHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	if err := h.reviews.Delete(c.UserContext(), middleware.BusinessID(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

/*
AnnouncementHandler serves the staff noticeboard.
*/
type AnnouncementHandler struct {
	announcements service.AnnouncementService
}

func NewAnnouncementHandler(announcements service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{announcements: announcements}
}

/*
List serves GET /hrm/announcements.

`current=true` narrows to notices in force today — published, on or before
today, and not yet expired — which is the staff-facing view, as opposed to the
full board an HR officer edits.
*/
func (h *AnnouncementHandler) List(c *fiber.Ctx) error {
	var query dto.AnnouncementListQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	params, err := listParams(query.PaginationQuery, dto.AnnouncementSortFields)
	if err != nil {
		return err
	}

	serviceQuery := service.AnnouncementQuery{
		ListQuery: toListQuery(params),
		Status:    query.Status,
	}
	if query.Current {
		today := time.Now().Truncate(24 * time.Hour)
		serviceQuery.PublishedOn = &today
	}

	rows, total, err := h.announcements.List(c.UserContext(), middleware.BusinessID(c), serviceQuery)
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, dto.NewListResponse(
		mapper.ToAnnouncementResponseList(rows), pagination.NewMeta(params, total),
	))
}

func (h *AnnouncementHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	row, err := h.announcements.Get(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToAnnouncementResponse(*row))
}

func (h *AnnouncementHandler) Create(c *fiber.Ctx) error {
	var req dto.AnnouncementRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	row := entity.Announcement{CreatedBy: actorID(c)}
	mapper.ApplyAnnouncementRequest(&row, req)

	created, err := h.announcements.Create(c.UserContext(), middleware.BusinessID(c), &row)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusCreated, mapper.ToAnnouncementResponse(*created))
}

func (h *AnnouncementHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	var req dto.AnnouncementRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	row, err := h.announcements.Update(
		c.UserContext(), middleware.BusinessID(c), id,
		func(a *entity.Announcement) { mapper.ApplyAnnouncementRequest(a, req) },
	)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToAnnouncementResponse(*row))
}

func (h *AnnouncementHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	if err := h.announcements.Delete(c.UserContext(), middleware.BusinessID(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
