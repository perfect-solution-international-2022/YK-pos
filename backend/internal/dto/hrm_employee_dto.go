package dto

// EmployeeSortFields whitelists the employee list's `sort` values; the first is
// the default.
var EmployeeSortFields = []string{"employee_code", "full_name", "joining_date", "created_at", "status", "basic_salary_cents"}

type EmployeeDocumentResponse struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Type *string `json:"doc_type,omitempty"`
	// Data URL, as with product images. Omitted from list responses — see
	// EmployeeResponse.Documents.
	DataURL   string `json:"data_url,omitempty"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt int64  `json:"created_at"`
}

/*
EmployeeResponse is one staff record.

Documents are present only on the detail read: each is a base64 data URL, so
including them in a page of twenty would make the list response tens of
megabytes. Designation and shift are sent both as ids (for form binding) and as
names (for display), because every screen showing an employee shows both.
*/
type EmployeeResponse struct {
	ID           string `json:"id"`
	EmployeeCode string `json:"employee_code"`

	FullName string `json:"full_name"`
	NIC      string `json:"nic"`
	// yyyy-mm-dd. A calendar day, deliberately not an epoch timestamp.
	DateOfBirth *string `json:"date_of_birth,omitempty"`
	Gender      string  `json:"gender"`
	Phone       string  `json:"phone"`
	Email       *string `json:"email,omitempty"`
	Address     *string `json:"address,omitempty"`

	EmergencyContactName  *string `json:"emergency_contact_name,omitempty"`
	EmergencyContactPhone *string `json:"emergency_contact_phone,omitempty"`

	DesignationID   string  `json:"designation_id"`
	DesignationName *string `json:"designation_name,omitempty"`
	JoiningDate     string  `json:"joining_date"`
	EmploymentType  string  `json:"employment_type"`
	ShiftID         *string `json:"shift_id,omitempty"`
	ShiftName       *string `json:"shift_name,omitempty"`
	BranchID        *string `json:"branch_id,omitempty"`

	BasicSalaryCents int64   `json:"basic_salary_cents"`
	BankName         *string `json:"bank_name,omitempty"`
	BankAccountNo    *string `json:"bank_account_no,omitempty"`
	BankBranch       *string `json:"bank_branch,omitempty"`

	// Set when the employee also has a login. The account itself is managed
	// through the users endpoints, not here.
	UserID *string `json:"user_id,omitempty"`

	PhotoURL   *string `json:"photo_url,omitempty"`
	Status     string  `json:"status"`
	ResignedAt *string `json:"resigned_at,omitempty"`
	Notes      *string `json:"notes,omitempty"`

	Documents []EmployeeDocumentResponse `json:"documents,omitempty"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

/*
EmployeeLoginRequest optionally provisions a till/back-office account alongside
the HR record.

The login identifier is the email address — the same one the rest of this API
authenticates on. There is no separate username: adding one here would give the
system a second way to sign in that the auth module knows nothing about.
Password rules match the auth module's exactly, bcryptsafe included, because the
account this creates is an ordinary user row.
*/
type EmployeeLoginRequest struct {
	// Defaults to the employee's own email when omitted.
	Email    string   `json:"email" validate:"omitempty,email,max=254"`
	Password string   `json:"password" validate:"required,min=8,bcryptsafe"`
	RoleIDs  []string `json:"role_ids" validate:"required,min=1,dive,uuid"`
}

/*
EmployeeCreateRequest is the POST /hrm/employees body.

employee_code is absent on purpose: it is allocated by the server, sequentially
per business, so two people can never be hired into the same number.
*/
type EmployeeCreateRequest struct {
	FullName    string  `json:"full_name" validate:"required,min=2,max=120"`
	NIC         string  `json:"nic" validate:"required,min=5,max=20"`
	DateOfBirth *string `json:"date_of_birth" validate:"omitempty,datetime=2006-01-02"`
	Gender      string  `json:"gender" validate:"omitempty,oneof=male female other"`
	Phone       string  `json:"phone" validate:"required,min=6,max=20"`
	Email       *string `json:"email" validate:"omitempty,email,max=254"`
	Address     *string `json:"address" validate:"omitempty,max=500"`

	EmergencyContactName  *string `json:"emergency_contact_name" validate:"omitempty,max=120"`
	EmergencyContactPhone *string `json:"emergency_contact_phone" validate:"omitempty,max=20"`

	DesignationID  string  `json:"designation_id" validate:"required,uuid"`
	JoiningDate    string  `json:"joining_date" validate:"required,datetime=2006-01-02"`
	EmploymentType string  `json:"employment_type" validate:"omitempty,oneof=full_time part_time contract intern"`
	ShiftID        *string `json:"shift_id" validate:"omitempty,uuid"`
	BranchID       *string `json:"branch_id" validate:"omitempty,uuid"`

	// Integer cents, and Cents rather than int64 for the same reason the order
	// DTOs use it: a client that computes a salary in floats must not have the
	// save rejected over a fractional cent.
	BasicSalaryCents Cents   `json:"basic_salary_cents" validate:"min=0"`
	BankName         *string `json:"bank_name" validate:"omitempty,max=120"`
	BankAccountNo    *string `json:"bank_account_no" validate:"omitempty,max=40"`
	BankBranch       *string `json:"bank_branch" validate:"omitempty,max=120"`

	// Data URL.
	PhotoURL *string `json:"photo_url"`
	Status   string  `json:"status" validate:"omitempty,oneof=active inactive suspended resigned"`
	Notes    *string `json:"notes" validate:"omitempty,max=2000"`

	Login *EmployeeLoginRequest `json:"login" validate:"omitempty"`
}

/*
EmployeeUpdateRequest is the PUT /hrm/employees/{id} body: a full replacement of
the editable fields, the same contract as PUT /products.

Not replaceable, and absent for that reason:
  - employee_code — allocated once and never reissued, or two people would share
    an identifier across payroll history.
  - login — an account's password and roles move through the auth and users
    endpoints, which know about the token denylist and the audit trail.
*/
type EmployeeUpdateRequest struct {
	FullName    string  `json:"full_name" validate:"required,min=2,max=120"`
	NIC         string  `json:"nic" validate:"required,min=5,max=20"`
	DateOfBirth *string `json:"date_of_birth" validate:"omitempty,datetime=2006-01-02"`
	Gender      string  `json:"gender" validate:"omitempty,oneof=male female other"`
	Phone       string  `json:"phone" validate:"required,min=6,max=20"`
	Email       *string `json:"email" validate:"omitempty,email,max=254"`
	Address     *string `json:"address" validate:"omitempty,max=500"`

	EmergencyContactName  *string `json:"emergency_contact_name" validate:"omitempty,max=120"`
	EmergencyContactPhone *string `json:"emergency_contact_phone" validate:"omitempty,max=20"`

	DesignationID  string  `json:"designation_id" validate:"required,uuid"`
	JoiningDate    string  `json:"joining_date" validate:"required,datetime=2006-01-02"`
	EmploymentType string  `json:"employment_type" validate:"omitempty,oneof=full_time part_time contract intern"`
	ShiftID        *string `json:"shift_id" validate:"omitempty,uuid"`
	BranchID       *string `json:"branch_id" validate:"omitempty,uuid"`

	BasicSalaryCents Cents   `json:"basic_salary_cents" validate:"min=0"`
	BankName         *string `json:"bank_name" validate:"omitempty,max=120"`
	BankAccountNo    *string `json:"bank_account_no" validate:"omitempty,max=40"`
	BankBranch       *string `json:"bank_branch" validate:"omitempty,max=120"`

	PhotoURL   *string `json:"photo_url"`
	Status     string  `json:"status" validate:"omitempty,oneof=active inactive suspended resigned"`
	ResignedAt *string `json:"resigned_at" validate:"omitempty,datetime=2006-01-02"`
	Notes      *string `json:"notes" validate:"omitempty,max=2000"`
}

/*
EmployeeDocumentRequest uploads one file as a data URL, the same way product
images travel. JSON rather than multipart keeps one content type across the API
and lets the client reuse the reader it already has for photos.

The 5 MB cap is on the encoded string; base64 inflates by about a third, so it
admits roughly a 3.7 MB file. The server's global body limit still applies on
top.
*/
type EmployeeDocumentRequest struct {
	Name      string  `json:"name" validate:"required,min=1,max=160"`
	DocType   *string `json:"doc_type" validate:"omitempty,max=60"`
	DataURL   string  `json:"data_url" validate:"required,max=5242880"`
	SizeBytes int64   `json:"size_bytes" validate:"min=0"`
}

/*
EmployeeListQuery is the GET /hrm/employees query string.
*/
type EmployeeListQuery struct {
	PaginationQuery

	DesignationID  string `query:"designation_id" validate:"omitempty,uuid"`
	ShiftID        string `query:"shift_id" validate:"omitempty,uuid"`
	BranchID       string `query:"branch_id" validate:"omitempty,uuid"`
	EmploymentType string `query:"employment_type" validate:"omitempty,oneof=full_time part_time contract intern"`
	Status         string `query:"status" validate:"omitempty,oneof=active inactive suspended resigned"`
}
