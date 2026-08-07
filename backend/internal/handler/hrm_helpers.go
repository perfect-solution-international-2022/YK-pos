package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/middleware"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
	appvalidator "github.com/SandaruwanWeerawardhana/pos-backend/internal/validator"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/pagination"
)

/*
Plumbing shared by the HRM handlers. Each piece exists because otherwise eleven
list endpoints would repeat it, and one of them would eventually repeat it
wrongly — most dangerously the sort whitelist, which is the only thing standing
between a query parameter and an unescaped ORDER BY.
*/

// parseQuery parses and validates a query-string struct.
func parseQuery(c *fiber.Ctx, dst any) error {
	if err := c.QueryParser(dst); err != nil {
		return apperror.Wrap(apperror.CodeBadRequest, "invalid query parameters", err)
	}
	return appvalidator.Struct(dst)
}

/*
listParams clamps paging and validates the sort field against the endpoint's
whitelist. The whitelist check is load-bearing: Sort is interpolated into ORDER
BY, which GORM does not escape.
*/
func listParams(query dto.PaginationQuery, sortWhitelist []string) (pagination.Params, error) {
	return pagination.Parse(
		query.Page, query.PerPage, query.Sort, query.Order, query.Search, sortWhitelist,
	)
}

// toListQuery converts validated pagination into the service's input shape.
func toListQuery(p pagination.Params) service.ListQuery {
	return service.ListQuery{
		Limit:  p.PerPage,
		Offset: p.Offset(),
		Sort:   p.Sort,
		Order:  p.Order,
		Search: p.Search,
	}
}

/*
parseDateWindow turns the yyyy-mm-dd bounds into a window, rejecting an inverted
range rather than silently returning nothing — a screen showing an empty month
for a range it thinks is valid reads as missing data, not as a bad request.

The strings are already validated as dates by the DTO tags, so a parse failure
here means the field was absent.
*/
func parseDateWindow(from, to string) (service.DateWindow, error) {
	window := service.DateWindow{}

	if from != "" {
		parsed, err := time.Parse(time.DateOnly, from)
		if err != nil {
			return window, invalidDate("from")
		}
		window.From = parsed
	}
	if to != "" {
		parsed, err := time.Parse(time.DateOnly, to)
		if err != nil {
			return window, invalidDate("to")
		}
		window.To = parsed
	}

	if !window.From.IsZero() && !window.To.IsZero() && window.To.Before(window.From) {
		return service.DateWindow{}, apperror.WithFields(
			apperror.CodeValidationError,
			"validation failed",
			[]apperror.FieldError{{
				Field:   "to",
				Rule:    "gtefield",
				Message: "to must not be earlier than from",
			}},
		)
	}

	return window, nil
}

/*
monthWindow bounds a calendar month. The end is the last day of the month rather
than the first of the next, because the HRM date filters are inclusive on both
ends — they run over DATE columns, not instants.
*/
func monthWindow(year, month int) service.DateWindow {
	from := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return service.DateWindow{From: from, To: from.AddDate(0, 1, -1)}
}

func invalidDate(field string) error {
	return apperror.WithFields(
		apperror.CodeValidationError,
		"validation failed",
		[]apperror.FieldError{{
			Field:   field,
			Rule:    "datetime",
			Message: field + " must be a date in yyyy-mm-dd format",
		}},
	)
}

/*
optionalID parses a filter that may be absent. Empty means "not filtered", which
is why it returns a nil pointer rather than uuid.Nil — a zero UUID is a value the
query would then match on and find nothing.
*/
func optionalID(value, field string) (*uuid.UUID, error) {
	if value == "" {
		return nil, nil
	}
	id, err := parseID(value, field)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// optionalInt distinguishes "not filtered" from a real zero for the year and
// month filters, where 0 is not a valid value but is the zero value.
func optionalInt(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

// actorID is the authenticated user, as the nullable column value the HRM
// tables store for "who did this".
func actorID(c *fiber.Ctx) *uuid.UUID {
	userID := middleware.UserID(c)
	if userID == uuid.Nil {
		return nil
	}
	return &userID
}

/*
recordAudit writes an HRM audit entry.

Called from the handler rather than the service because that is where the
request-scoped detail lives — the IP address, the user agent, the request id —
and because the worker behind it is asynchronous: the write happens off the
request path and a full queue drops the entry rather than failing the action it
describes. The error is deliberately discarded for that reason.
*/
func recordAudit(
	c *fiber.Ctx, audit service.AuditService, action, resourceType string,
	resourceID *uuid.UUID, values any,
) {
	businessID := middleware.BusinessID(c)
	meta := requestMeta(c)

	_ = audit.Log(c.UserContext(), service.AuditEntry{
		BusinessID:   &businessID,
		UserID:       actorID(c),
		Action:       action,
		Status:       entity.AuditStatusSuccess,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		NewValues:    values,
		IPAddress:    meta.IPAddress,
		UserAgent:    meta.UserAgent,
		RequestID:    meta.RequestID,
	})
}
