package mapper

import (
	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
)

// ToAttendanceResponse maps one day of the register onto the wire. The employee
// name and code are present when the row was preloaded, which the list and
// detail reads both do.
func ToAttendanceResponse(a entity.Attendance) dto.AttendanceResponse {
	res := dto.AttendanceResponse{
		ID:         a.ID.String(),
		EmployeeID: a.EmployeeID.String(),
		ShiftID:    uuidPtrToStringPtr(a.ShiftID),

		WorkDate:   isoDate(a.WorkDate),
		ClockInAt:  epochMillisPtr(a.ClockInAt),
		ClockOutAt: epochMillisPtr(a.ClockOutAt),

		Status: a.Status,
		Source: a.Source,

		WorkedMinutes:     a.WorkedMinutes,
		LateMinutes:       a.LateMinutes,
		EarlyLeaveMinutes: a.EarlyLeaveMinutes,
		OvertimeMinutes:   a.OvertimeMinutes,

		Note:      a.Note,
		CreatedAt: epochMillis(a.CreatedAt),
		UpdatedAt: epochMillis(a.UpdatedAt),
	}

	if a.Employee != nil {
		res.EmployeeName = stringPtr(a.Employee.FullName)
		res.EmployeeCode = stringPtr(a.Employee.EmployeeCode)
	}
	return res
}

func ToAttendanceResponseList(rows []entity.Attendance) []dto.AttendanceResponse {
	out := make([]dto.AttendanceResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToAttendanceResponse(row))
	}
	return out
}

/*
ToAttendanceSummaryRows maps the aggregate onto the wire, labelling each row
with the employee it belongs to.

The names come from a map the caller already loaded rather than from a join: the
aggregate groups by employee_id, and adding every displayed column to that GROUP
BY would cost more and gain nothing. A row whose employee is missing from the
map still goes out with its figures — an unnamed row is a gap in the label, not
a reason to drop somebody's attendance from a report.
*/
func ToAttendanceSummaryRows(
	rows []repository.AttendanceSummary, employees map[uuid.UUID]entity.Employee,
) []dto.AttendanceSummaryRow {
	out := make([]dto.AttendanceSummaryRow, 0, len(rows))
	for _, row := range rows {
		entry := dto.AttendanceSummaryRow{
			EmployeeID: row.EmployeeID.String(),

			PresentDays: row.PresentDays,
			LateDays:    row.LateDays,
			HalfDays:    row.HalfDays,
			AbsentDays:  row.AbsentDays,
			LeaveDays:   row.LeaveDays,

			WorkedMinutes:   row.WorkedMinutes,
			OvertimeMinutes: row.OvertimeMinutes,
			LateMinutes:     row.LateMinutes,
		}
		if employee, ok := employees[row.EmployeeID]; ok {
			entry.EmployeeName = stringPtr(employee.FullName)
			entry.EmployeeCode = stringPtr(employee.EmployeeCode)
		}
		out = append(out, entry)
	}
	return out
}

// ToManualAttendanceInput builds the service input a manual entry describes.
func ToManualAttendanceInput(
	req dto.AttendanceRecordRequest, recordedBy *uuid.UUID,
) (service.ManualAttendanceInput, error) {
	employeeID, err := uuid.Parse(req.EmployeeID)
	if err != nil {
		return service.ManualAttendanceInput{}, err
	}

	return service.ManualAttendanceInput{
		EmployeeID: employeeID,
		WorkDate:   parseISODateValue(req.WorkDate),
		ClockIn:    millisToTimePtr(req.ClockInAt),
		ClockOut:   millisToTimePtr(req.ClockOutAt),
		Status:     req.Status,
		Note:       req.Note,
		RecordedBy: recordedBy,
	}, nil
}
