package entity

import (
	"time"

	"github.com/google/uuid"
)

const (
	GenderMale   = "male"
	GenderFemale = "female"
	GenderOther  = "other"
)

const (
	EmploymentTypeFullTime = "full_time"
	EmploymentTypePartTime = "part_time"
	EmploymentTypeContract = "contract"
	EmploymentTypeIntern   = "intern"
)

// Employment lifecycle. "inactive" is a temporary pause (unpaid sabbatical),
// "suspended" is disciplinary, "resigned" is terminal — payroll pays none of
// the three, but only "resigned" stops the person appearing on a roster.
const (
	EmployeeStatusActive    = "active"
	EmployeeStatusInactive  = "inactive"
	EmployeeStatusSuspended = "suspended"
	EmployeeStatusResigned  = "resigned"
)

// Employee is the HR record for a member of staff.
//
// UserID is the optional link to a login. Not every employee gets one — a
// warehouse hand may never touch the till — and the FK is ON DELETE SET NULL so
// closing an account never takes payroll history with it. The HR record is
// therefore the person; the user row is only their key to the building.
//
// Money is integer cents, as everywhere else. DateOfBirth, JoiningDate and
// ResignedAt are calendar days (DATE columns): no time-of-day, no timezone.
type Employee struct {
	IDMixin
	Timestamps
	SoftDelete

	BusinessID uuid.UUID  `gorm:"column:business_id;not null"`
	BranchID   *uuid.UUID `gorm:"column:branch_id"`
	UserID     *uuid.UUID `gorm:"column:user_id"`

	// Sequential within a business, assigned by the service on create. Text
	// rather than an integer because shops number staff with prefixes and
	// leading zeros.
	EmployeeCode string `gorm:"column:employee_code;not null"`

	FullName    string     `gorm:"column:full_name;not null"`
	NIC         string     `gorm:"column:nic;not null"`
	DateOfBirth *time.Time `gorm:"column:date_of_birth;type:date"`
	Gender      string     `gorm:"column:gender;not null"`
	Phone       string     `gorm:"column:phone;not null"`
	Email       *string    `gorm:"column:email"`
	Address     *string    `gorm:"column:address"`

	EmergencyContactName  *string `gorm:"column:emergency_contact_name"`
	EmergencyContactPhone *string `gorm:"column:emergency_contact_phone"`

	DesignationID  uuid.UUID  `gorm:"column:designation_id;not null"`
	JoiningDate    time.Time  `gorm:"column:joining_date;type:date;not null"`
	EmploymentType string     `gorm:"column:employment_type;not null"`
	ShiftID        *uuid.UUID `gorm:"column:shift_id"`

	BasicSalaryCents int64   `gorm:"column:basic_salary_cents;not null"`
	BankName         *string `gorm:"column:bank_name"`
	BankAccountNo    *string `gorm:"column:bank_account_no"`
	BankBranch       *string `gorm:"column:bank_branch"`

	// Data URL, the same way products carry their images in this app.
	PhotoURL *string `gorm:"column:photo_url"`

	Status     string     `gorm:"column:status;not null"`
	ResignedAt *time.Time `gorm:"column:resigned_at;type:date"`
	Notes      *string    `gorm:"column:notes"`

	// Loaded explicitly via Preload. The list screen never needs them and a
	// document is a base64 data URL, so pulling them by default would make
	// every page of the employee list tens of megabytes.
	Documents []EmployeeDocument `gorm:"foreignKey:EmployeeID;references:ID"`

	// Preloaded for the detail and list responses, which show the job title and
	// shift name rather than raw ids.
	Designation *Designation `gorm:"foreignKey:DesignationID;references:ID"`
	Shift       *Shift       `gorm:"foreignKey:ShiftID;references:ID"`
}

func (Employee) TableName() string { return "hrm_employees" }

// IsPayable reports whether the employee draws a salary for the period. Only
// active staff do; the other three states are all forms of "not on the payroll
// this month".
func (e Employee) IsPayable() bool { return e.Status == EmployeeStatusActive }

// EmployeeDocument is one uploaded file — an NIC scan, a contract, a
// certificate — held as a data URL the same way product images are.
type EmployeeDocument struct {
	IDMixin
	Timestamps

	EmployeeID uuid.UUID  `gorm:"column:employee_id;not null"`
	Name       string     `gorm:"column:name;not null"`
	DocType    *string    `gorm:"column:doc_type"`
	DataURL    string     `gorm:"column:data_url;not null"`
	SizeBytes  int64      `gorm:"column:size_bytes;not null"`
	UploadedBy *uuid.UUID `gorm:"column:uploaded_by"`
}

func (EmployeeDocument) TableName() string { return "hrm_employee_documents" }
