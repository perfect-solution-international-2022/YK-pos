package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

//go:generate go run go.uber.org/mock/mockgen -source=audit_log_query_repository.go -destination=mocks/audit_log_query_repository_mock.go -package=mocks

/*
AuditLogSearchParams narrows the audit trail. Every field is optional.

Module matches the resource_type column — "hrm.employee", "hrm.payroll" — with
a prefix match, so filtering on "hrm" returns the whole module's trail without
the caller having to enumerate its resources.
*/
type AuditLogSearchParams struct {
	ListParams

	UserID *uuid.UUID
	Action string
	Module string
	Status string
	Range  DateRange
}

/*
AuditLogQueryRepository is the read side of the audit trail.

Separate from AuditLogRepository rather than bolted onto it, because the two
have genuinely different shapes: the write side is append-only and is called
from the hot path through a buffered worker, while this side is an
administrator's filtered search. Keeping them apart means the write path's
interface — the one mocked in every service test — does not grow query methods
it never calls.
*/
type AuditLogQueryRepository interface {
	Search(ctx context.Context, businessID uuid.UUID, params AuditLogSearchParams) ([]entity.AuditLog, int64, error)
}

type auditLogQueryRepository struct {
	db *gorm.DB
}

func NewAuditLogQueryRepository(db *gorm.DB) AuditLogQueryRepository {
	return &auditLogQueryRepository{db: db}
}

func (r *auditLogQueryRepository) Search(
	ctx context.Context, businessID uuid.UUID, params AuditLogSearchParams,
) ([]entity.AuditLog, int64, error) {
	scoped := func() *gorm.DB {
		q := r.db.WithContext(ctx).
			Model(&entity.AuditLog{}).
			Where("business_id = ?", businessID)

		if params.UserID != nil {
			q = q.Where("user_id = ?", *params.UserID)
		}
		if params.Action != "" {
			q = q.Where("action = ?", params.Action)
		}
		if params.Module != "" {
			// Already LIKE-escaped by pagination.Parse when it came from a
			// search box; a literal module name has nothing to escape.
			q = q.Where("resource_type LIKE ?", params.Module+"%")
		}
		if params.Status != "" {
			q = q.Where("status = ?", params.Status)
		}
		q = applyDateRange(q, "created_at", params.Range)

		if like := likePattern(params.Search); like != "" {
			q = q.Where("(action ILIKE ? OR resource_type ILIKE ? OR ip_address ILIKE ?)", like, like, like)
		}
		return q
	}

	var total int64
	if err := scoped().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entity.AuditLog{}, 0, nil
	}

	var rows []entity.AuditLog
	if err := paginate(scoped(), params.ListParams).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
