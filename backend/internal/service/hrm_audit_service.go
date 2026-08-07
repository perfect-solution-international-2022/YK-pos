package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
)

/*
Audit actions written by the HRM module. Named "<module>.<resource>.<verb>" so
the resource_type prefix filter ("hrm") returns the module's whole trail without
callers having to enumerate its resources.
*/
const (
	AuditResourceHRM = "hrm"

	AuditActionEmployeeCreate = "hrm.employee.create"
	AuditActionEmployeeUpdate = "hrm.employee.update"
	AuditActionEmployeeDelete = "hrm.employee.delete"
	AuditActionDocumentUpload = "hrm.employee.document_upload"
	AuditActionDocumentDelete = "hrm.employee.document_delete"

	AuditActionAttendanceClockIn  = "hrm.attendance.clock_in"
	AuditActionAttendanceClockOut = "hrm.attendance.clock_out"
	AuditActionAttendanceRecord   = "hrm.attendance.record"

	AuditActionLeaveRequest = "hrm.leave.request"
	AuditActionLeaveApprove = "hrm.leave.approve"
	AuditActionLeaveReject  = "hrm.leave.reject"

	AuditActionPayrollGenerate = "hrm.payroll.generate"
	AuditActionPayrollFinalize = "hrm.payroll.finalize"
	AuditActionPayrollPaid     = "hrm.payroll.paid"
	AuditActionPayslipAdjust   = "hrm.payroll.payslip_adjust"

	AuditActionSettingsUpdate = "hrm.settings.update"
)

/*
AuditQueryService reads the audit trail back.

Deliberately separate from AuditService, which only writes: the write side runs
off the request path through a buffered worker and is mocked in service tests,
and giving it query methods it never calls would drag those tests along for the
ride.
*/
type AuditQueryService interface {
	Search(ctx context.Context, businessID uuid.UUID, query AuditQuery) ([]entity.AuditLog, int64, error)
}

type auditQueryService struct {
	logs repository.AuditLogQueryRepository
}

func NewAuditQueryService(logs repository.AuditLogQueryRepository) AuditQueryService {
	return &auditQueryService{logs: logs}
}

func (s *auditQueryService) Search(
	ctx context.Context, businessID uuid.UUID, query AuditQuery,
) ([]entity.AuditLog, int64, error) {
	rows, total, err := s.logs.Search(ctx, businessID, query.toParams())
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "failed to load audit logs", err)
	}
	return rows, total, nil
}
