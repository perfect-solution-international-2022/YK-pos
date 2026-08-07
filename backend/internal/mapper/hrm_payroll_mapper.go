package mapper

import (
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
)

/*
ToPayrollRunResponse maps a run onto the wire, with its payslips when they were
preloaded (the detail read) and without when they were not (the list).

The run totals are summed here rather than queried, because on the detail read
the payslips are already in hand and on the list read there are none to sum —
either way, a second aggregate query would tell nobody anything new.
*/
func ToPayrollRunResponse(r entity.PayrollRun) dto.PayrollRunResponse {
	res := dto.PayrollRunResponse{
		ID:          r.ID.String(),
		PeriodYear:  r.PeriodYear,
		PeriodMonth: r.PeriodMonth,
		Status:      r.Status,
		Rules:       jsonToMap(r.Rules),
		Notes:       r.Notes,
		FinalizedAt: epochMillisPtr(r.FinalizedAt),
		PaidAt:      epochMillisPtr(r.PaidAt),
		CreatedAt:   epochMillis(r.CreatedAt),
		UpdatedAt:   epochMillis(r.UpdatedAt),
	}

	if len(r.Payslips) == 0 {
		return res
	}

	res.Payslips = make([]dto.PayslipResponse, 0, len(r.Payslips))
	for _, payslip := range r.Payslips {
		res.EmployeeCount++
		res.GrossCents += payslip.GrossCents
		res.DeductionCents += payslip.DeductionCents
		res.NetCents += payslip.NetCents
		res.Payslips = append(res.Payslips, ToPayslipResponse(payslip))
	}
	return res
}

func ToPayrollRunResponseList(rows []entity.PayrollRun) []dto.PayrollRunResponse {
	out := make([]dto.PayrollRunResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToPayrollRunResponse(row))
	}
	return out
}

// ToPayslipResponse maps one employee's pay onto the wire.
func ToPayslipResponse(p entity.Payslip) dto.PayslipResponse {
	res := dto.PayslipResponse{
		ID:           p.ID.String(),
		PayrollRunID: p.PayrollRunID.String(),
		EmployeeID:   p.EmployeeID.String(),

		PeriodYear:  p.PeriodYear,
		PeriodMonth: p.PeriodMonth,

		BasicSalaryCents: p.BasicSalaryCents,
		PayableDays:      p.PayableDays,
		PresentDays:      p.PresentDays,
		AbsentDays:       p.AbsentDays,
		PaidLeaveDays:    p.PaidLeaveDays,
		UnpaidLeaveDays:  p.UnpaidLeaveDays,

		OvertimeMinutes: p.OvertimeMinutes,
		OvertimeCents:   p.OvertimeCents,
		BonusCents:      p.BonusCents,
		AllowanceCents:  p.AllowanceCents,

		AbsenceDeductionCents: p.AbsenceDeductionCents,
		EPFEmployeeCents:      p.EPFEmployeeCents,
		EPFEmployerCents:      p.EPFEmployerCents,
		ETFCents:              p.ETFCents,
		TaxCents:              p.TaxCents,
		OtherDeductionCents:   p.OtherDeductionCents,

		GrossCents:     p.GrossCents,
		DeductionCents: p.DeductionCents,
		NetCents:       p.NetCents,

		Status:    p.Status,
		CreatedAt: epochMillis(p.CreatedAt),
		UpdatedAt: epochMillis(p.UpdatedAt),
	}

	if p.Employee != nil {
		res.EmployeeName = stringPtr(p.Employee.FullName)
		res.EmployeeCode = stringPtr(p.Employee.EmployeeCode)
	}

	if len(p.Items) > 0 {
		res.Items = make([]dto.PayslipItemResponse, 0, len(p.Items))
		for _, item := range p.Items {
			res.Items = append(res.Items, dto.PayslipItemResponse{
				ID:          item.ID.String(),
				Kind:        item.Kind,
				Label:       item.Label,
				AmountCents: item.AmountCents,
			})
		}
	}

	return res
}

func ToPayslipResponseList(rows []entity.Payslip) []dto.PayslipResponse {
	out := make([]dto.PayslipResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToPayslipResponse(row))
	}
	return out
}

// ToPayslipAdjustment builds the service input a draft edit describes.
func ToPayslipAdjustment(req dto.PayslipAdjustRequest) service.PayslipAdjustment {
	items := make([]entity.PayslipItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, entity.PayslipItem{
			Kind:        item.Kind,
			Label:       item.Label,
			AmountCents: item.AmountCents.Int64(),
		})
	}
	return service.PayslipAdjustment{Items: items}
}
