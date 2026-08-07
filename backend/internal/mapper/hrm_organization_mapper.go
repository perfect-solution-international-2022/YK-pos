package mapper

import (
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

// ToDesignationResponse maps a job title onto the wire.
func ToDesignationResponse(d entity.Designation) dto.DesignationResponse {
	return dto.DesignationResponse{
		ID:          d.ID.String(),
		Name:        d.Name,
		Description: d.Description,
		Status:      d.Status,
		CreatedAt:   epochMillis(d.CreatedAt),
		UpdatedAt:   epochMillis(d.UpdatedAt),
	}
}

// ToDesignationResponseList always returns a non-nil slice: encoding/json
// renders a nil slice as null, and the client maps over the response directly.
func ToDesignationResponseList(rows []entity.Designation) []dto.DesignationResponse {
	out := make([]dto.DesignationResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToDesignationResponse(row))
	}
	return out
}

// ApplyDesignationRequest writes a request onto a designation. Every field is
// assigned, so clearing a description actually clears it — the replacement
// semantics the endpoint documents.
func ApplyDesignationRequest(d *entity.Designation, req dto.DesignationRequest) {
	d.Name = req.Name
	d.Description = req.Description
	// Empty is not a legal column value (there is a CHECK), so an absent status
	// keeps what is stored rather than clearing it.
	if req.Status != "" {
		d.Status = req.Status
	}
}

// ToShiftResponse maps a working pattern onto the wire, including the derived
// scheduled length so the client does not re-implement the midnight arithmetic.
func ToShiftResponse(s entity.Shift) dto.ShiftResponse {
	return dto.ShiftResponse{
		ID:               s.ID.String(),
		Name:             s.Name,
		StartTime:        formatClock(s.StartTime),
		EndTime:          formatClock(s.EndTime),
		BreakMinutes:     s.BreakMinutes,
		GraceMinutes:     s.GraceMinutes,
		WeeklyOff:        jsonToIntSlice(s.WeeklyOff),
		Status:           s.Status,
		ScheduledMinutes: s.ScheduledMinutes(),
		CreatedAt:        epochMillis(s.CreatedAt),
		UpdatedAt:        epochMillis(s.UpdatedAt),
	}
}

func ToShiftResponseList(rows []entity.Shift) []dto.ShiftResponse {
	out := make([]dto.ShiftResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToShiftResponse(row))
	}
	return out
}

// ApplyShiftRequest writes a request onto a shift.
func ApplyShiftRequest(s *entity.Shift, req dto.ShiftRequest) {
	s.Name = req.Name
	s.StartTime = parseClock(req.StartTime)
	s.EndTime = parseClock(req.EndTime)
	s.BreakMinutes = req.BreakMinutes
	s.GraceMinutes = req.GraceMinutes
	s.WeeklyOff = intSliceToJSON(req.WeeklyOff)
	if req.Status != "" {
		s.Status = req.Status
	}
}
