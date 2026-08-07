package mapper

import (
	"strings"

	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
)

// ToLeaveTypeResponse maps an entitlement category onto the wire.
func ToLeaveTypeResponse(t entity.LeaveType) dto.LeaveTypeResponse {
	return dto.LeaveTypeResponse{
		ID:              t.ID.String(),
		Name:            t.Name,
		Code:            t.Code,
		AnnualQuotaDays: t.AnnualQuotaDays,
		IsPaid:          t.IsPaid,
		IsSystem:        t.IsSystem,
		Status:          t.Status,
		CreatedAt:       epochMillis(t.CreatedAt),
		UpdatedAt:       epochMillis(t.UpdatedAt),
	}
}

func ToLeaveTypeResponseList(rows []entity.LeaveType) []dto.LeaveTypeResponse {
	out := make([]dto.LeaveTypeResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToLeaveTypeResponse(row))
	}
	return out
}

// ApplyLeaveTypeRequest writes a request onto a leave type. The code is
// uppercased to match the case-insensitive unique index it is checked against.
func ApplyLeaveTypeRequest(t *entity.LeaveType, req dto.LeaveTypeRequest) {
	t.Name = req.Name
	t.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	t.AnnualQuotaDays = req.AnnualQuotaDays
	t.IsPaid = req.IsPaid
	if req.Status != "" {
		t.Status = req.Status
	}
}

// ToLeaveRequestResponse maps one application and its decision onto the wire.
func ToLeaveRequestResponse(r entity.LeaveRequest) dto.LeaveRequestResponse {
	res := dto.LeaveRequestResponse{
		ID:          r.ID.String(),
		EmployeeID:  r.EmployeeID.String(),
		LeaveTypeID: r.LeaveTypeID.String(),

		StartDate: isoDate(r.StartDate),
		EndDate:   isoDate(r.EndDate),
		Days:      r.Days,
		HalfDay:   r.HalfDay,
		Reason:    r.Reason,

		Status:     r.Status,
		ReviewedBy: uuidPtrToStringPtr(r.ReviewedBy),
		ReviewedAt: epochMillisPtr(r.ReviewedAt),
		ReviewNote: r.ReviewNote,

		CreatedAt: epochMillis(r.CreatedAt),
		UpdatedAt: epochMillis(r.UpdatedAt),
	}

	if r.Employee != nil {
		res.EmployeeName = stringPtr(r.Employee.FullName)
		res.EmployeeCode = stringPtr(r.Employee.EmployeeCode)
	}
	if r.LeaveType != nil {
		res.LeaveTypeName = stringPtr(r.LeaveType.Name)
		isPaid := r.LeaveType.IsPaid
		res.IsPaid = &isPaid
	}
	return res
}

func ToLeaveRequestResponseList(rows []entity.LeaveRequest) []dto.LeaveRequestResponse {
	out := make([]dto.LeaveRequestResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToLeaveRequestResponse(row))
	}
	return out
}

// ToLeaveRequestInput builds the service input an application describes.
func ToLeaveRequestInput(req dto.LeaveApplyRequest, requestedBy *uuid.UUID) (service.LeaveRequestInput, error) {
	employeeID, err := uuid.Parse(req.EmployeeID)
	if err != nil {
		return service.LeaveRequestInput{}, err
	}
	leaveTypeID, err := uuid.Parse(req.LeaveTypeID)
	if err != nil {
		return service.LeaveRequestInput{}, err
	}

	return service.LeaveRequestInput{
		EmployeeID:  employeeID,
		LeaveTypeID: leaveTypeID,
		StartDate:   parseISODateValue(req.StartDate),
		EndDate:     parseISODateValue(req.EndDate),
		HalfDay:     req.HalfDay,
		Reason:      req.Reason,
		RequestedBy: requestedBy,
	}, nil
}

// ToLeaveBalanceResponseList maps the computed balances onto the wire.
func ToLeaveBalanceResponseList(balances []service.LeaveBalance) []dto.LeaveBalanceResponse {
	out := make([]dto.LeaveBalanceResponse, 0, len(balances))
	for _, b := range balances {
		out = append(out, dto.LeaveBalanceResponse{
			LeaveTypeID:   b.LeaveType.ID.String(),
			LeaveTypeName: b.LeaveType.Name,
			Code:          b.LeaveType.Code,
			IsPaid:        b.LeaveType.IsPaid,
			Entitled:      b.Entitled,
			Taken:         b.Taken,
			Pending:       b.Pending,
			Remaining:     b.Remaining,
			Unlimited:     b.Unlimited,
		})
	}
	return out
}
