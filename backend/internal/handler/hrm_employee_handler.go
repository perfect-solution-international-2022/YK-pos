package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/mapper"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/middleware"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/pagination"
)

/*
EmployeeHandler serves the staff endpoints.

The audit worker is a direct dependency because HR writes are exactly the ones
an auditor asks about later — who changed a salary, who removed a record — and
the request-scoped detail those entries carry (IP, user agent, request id) only
exists at this layer.
*/
type EmployeeHandler struct {
	employees service.EmployeeService
	audit     service.AuditService
}

func NewEmployeeHandler(employees service.EmployeeService, audit service.AuditService) *EmployeeHandler {
	return &EmployeeHandler{employees: employees, audit: audit}
}

/*
List serves GET /hrm/employees: one page, filterable and searchable.

Paginated, unlike GET /products. The catalogue is cached wholesale by the till
for offline selling; a staff list is a back-office screen with no offline
requirement, and a shop's headcount only grows.

`search` covers name, employee code, NIC, phone and email — which is also what
the spec's "search employee" asks for, so there is no second endpoint doing the
same query under a different path.
*/
func (h *EmployeeHandler) List(c *fiber.Ctx) error {
	var query dto.EmployeeListQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	params, err := listParams(query.PaginationQuery, dto.EmployeeSortFields)
	if err != nil {
		return err
	}

	designationID, err := optionalID(query.DesignationID, "designation_id")
	if err != nil {
		return err
	}
	shiftID, err := optionalID(query.ShiftID, "shift_id")
	if err != nil {
		return err
	}
	branchID, err := optionalID(query.BranchID, "branch_id")
	if err != nil {
		return err
	}

	rows, total, err := h.employees.List(c.UserContext(), middleware.BusinessID(c), service.EmployeeQuery{
		ListQuery:      toListQuery(params),
		DesignationID:  designationID,
		ShiftID:        shiftID,
		BranchID:       branchID,
		EmploymentType: query.EmploymentType,
		Status:         query.Status,
	})
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, dto.NewListResponse(
		mapper.ToEmployeeResponseList(rows), pagination.NewMeta(params, total),
	))
}

// Get serves GET /hrm/employees/{id}, with documents attached.
func (h *EmployeeHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	row, err := h.employees.Get(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToEmployeeResponse(*row))
}

/*
Me serves GET /hrm/employees/me: the HR record behind the authenticated user.

Gated on hrm.view like the rest, but scoped to the caller — it is what lets the
till show "you clocked in at 08:04" without giving a cashier the whole staff
list.
*/
func (h *EmployeeHandler) Me(c *fiber.Ctx) error {
	row, err := h.employees.GetByUserID(c.UserContext(), middleware.BusinessID(c), middleware.UserID(c))
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToEmployeeResponse(*row))
}

/*
Create serves POST /hrm/employees → 201.

The optional `login` block provisions a user account in the same transaction, so
a hire that fails partway cannot leave behind an account with a password
somebody has already been given.
*/
func (h *EmployeeHandler) Create(c *fiber.Ctx) error {
	var req dto.EmployeeCreateRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	login, err := mapper.ToEmployeeLogin(req.Login)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidationError, "invalid role id", err)
	}

	row := mapper.ToEmployeeEntity(req)
	created, err := h.employees.Create(c.UserContext(), middleware.BusinessID(c), &row, login)
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionEmployeeCreate, "hrm.employee", &created.ID, map[string]any{
		"employee_code": created.EmployeeCode,
		"full_name":     created.FullName,
		"has_login":     created.UserID != nil,
	})

	return ok(c, fiber.StatusCreated, mapper.ToEmployeeResponse(*created))
}

/*
Update serves PUT /hrm/employees/{id}: a full replacement of the editable
fields, returning the stored row.

The salary is audited by value because "who changed whose pay, and to what" is
the single question this trail exists to answer.
*/
func (h *EmployeeHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	var req dto.EmployeeUpdateRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	row, err := h.employees.Update(
		c.UserContext(), middleware.BusinessID(c), id,
		func(e *entity.Employee) { mapper.ApplyEmployeeUpdate(e, req) },
	)
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionEmployeeUpdate, "hrm.employee", &row.ID, map[string]any{
		"employee_code":      row.EmployeeCode,
		"full_name":          row.FullName,
		"status":             row.Status,
		"basic_salary_cents": row.BasicSalaryCents,
	})

	return ok(c, fiber.StatusOK, mapper.ToEmployeeResponse(*row))
}

/*
Delete serves DELETE /hrm/employees/{id} → 204.

Soft delete: payslips reference the employee, so the row has to survive for a
historical payslip to still name who it paid. A linked login is left in place —
losing an HR file must not silently lock somebody out of the till.
*/
func (h *EmployeeHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	if err := h.employees.Delete(c.UserContext(), middleware.BusinessID(c), id); err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionEmployeeDelete, "hrm.employee", &id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// ListDocuments serves GET /hrm/employees/{id}/documents, data URLs included.
func (h *EmployeeHandler) ListDocuments(c *fiber.Ctx) error {
	employeeID, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	docs, err := h.employees.ListDocuments(c.UserContext(), middleware.BusinessID(c), employeeID)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToEmployeeDocumentResponseList(docs))
}

/*
UploadDocument serves POST /hrm/employees/{id}/documents → 201.

The file arrives as a data URL in JSON rather than as multipart, the same way
product images do: one content type across the API, and the client reuses the
reader it already has for photos.
*/
func (h *EmployeeHandler) UploadDocument(c *fiber.Ctx) error {
	employeeID, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	var req dto.EmployeeDocumentRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	doc := mapper.ToEmployeeDocumentEntity(req, actorID(c))
	created, err := h.employees.AddDocument(c.UserContext(), middleware.BusinessID(c), employeeID, &doc)
	if err != nil {
		return err
	}

	// The data URL is deliberately not in the audit values: the trail should
	// record that a file was attached, not carry a copy of it.
	recordAudit(c, h.audit, service.AuditActionDocumentUpload, "hrm.employee", &employeeID, map[string]any{
		"document_id": created.ID.String(),
		"name":        created.Name,
		"size_bytes":  created.SizeBytes,
	})

	return ok(c, fiber.StatusCreated, mapper.ToEmployeeDocumentResponse(*created))
}

// DeleteDocument serves DELETE /hrm/employees/{id}/documents/{documentId} → 204.
func (h *EmployeeHandler) DeleteDocument(c *fiber.Ctx) error {
	employeeID, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}
	documentID, err := parseID(c.Params("documentId"), "documentId")
	if err != nil {
		return err
	}

	if err := h.employees.DeleteDocument(
		c.UserContext(), middleware.BusinessID(c), employeeID, documentID,
	); err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionDocumentDelete, "hrm.employee", &employeeID, map[string]any{
		"document_id": documentID.String(),
	})
	return c.SendStatus(fiber.StatusNoContent)
}
