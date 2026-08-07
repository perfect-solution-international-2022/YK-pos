package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/mapper"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/middleware"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/pagination"
)

/*
PayrollHandler serves the payroll endpoints.

Every write here is audited: a payroll run is the most consequential thing this
module does, and "who generated this month, who signed it off, who adjusted
whose payslip" is the question the trail exists to answer.
*/
type PayrollHandler struct {
	payroll service.PayrollService
	audit   service.AuditService
}

func NewPayrollHandler(payroll service.PayrollService, audit service.AuditService) *PayrollHandler {
	return &PayrollHandler{payroll: payroll, audit: audit}
}

// ListRuns serves GET /hrm/payroll/runs: one page of months, newest first.
func (h *PayrollHandler) ListRuns(c *fiber.Ctx) error {
	var query dto.PaginationQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	params, err := listParams(query, dto.PayrollRunSortFields)
	if err != nil {
		return err
	}

	rows, total, err := h.payroll.ListRuns(c.UserContext(), middleware.BusinessID(c), toListQuery(params))
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, dto.NewListResponse(
		mapper.ToPayrollRunResponseList(rows), pagination.NewMeta(params, total),
	))
}

// GetRun serves GET /hrm/payroll/runs/{id}, payslips included.
func (h *PayrollHandler) GetRun(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	row, err := h.payroll.GetRun(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToPayrollRunResponse(*row))
}

/*
Generate serves POST /hrm/payroll/runs → 201.

Re-posting the same period recomputes a draft in place and replaces its
payslips, because attendance gets corrected after the fact and a manager needs
to re-run the month rather than patch twenty slips by hand. A finalised period
is refused with 409.
*/
func (h *PayrollHandler) Generate(c *fiber.Ctx) error {
	var req dto.GeneratePayrollRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	employeeIDs, err := parseIDs(req.EmployeeIDs, "employee_ids")
	if err != nil {
		return err
	}

	run, err := h.payroll.Generate(c.UserContext(), middleware.BusinessID(c), service.GeneratePayrollInput{
		Year:        req.Year,
		Month:       req.Month,
		EmployeeIDs: employeeIDs,
		Notes:       req.Notes,
		GeneratedBy: actorID(c),
	})
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionPayrollGenerate, "hrm.payroll", &run.ID, map[string]any{
		"period_year":  run.PeriodYear,
		"period_month": run.PeriodMonth,
		"payslips":     len(run.Payslips),
	})

	return ok(c, fiber.StatusCreated, mapper.ToPayrollRunResponse(*run))
}

/*
Finalize serves POST /hrm/payroll/runs/{id}/finalize.

One-way: reopening would let a figure change after staff had been shown it,
which is the thing the status exists to prevent.
*/
func (h *PayrollHandler) Finalize(c *fiber.Ctx) error {
	return h.transition(c, service.AuditActionPayrollFinalize, h.payroll.Finalize)
}

// MarkPaid serves POST /hrm/payroll/runs/{id}/pay, recording that a finalised
// run was actually disbursed.
func (h *PayrollHandler) MarkPaid(c *fiber.Ctx) error {
	return h.transition(c, service.AuditActionPayrollPaid, h.payroll.MarkPaid)
}

// DeleteRun serves DELETE /hrm/payroll/runs/{id} → 204. Drafts only: a finalised
// run is the record of what was paid.
func (h *PayrollHandler) DeleteRun(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	if err := h.payroll.DeleteRun(c.UserContext(), middleware.BusinessID(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

/*
ListPayslips serves GET /hrm/payroll/payslips: payslip history across runs,
filterable by employee, period and status.
*/
func (h *PayrollHandler) ListPayslips(c *fiber.Ctx) error {
	var query dto.PayslipListQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	params, err := listParams(query.PaginationQuery, dto.PayslipSortFields)
	if err != nil {
		return err
	}
	employeeID, err := optionalID(query.EmployeeID, "employee_id")
	if err != nil {
		return err
	}

	rows, total, err := h.payroll.ListPayslips(c.UserContext(), middleware.BusinessID(c), service.PayslipQuery{
		ListQuery:   toListQuery(params),
		EmployeeID:  employeeID,
		PeriodYear:  optionalInt(query.Year),
		PeriodMonth: optionalInt(query.Month),
		Status:      query.Status,
	})
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, dto.NewListResponse(
		mapper.ToPayslipResponseList(rows), pagination.NewMeta(params, total),
	))
}

/*
GetPayslip serves GET /hrm/payroll/payslips/{id}: the payslip a client renders
or prints, with its ad-hoc lines so the breakdown adds up on the page.
*/
func (h *PayrollHandler) GetPayslip(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	row, err := h.payroll.GetPayslip(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToPayslipResponse(*row))
}

/*
AdjustPayslip serves PUT /hrm/payroll/payslips/{id}: replacing a draft payslip's
ad-hoc earnings and deductions and re-totalling it.

Replacement rather than append, so removing a bonus is expressible. The computed
components are not editable — they come from attendance and the run's rules
snapshot, and a form overwriting them would make the payslip unexplainable.
*/
func (h *PayrollHandler) AdjustPayslip(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	var req dto.PayslipAdjustRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	row, err := h.payroll.AdjustPayslip(
		c.UserContext(), middleware.BusinessID(c), id, mapper.ToPayslipAdjustment(req),
	)
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionPayslipAdjust, "hrm.payroll", &row.ID, map[string]any{
		"employee_id":           row.EmployeeID.String(),
		"bonus_cents":           row.BonusCents,
		"other_deduction_cents": row.OtherDeductionCents,
		"net_cents":             row.NetCents,
	})

	return ok(c, fiber.StatusOK, mapper.ToPayslipResponse(*row))
}

// transition is the shared body of finalize and pay: same id, same audit shape,
// different action and service call.
func (h *PayrollHandler) transition(
	c *fiber.Ctx,
	action string,
	apply func(ctx context.Context, businessID, id uuid.UUID) (*entity.PayrollRun, error),
) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	run, err := apply(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, action, "hrm.payroll", &run.ID, map[string]any{
		"period_year":  run.PeriodYear,
		"period_month": run.PeriodMonth,
		"status":       run.Status,
	})

	return ok(c, fiber.StatusOK, mapper.ToPayrollRunResponse(*run))
}
