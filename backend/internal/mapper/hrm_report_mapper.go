package mapper

import (
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
)

// ToEmployeeReportResponse maps the headcount picture onto the wire.
func ToEmployeeReportResponse(r service.EmployeeReport) dto.EmployeeReportResponse {
	res := dto.EmployeeReportResponse{
		TotalEmployees:     r.TotalEmployees,
		ActiveEmployees:    r.ActiveEmployees,
		MonthlySalaryCents: r.MonthlySalaryCost,
		ByDesignation:      make([]dto.DesignationHeadcountRow, 0, len(r.ByDesignation)),
		ByEmploymentType:   make([]dto.EmploymentTypeRow, 0, len(r.ByEmploymentType)),
	}

	for _, row := range r.ByDesignation {
		res.ByDesignation = append(res.ByDesignation, dto.DesignationHeadcountRow{
			DesignationID:   row.DesignationID.String(),
			DesignationName: row.DesignationName,
			Total:           row.Total,
			ActiveTotal:     row.ActiveTotal,
			SalaryCents:     row.SalaryCents,
		})
	}
	for _, row := range r.ByEmploymentType {
		res.ByEmploymentType = append(res.ByEmploymentType, dto.EmploymentTypeRow{
			EmploymentType: row.EmploymentType,
			Total:          row.Total,
		})
	}

	return res
}

// ToAttendanceReportResponse reuses the summary row shape, so a client renders
// the register identically whether it came from the summary endpoint or here.
func ToAttendanceReportResponse(r service.AttendanceReport) dto.AttendanceReportResponse {
	return dto.AttendanceReportResponse{
		From: isoDate(r.Range.From),
		To:   isoDate(r.Range.To),
		Rows: ToAttendanceSummaryRows(r.Rows, r.Employees),
	}
}

// ToLeaveReportResponse maps a window's leave usage onto the wire.
func ToLeaveReportResponse(r service.LeaveReport) dto.LeaveReportResponse {
	res := dto.LeaveReportResponse{
		From: isoDate(r.Range.From),
		To:   isoDate(r.Range.To),
		Rows: make([]dto.LeaveReportRow, 0, len(r.Rows)),
	}
	for _, row := range r.Rows {
		res.Rows = append(res.Rows, dto.LeaveReportRow{
			LeaveTypeID:   row.LeaveTypeID.String(),
			LeaveTypeName: row.LeaveTypeName,
			IsPaid:        row.IsPaid,
			Requests:      row.Requests,
			ApprovedDays:  row.ApprovedDays,
			PendingDays:   row.PendingDays,
			RejectedCount: row.RejectedCount,
		})
	}
	return res
}

/*
ToPayrollReportResponse maps a year of monthly totals onto the wire and sums
them.

The year total is computed here rather than left to the client so both sides
cannot disagree about the answer — twelve rows re-added in a browser is twelve
chances to round differently.
*/
func ToPayrollReportResponse(r service.PayrollReport) dto.PayrollReportResponse {
	res := dto.PayrollReportResponse{
		Year: r.Year,
		Rows: make([]dto.PayrollReportRow, 0, len(r.Rows)),
	}

	for _, row := range r.Rows {
		res.Rows = append(res.Rows, dto.PayrollReportRow{
			PeriodMonth:      row.PeriodMonth,
			Employees:        row.Employees,
			GrossCents:       row.GrossCents,
			DeductionCents:   row.DeductionCents,
			NetCents:         row.NetCents,
			OvertimeCents:    row.OvertimeCents,
			BonusCents:       row.BonusCents,
			EPFEmployerCents: row.EPFEmployer,
			ETFCents:         row.ETFCents,
		})

		res.Totals.GrossCents += row.GrossCents
		res.Totals.DeductionCents += row.DeductionCents
		res.Totals.NetCents += row.NetCents
		res.Totals.OvertimeCents += row.OvertimeCents
		res.Totals.BonusCents += row.BonusCents
		res.Totals.EPFEmployerCents += row.EPFEmployer
		res.Totals.ETFCents += row.ETFCents
	}

	return res
}

// ToHRMSettingsResponse maps the policy set onto the wire.
func ToHRMSettingsResponse(s service.HRMSettings) dto.HRMSettingsResponse {
	return dto.HRMSettingsResponse{
		Attendance: dto.AttendanceRulesDTO{
			FullDayMinutes:     s.Attendance.FullDayMinutes,
			HalfDayMinutes:     s.Attendance.HalfDayMinutes,
			OvertimeEnabled:    s.Attendance.OvertimeEnabled,
			MinOvertimeMinutes: s.Attendance.MinOvertimeMinutes,
			AllowFutureEntry:   s.Attendance.AllowFutureEntry,
		},
		Leave: dto.LeaveRulesDTO{
			MaxConsecutiveDays:   s.Leave.MaxConsecutiveDays,
			MinNoticeDays:        s.Leave.MinNoticeDays,
			AllowNegativeBalance: s.Leave.AllowNegativeBalance,
			ExcludeWeeklyOff:     s.Leave.ExcludeWeeklyOff,
		},
		Payroll: dto.PayrollRulesDTO{
			WorkingDaysPerMonth: s.Payroll.WorkingDaysPerMonth,
			WorkingHoursPerDay:  s.Payroll.WorkingHoursPerDay,
			OvertimeMultiplier:  s.Payroll.OvertimeMultiplier,
			EPFEmployeePercent:  s.Payroll.EPFEmployeePercent,
			EPFEmployerPercent:  s.Payroll.EPFEmployerPercent,
			ETFPercent:          s.Payroll.ETFPercent,
			TaxPercent:          s.Payroll.TaxPercent,
			DeductAbsentDays:    s.Payroll.DeductAbsentDays,
			DeductUnpaidLeave:   s.Payroll.DeductUnpaidLeave,
		},
	}
}

// ToHRMSettings is the inverse, for the settings screen's save.
func ToHRMSettings(req dto.HRMSettingsResponse) service.HRMSettings {
	return service.HRMSettings{
		Attendance: service.AttendanceRules{
			FullDayMinutes:     req.Attendance.FullDayMinutes,
			HalfDayMinutes:     req.Attendance.HalfDayMinutes,
			OvertimeEnabled:    req.Attendance.OvertimeEnabled,
			MinOvertimeMinutes: req.Attendance.MinOvertimeMinutes,
			AllowFutureEntry:   req.Attendance.AllowFutureEntry,
		},
		Leave: service.LeaveRules{
			MaxConsecutiveDays:   req.Leave.MaxConsecutiveDays,
			MinNoticeDays:        req.Leave.MinNoticeDays,
			AllowNegativeBalance: req.Leave.AllowNegativeBalance,
			ExcludeWeeklyOff:     req.Leave.ExcludeWeeklyOff,
		},
		Payroll: service.PayrollRules{
			WorkingDaysPerMonth: req.Payroll.WorkingDaysPerMonth,
			WorkingHoursPerDay:  req.Payroll.WorkingHoursPerDay,
			OvertimeMultiplier:  req.Payroll.OvertimeMultiplier,
			EPFEmployeePercent:  req.Payroll.EPFEmployeePercent,
			EPFEmployerPercent:  req.Payroll.EPFEmployerPercent,
			ETFPercent:          req.Payroll.ETFPercent,
			TaxPercent:          req.Payroll.TaxPercent,
			DeductAbsentDays:    req.Payroll.DeductAbsentDays,
			DeductUnpaidLeave:   req.Payroll.DeductUnpaidLeave,
		},
	}
}
