package mapper

import (
	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
)

/*
ToEmployeeResponse maps a staff record onto the wire.

Documents are included only when they were preloaded — the list read does not
load them, because each is a base64 data URL and a page of twenty would run to
tens of megabytes. The data URL itself is omitted from these entries for the
same reason; the detail read carries it.
*/
func ToEmployeeResponse(e entity.Employee) dto.EmployeeResponse {
	res := dto.EmployeeResponse{
		ID:           e.ID.String(),
		EmployeeCode: e.EmployeeCode,

		FullName:    e.FullName,
		NIC:         e.NIC,
		DateOfBirth: isoDatePtr(e.DateOfBirth),
		Gender:      e.Gender,
		Phone:       e.Phone,
		Email:       e.Email,
		Address:     e.Address,

		EmergencyContactName:  e.EmergencyContactName,
		EmergencyContactPhone: e.EmergencyContactPhone,

		DesignationID:  e.DesignationID.String(),
		JoiningDate:    isoDate(e.JoiningDate),
		EmploymentType: e.EmploymentType,
		ShiftID:        uuidPtrToStringPtr(e.ShiftID),
		BranchID:       uuidPtrToStringPtr(e.BranchID),

		BasicSalaryCents: e.BasicSalaryCents,
		BankName:         e.BankName,
		BankAccountNo:    e.BankAccountNo,
		BankBranch:       e.BankBranch,

		UserID: uuidPtrToStringPtr(e.UserID),

		PhotoURL:   e.PhotoURL,
		Status:     e.Status,
		ResignedAt: isoDatePtr(e.ResignedAt),
		Notes:      e.Notes,

		CreatedAt: epochMillis(e.CreatedAt),
		UpdatedAt: epochMillis(e.UpdatedAt),
	}

	if e.Designation != nil {
		res.DesignationName = stringPtr(e.Designation.Name)
	}
	if e.Shift != nil {
		res.ShiftName = stringPtr(e.Shift.Name)
	}

	if len(e.Documents) > 0 {
		res.Documents = make([]dto.EmployeeDocumentResponse, 0, len(e.Documents))
		for _, doc := range e.Documents {
			entry := ToEmployeeDocumentResponse(doc)
			entry.DataURL = ""
			res.Documents = append(res.Documents, entry)
		}
	}

	return res
}

func ToEmployeeResponseList(rows []entity.Employee) []dto.EmployeeResponse {
	out := make([]dto.EmployeeResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToEmployeeResponse(row))
	}
	return out
}

// ToEmployeeDocumentResponse maps one uploaded file, data URL included. The
// employee mappers above strip it; the document endpoints keep it.
func ToEmployeeDocumentResponse(d entity.EmployeeDocument) dto.EmployeeDocumentResponse {
	return dto.EmployeeDocumentResponse{
		ID:        d.ID.String(),
		Name:      d.Name,
		Type:      d.DocType,
		DataURL:   d.DataURL,
		SizeBytes: d.SizeBytes,
		CreatedAt: epochMillis(d.CreatedAt),
	}
}

func ToEmployeeDocumentResponseList(rows []entity.EmployeeDocument) []dto.EmployeeDocumentResponse {
	out := make([]dto.EmployeeDocumentResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ToEmployeeDocumentResponse(row))
	}
	return out
}

/*
ToEmployeeEntity builds the row a create request describes.

business_id, employee_code and user_id are absent: the first two come from the
token and the server's allocator, and the third is set by the service when a
login is provisioned. The designation id is parsed rather than validated here —
the DTO's uuid tag has already rejected anything unparseable.
*/
func ToEmployeeEntity(req dto.EmployeeCreateRequest) entity.Employee {
	designationID, _ := uuid.Parse(req.DesignationID)

	return entity.Employee{
		FullName:    req.FullName,
		NIC:         req.NIC,
		DateOfBirth: parseISODate(req.DateOfBirth),
		Gender:      req.Gender,
		Phone:       req.Phone,
		Email:       req.Email,
		Address:     req.Address,

		EmergencyContactName:  req.EmergencyContactName,
		EmergencyContactPhone: req.EmergencyContactPhone,

		DesignationID:  designationID,
		JoiningDate:    parseISODateValue(req.JoiningDate),
		EmploymentType: req.EmploymentType,
		ShiftID:        parseUUIDPtr(req.ShiftID),
		BranchID:       parseUUIDPtr(req.BranchID),

		BasicSalaryCents: req.BasicSalaryCents.Int64(),
		BankName:         req.BankName,
		BankAccountNo:    req.BankAccountNo,
		BankBranch:       req.BankBranch,

		PhotoURL: req.PhotoURL,
		Status:   req.Status,
		Notes:    req.Notes,
	}
}

/*
ApplyEmployeeUpdate overwrites an employee's editable fields with the request's.

Every mutable field is assigned unconditionally, pointers included: that is what
makes an omitted optional field clear the column, which is the whole point of
replacement semantics. Untouched on purpose: ID, BusinessID, EmployeeCode,
UserID and the timestamps.
*/
func ApplyEmployeeUpdate(e *entity.Employee, req dto.EmployeeUpdateRequest) {
	designationID, _ := uuid.Parse(req.DesignationID)

	e.FullName = req.FullName
	e.NIC = req.NIC
	e.DateOfBirth = parseISODate(req.DateOfBirth)
	e.Phone = req.Phone
	e.Email = req.Email
	e.Address = req.Address

	e.EmergencyContactName = req.EmergencyContactName
	e.EmergencyContactPhone = req.EmergencyContactPhone

	e.DesignationID = designationID
	e.JoiningDate = parseISODateValue(req.JoiningDate)
	e.ShiftID = parseUUIDPtr(req.ShiftID)
	e.BranchID = parseUUIDPtr(req.BranchID)

	e.BasicSalaryCents = req.BasicSalaryCents.Int64()
	e.BankName = req.BankName
	e.BankAccountNo = req.BankAccountNo
	e.BankBranch = req.BankBranch

	e.PhotoURL = req.PhotoURL
	e.ResignedAt = parseISODate(req.ResignedAt)
	e.Notes = req.Notes

	// Empty is not a legal value for any of these three — each column has a
	// CHECK — so an absent one keeps what is already stored.
	if req.Gender != "" {
		e.Gender = req.Gender
	}
	if req.EmploymentType != "" {
		e.EmploymentType = req.EmploymentType
	}
	if req.Status != "" {
		e.Status = req.Status
	}
}

// ToEmployeeDocumentEntity builds the row an upload describes.
func ToEmployeeDocumentEntity(req dto.EmployeeDocumentRequest, uploadedBy *uuid.UUID) entity.EmployeeDocument {
	return entity.EmployeeDocument{
		Name:       req.Name,
		DocType:    req.DocType,
		DataURL:    req.DataURL,
		SizeBytes:  req.SizeBytes,
		UploadedBy: uploadedBy,
	}
}

/*
ToEmployeeLogin builds the optional account request. Returns nil when the body
carried no login block, which is the common case — most employees never get a
till account.
*/
func ToEmployeeLogin(req *dto.EmployeeLoginRequest) (*service.EmployeeLogin, error) {
	if req == nil {
		return nil, nil
	}

	roleIDs := make([]uuid.UUID, 0, len(req.RoleIDs))
	for _, raw := range req.RoleIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, err
		}
		roleIDs = append(roleIDs, id)
	}

	return &service.EmployeeLogin{
		Email:    req.Email,
		Password: req.Password,
		RoleIDs:  roleIDs,
	}, nil
}
