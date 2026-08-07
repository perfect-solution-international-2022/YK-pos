package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/mapper"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/middleware"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/pagination"
)

/*
LeaveHandler serves the leave endpoints: entitlement types, applications,
decisions and balances.
*/
type LeaveHandler struct {
	leave service.LeaveService
	audit service.AuditService
}

func NewLeaveHandler(leave service.LeaveService, audit service.AuditService) *LeaveHandler {
	return &LeaveHandler{leave: leave, audit: audit}
}

/*
ListTypes serves GET /hrm/leave-types as a bare array.

Unpaginated, like GET /products: a shop has a handful of leave types, every
leave form needs all of them at once to populate its dropdown, and a page of
them would be a page of nothing.
*/
func (h *LeaveHandler) ListTypes(c *fiber.Ctx) error {
	var query struct {
		IncludeInactive bool `query:"include_inactive"`
	}
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	rows, err := h.leave.ListTypes(c.UserContext(), middleware.BusinessID(c), query.IncludeInactive)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToLeaveTypeResponseList(rows))
}

func (h *LeaveHandler) CreateType(c *fiber.Ctx) error {
	var req dto.LeaveTypeRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	var row entity.LeaveType
	mapper.ApplyLeaveTypeRequest(&row, req)

	created, err := h.leave.CreateType(c.UserContext(), middleware.BusinessID(c), &row)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusCreated, mapper.ToLeaveTypeResponse(*created))
}

// UpdateType serves PUT /hrm/leave-types/{id}. A system type is refused with
// 403: it is shared by every tenant, so a shop configures its own instead.
func (h *LeaveHandler) UpdateType(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	var req dto.LeaveTypeRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	row, err := h.leave.UpdateType(
		c.UserContext(), middleware.BusinessID(c), id,
		func(t *entity.LeaveType) { mapper.ApplyLeaveTypeRequest(t, req) },
	)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToLeaveTypeResponse(*row))
}

func (h *LeaveHandler) DeleteType(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	if err := h.leave.DeleteType(c.UserContext(), middleware.BusinessID(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

/*
List serves GET /hrm/leave-requests: one page of applications, filterable by
employee, type, status and start-date window. Filtering on status=pending is the
approvals queue.
*/
func (h *LeaveHandler) List(c *fiber.Ctx) error {
	var query dto.LeaveListQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	params, err := listParams(query.PaginationQuery, dto.LeaveRequestSortFields)
	if err != nil {
		return err
	}
	window, err := parseDateWindow(query.From, query.To)
	if err != nil {
		return err
	}
	employeeID, err := optionalID(query.EmployeeID, "employee_id")
	if err != nil {
		return err
	}
	leaveTypeID, err := optionalID(query.LeaveTypeID, "leave_type_id")
	if err != nil {
		return err
	}

	rows, total, err := h.leave.List(c.UserContext(), middleware.BusinessID(c), service.LeaveQuery{
		ListQuery:   toListQuery(params),
		EmployeeID:  employeeID,
		LeaveTypeID: leaveTypeID,
		Status:      query.Status,
		Window:      window,
	})
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, dto.NewListResponse(
		mapper.ToLeaveRequestResponseList(rows), pagination.NewMeta(params, total),
	))
}

func (h *LeaveHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	row, err := h.leave.Get(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToLeaveRequestResponse(*row))
}

// Request serves POST /hrm/leave-requests → 201, pending a decision.
func (h *LeaveHandler) Request(c *fiber.Ctx) error {
	var req dto.LeaveApplyRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	input, err := mapper.ToLeaveRequestInput(req, actorID(c))
	if err != nil {
		return apperror.Wrap(apperror.CodeValidationError, "invalid id in request", err)
	}

	row, err := h.leave.Request(c.UserContext(), middleware.BusinessID(c), input)
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionLeaveRequest, "hrm.leave", &row.ID, map[string]any{
		"employee_id": row.EmployeeID.String(),
		"start_date":  row.StartDate.Format(time.DateOnly),
		"end_date":    row.EndDate.Format(time.DateOnly),
		"days":        row.Days,
	})

	return ok(c, fiber.StatusCreated, mapper.ToLeaveRequestResponse(*row))
}

/*
Approve serves POST /hrm/leave-requests/{id}/approve.

The balance is rechecked at this point, not only when the request was filed:
several applications can sit pending at once, and approving each in turn is
exactly how a quota gets overspent without anybody deciding to.
*/
func (h *LeaveHandler) Approve(c *fiber.Ctx) error {
	return h.decide(c, service.AuditActionLeaveApprove, h.leave.Approve)
}

// Reject serves POST /hrm/leave-requests/{id}/reject.
func (h *LeaveHandler) Reject(c *fiber.Ctx) error {
	return h.decide(c, service.AuditActionLeaveReject, h.leave.Reject)
}

/*
Cancel serves POST /hrm/leave-requests/{id}/cancel: withdrawing an application
that has not been decided yet.

An approved request is not cancellable here — the days are already on the
register and may already sit inside a finalised payroll run, so unwinding one is
a correction a manager makes deliberately.
*/
func (h *LeaveHandler) Cancel(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	row, err := h.leave.Cancel(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToLeaveRequestResponse(*row))
}

/*
Balances serves GET /hrm/leave-balances: every active leave type for one
employee in one year.

Derived from the approved requests rather than stored, so the figure cannot
drift from the applications it is computed out of. The year defaults to the
current one.
*/
func (h *LeaveHandler) Balances(c *fiber.Ctx) error {
	var query dto.LeaveBalanceQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	employeeID, err := parseID(query.EmployeeID, "employee_id")
	if err != nil {
		return err
	}

	year := query.Year
	if year == 0 {
		year = time.Now().Year()
	}

	balances, err := h.leave.Balances(c.UserContext(), middleware.BusinessID(c), employeeID, year)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToLeaveBalanceResponseList(balances))
}

/*
decide is the shared body of approve and reject: both parse the same id, take
the same optional note, and record the same audit shape under a different
action.
*/
func (h *LeaveHandler) decide(
	c *fiber.Ctx,
	action string,
	apply func(ctx context.Context, businessID, id, reviewerID uuid.UUID, note *string) (*entity.LeaveRequest, error),
) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	var req dto.LeaveDecisionRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	row, err := apply(c.UserContext(), middleware.BusinessID(c), id, middleware.UserID(c), req.Note)
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, action, "hrm.leave", &row.ID, map[string]any{
		"employee_id": row.EmployeeID.String(),
		"status":      row.Status,
		"days":        row.Days,
	})

	return ok(c, fiber.StatusOK, mapper.ToLeaveRequestResponse(*row))
}
