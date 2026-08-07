package mapper

import (
	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

// ToPerformanceReviewResponse maps one evaluation onto the wire.
func ToPerformanceReviewResponse(r entity.PerformanceReview) dto.PerformanceReviewResponse {
	res := dto.PerformanceReviewResponse{
		ID:         r.ID.String(),
		EmployeeID: r.EmployeeID.String(),

		PeriodYear:  r.PeriodYear,
		PeriodMonth: r.PeriodMonth,

		Rating:       r.Rating,
		Punctuality:  r.Punctuality,
		Teamwork:     r.Teamwork,
		Productivity: r.Productivity,

		ManagerNotes: r.ManagerNotes,
		ReviewerID:   uuidPtrToStringPtr(r.ReviewerID),

		CreatedAt: epochMillis(r.CreatedAt),
		UpdatedAt: epochMillis(r.UpdatedAt),
	}

	if r.Employee != nil {
		res.EmployeeName = stringPtr(r.Employee.FullName)
		res.EmployeeCode = stringPtr(r.Employee.EmployeeCode)
	}
	return res
}

func ToPerformanceReviewResponseList(rows []entity.PerformanceReview) []dto.PerformanceReviewResponse {
	out := make([]dto.PerformanceReviewResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToPerformanceReviewResponse(row))
	}
	return out
}

// ToPerformanceReviewEntity builds the row a create request describes. The
// employee id is parsed rather than validated here — the DTO's uuid tag has
// already rejected anything unparseable.
func ToPerformanceReviewEntity(
	req dto.PerformanceReviewCreateRequest, reviewerID *uuid.UUID,
) entity.PerformanceReview {
	employeeID, _ := uuid.Parse(req.EmployeeID)

	return entity.PerformanceReview{
		EmployeeID:   employeeID,
		PeriodYear:   req.PeriodYear,
		PeriodMonth:  req.PeriodMonth,
		Rating:       req.Rating,
		Punctuality:  req.Punctuality,
		Teamwork:     req.Teamwork,
		Productivity: req.Productivity,
		ManagerNotes: req.ManagerNotes,
		ReviewerID:   reviewerID,
	}
}

/*
ApplyPerformanceReviewUpdate overwrites the scores and notes. The employee and
the period are not touched: moving a review to another month is filing a
different review, and would collide with whatever already sits there.

The reviewer is reassigned to whoever made the edit, because the notes now
reflect their judgement rather than the original author's.
*/
func ApplyPerformanceReviewUpdate(
	r *entity.PerformanceReview, req dto.PerformanceReviewUpdateRequest, reviewerID *uuid.UUID,
) {
	r.Rating = req.Rating
	r.Punctuality = req.Punctuality
	r.Teamwork = req.Teamwork
	r.Productivity = req.Productivity
	r.ManagerNotes = req.ManagerNotes
	if reviewerID != nil {
		r.ReviewerID = reviewerID
	}
}

// ToAnnouncementResponse maps one notice onto the wire.
func ToAnnouncementResponse(a entity.Announcement) dto.AnnouncementResponse {
	return dto.AnnouncementResponse{
		ID:          a.ID.String(),
		Title:       a.Title,
		Description: a.Description,
		PublishDate: isoDate(a.PublishDate),
		ExpiresOn:   isoDatePtr(a.ExpiresOn),
		Status:      a.Status,
		CreatedBy:   uuidPtrToStringPtr(a.CreatedBy),
		CreatedAt:   epochMillis(a.CreatedAt),
		UpdatedAt:   epochMillis(a.UpdatedAt),
	}
}

func ToAnnouncementResponseList(rows []entity.Announcement) []dto.AnnouncementResponse {
	out := make([]dto.AnnouncementResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToAnnouncementResponse(row))
	}
	return out
}

// ApplyAnnouncementRequest writes a request onto a notice.
func ApplyAnnouncementRequest(a *entity.Announcement, req dto.AnnouncementRequest) {
	a.Title = req.Title
	a.Description = req.Description
	a.PublishDate = parseISODateValue(req.PublishDate)
	a.ExpiresOn = parseISODate(req.ExpiresOn)
	if req.Status != "" {
		a.Status = req.Status
	}
}

/*
ToAuditLogResponse maps one entry of the trail onto the wire.

The module is the resource type — "hrm.employee", "hrm.payroll" — which is what
the audit screen filters on. Old and new values decode from their JSONB columns;
a malformed blob yields nil rather than failing the page, because one bad entry
must not hide the rest of the trail.
*/
func ToAuditLogResponse(a entity.AuditLog) dto.AuditLogResponse {
	return dto.AuditLogResponse{
		ID:     a.ID.String(),
		UserID: uuidPtrToStringPtr(a.UserID),
		Action: a.Action,
		Module: a.ResourceType,
		Status: a.Status,

		ResourceID: uuidPtrToStringPtr(a.ResourceID),
		OldValues:  jsonToMap(a.OldValues),
		NewValues:  jsonToMap(a.NewValues),

		IPAddress: a.IPAddress,
		UserAgent: a.UserAgent,
		RequestID: a.RequestID,

		CreatedAt: epochMillis(a.CreatedAt),
	}
}

func ToAuditLogResponseList(rows []entity.AuditLog) []dto.AuditLogResponse {
	out := make([]dto.AuditLogResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToAuditLogResponse(row))
	}
	return out
}
