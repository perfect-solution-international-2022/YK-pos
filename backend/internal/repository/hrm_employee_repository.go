package repository

import (
	"context"
	"encoding/binary"
	"strconv"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

//go:generate go run go.uber.org/mock/mockgen -source=hrm_employee_repository.go -destination=mocks/hrm_employee_repository_mock.go -package=mocks

/*
EmployeeListParams narrows the employee list. Every field is optional; the zero
value lists everyone in the business.
*/
type EmployeeListParams struct {
	ListParams

	// IDs narrows to an explicit set — how the reports resolve names for the
	// employees an aggregate query already grouped by, without also having to
	// filter on status.
	IDs            []uuid.UUID
	DesignationID  *uuid.UUID
	ShiftID        *uuid.UUID
	BranchID       *uuid.UUID
	EmploymentType string
	Status         string
}

/*
EmployeeRepository persists staff records.

Documents are never loaded by the list read: each one is a base64 data URL, so
preloading them would turn a page of twenty employees into tens of megabytes.
Designation and shift are preloaded, because every screen showing an employee
shows their job title and roster by name.
*/
type EmployeeRepository interface {
	List(ctx context.Context, businessID uuid.UUID, params EmployeeListParams) ([]entity.Employee, int64, error)
	// FindByID loads one employee with documents, designation and shift.
	FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Employee, error)
	// FindByUserID resolves the employee behind a login, for the self-service
	// paths (clock in/out, my leave) where the caller is the employee.
	FindByUserID(ctx context.Context, businessID, userID uuid.UUID) (*entity.Employee, error)
	// FindByNIC and FindByEmail back the duplicate checks, so a genuine clash
	// is reported before the partial unique index rejects the insert with a
	// driver error nothing above can interpret.
	FindByNIC(ctx context.Context, businessID uuid.UUID, nic string) (*entity.Employee, error)
	FindByEmail(ctx context.Context, businessID uuid.UUID, email string) (*entity.Employee, error)
	Create(ctx context.Context, tx *gorm.DB, e *entity.Employee) error
	Update(ctx context.Context, e *entity.Employee) error
	// SetUserID links (or unlinks) the login without touching any other column,
	// so creating an account for an existing employee is one narrow write.
	SetUserID(ctx context.Context, tx *gorm.DB, businessID, id uuid.UUID, userID *uuid.UUID) error
	Delete(ctx context.Context, businessID, id uuid.UUID) error

	// NextEmployeeCode returns the next sequential code for the business. It
	// must be called inside a transaction: it takes a per-business advisory
	// lock so two concurrent hires cannot both read the same maximum.
	NextEmployeeCode(ctx context.Context, tx *gorm.DB, businessID uuid.UUID) (string, error)

	// ListPayable returns the active employees a payroll run covers, ordered by
	// code so a run's payslips come out in a stable, printable order.
	ListPayable(ctx context.Context, businessID uuid.UUID, ids []uuid.UUID) ([]entity.Employee, error)

	AddDocument(ctx context.Context, doc *entity.EmployeeDocument) error
	ListDocuments(ctx context.Context, employeeID uuid.UUID) ([]entity.EmployeeDocument, error)
	DeleteDocument(ctx context.Context, employeeID, documentID uuid.UUID) error
}

type employeeRepository struct {
	db *gorm.DB
}

func NewEmployeeRepository(db *gorm.DB) EmployeeRepository {
	return &employeeRepository{db: db}
}

func (r *employeeRepository) List(
	ctx context.Context, businessID uuid.UUID, params EmployeeListParams,
) ([]entity.Employee, int64, error) {
	var total int64
	if err := r.scoped(ctx, businessID, params).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entity.Employee{}, 0, nil
	}

	var rows []entity.Employee
	err := paginate(r.scoped(ctx, businessID, params), params.ListParams).
		Preload("Designation").
		Preload("Shift").
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

/*
scoped builds the filtered query shared by the count and the page fetch, so the
two cannot drift and report a total that paging through does not yield.
*/
func (r *employeeRepository) scoped(
	ctx context.Context, businessID uuid.UUID, params EmployeeListParams,
) *gorm.DB {
	query := r.db.WithContext(ctx).
		Model(&entity.Employee{}).
		Where("business_id = ?", businessID)

	if len(params.IDs) > 0 {
		query = query.Where("id IN ?", params.IDs)
	}
	if params.DesignationID != nil {
		query = query.Where("designation_id = ?", *params.DesignationID)
	}
	if params.ShiftID != nil {
		query = query.Where("shift_id = ?", *params.ShiftID)
	}
	if params.BranchID != nil {
		query = query.Where("branch_id = ?", *params.BranchID)
	}
	if params.EmploymentType != "" {
		query = query.Where("employment_type = ?", params.EmploymentType)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if like := likePattern(params.Search); like != "" {
		/*
			The five identifiers an HR officer actually searches by. Address and
			notes are deliberately excluded: matching free text would make every
			search return most of the shop.
		*/
		query = query.Where(
			`(full_name ILIKE ? OR employee_code ILIKE ? OR nic ILIKE ?
			  OR phone ILIKE ? OR email ILIKE ?)`,
			like, like, like, like, like,
		)
	}

	return query
}

func (r *employeeRepository) FindByID(ctx context.Context, businessID, id uuid.UUID) (*entity.Employee, error) {
	var row entity.Employee
	err := r.db.WithContext(ctx).
		Preload("Designation").
		Preload("Shift").
		Preload("Documents", func(db *gorm.DB) *gorm.DB { return db.Order("created_at DESC") }).
		Where("business_id = ? AND id = ?", businessID, id).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *employeeRepository) FindByUserID(ctx context.Context, businessID, userID uuid.UUID) (*entity.Employee, error) {
	var row entity.Employee
	err := r.db.WithContext(ctx).
		Preload("Shift").
		Where("business_id = ? AND user_id = ?", businessID, userID).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *employeeRepository) FindByNIC(ctx context.Context, businessID uuid.UUID, nic string) (*entity.Employee, error) {
	var row entity.Employee
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND upper(nic) = upper(?)", businessID, nic).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *employeeRepository) FindByEmail(ctx context.Context, businessID uuid.UUID, email string) (*entity.Employee, error) {
	var row entity.Employee
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND lower(email) = lower(?)", businessID, email).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

/*
Create inserts the employee row only. Documents arrive through their own
endpoint, and the association is omitted so a create carrying a preloaded
designation cannot write to that table by accident.
*/
func (r *employeeRepository) Create(ctx context.Context, tx *gorm.DB, e *entity.Employee) error {
	return r.conn(ctx, tx).Omit("Documents", "Designation", "Shift").Create(e).Error
}

/*
Update writes an explicit column map for the same reason productRepository does:
a struct update skips zero values, so clearing an optional field — a bank
account that turned out to be wrong, an emergency contact who moved — would be
silently dropped rather than applied.

employee_code, user_id, business_id and the timestamps are absent on purpose.
The code is assigned once and never reissued, the login link moves through
SetUserID, and updated_at is the table trigger's job.
*/
func (r *employeeRepository) Update(ctx context.Context, e *entity.Employee) error {
	res := r.db.WithContext(ctx).
		Model(&entity.Employee{}).
		Where("business_id = ? AND id = ?", e.BusinessID, e.ID).
		Updates(map[string]any{
			"branch_id":               e.BranchID,
			"full_name":               e.FullName,
			"nic":                     e.NIC,
			"date_of_birth":           e.DateOfBirth,
			"gender":                  e.Gender,
			"phone":                   e.Phone,
			"email":                   e.Email,
			"address":                 e.Address,
			"emergency_contact_name":  e.EmergencyContactName,
			"emergency_contact_phone": e.EmergencyContactPhone,
			"designation_id":          e.DesignationID,
			"joining_date":            e.JoiningDate,
			"employment_type":         e.EmploymentType,
			"shift_id":                e.ShiftID,
			"basic_salary_cents":      e.BasicSalaryCents,
			"bank_name":               e.BankName,
			"bank_account_no":         e.BankAccountNo,
			"bank_branch":             e.BankBranch,
			"photo_url":               e.PhotoURL,
			"status":                  e.Status,
			"resigned_at":             e.ResignedAt,
			"notes":                   e.Notes,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *employeeRepository) SetUserID(
	ctx context.Context, tx *gorm.DB, businessID, id uuid.UUID, userID *uuid.UUID,
) error {
	res := r.conn(ctx, tx).
		Model(&entity.Employee{}).
		Where("business_id = ? AND id = ?", businessID, id).
		Update("user_id", userID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

/*
Delete soft-deletes. Payslips reference the employee with ON DELETE RESTRICT, so
the row has to survive for a historical payslip to still name who it paid; the
partial unique indexes are scoped to non-deleted rows, which frees the NIC and
the employee code for reuse.
*/
func (r *employeeRepository) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("business_id = ? AND id = ?", businessID, id).
		Delete(&entity.Employee{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

/*
employeeCodeLockClass namespaces the advisory lock NextEmployeeCode takes, so it
can never collide with a lock some other feature takes on the same business.
*/
const employeeCodeLockClass = 1001

/*
NextEmployeeCode reads the highest numeric code in the business and returns the
next one, starting at 1001 for a shop's first hire.

A read-then-insert of a maximum is a classic race: two hires entered at the same
moment both read 1042 and both try to insert 1043, and one of them dies on the
unique index having already created a user account. The transaction-scoped
advisory lock serialises just this allocation, per business — it is released by
commit or rollback with no unlock call to forget, and it blocks nothing except
another hire at the same shop in the same instant. Which is why the tx argument
is not optional: outside a transaction the lock would be released immediately
and buy nothing.

Codes that are not numeric (a shop that migrated in "A-14") are skipped by the
regex rather than failing the cast — they keep their code, and generated ones
carry on from the numeric series. Soft-deleted rows are deliberately included in
the maximum: reissuing a departed employee's code would make two people share an
identifier across payroll history.
*/
func (r *employeeRepository) NextEmployeeCode(ctx context.Context, tx *gorm.DB, businessID uuid.UUID) (string, error) {
	db := r.conn(ctx, tx)

	if err := db.Exec(
		"SELECT pg_advisory_xact_lock(?, ?)", employeeCodeLockClass, lockKey(businessID),
	).Error; err != nil {
		return "", err
	}

	/*
		The regex guard is inside the CASE rather than in the WHERE clause:
		Postgres may evaluate a cast in the target list before the filter that
		was meant to protect it, so `WHERE code ~ '^[0-9]+$'` alongside
		`MAX(code::BIGINT)` can still fail on a non-numeric row. CASE fixes the
		order.
	*/
	var highest int64
	err := db.Raw(
		`SELECT COALESCE(
		     MAX(CASE WHEN employee_code ~ '^[0-9]+$' THEN employee_code::BIGINT END),
		     1000
		 )
		 FROM hrm_employees
		 WHERE business_id = ?`, businessID,
	).Scan(&highest).Error
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(highest+1, 10), nil
}

/*
lockKey folds a UUID into the int32 Postgres advisory locks are keyed on.

Collisions between two businesses are possible and harmless: the worst case is
that two shops briefly serialise their hiring against each other. Correctness
does not depend on the key being unique, only on it being stable.
*/
func lockKey(id uuid.UUID) int32 {
	raw := id[:]
	return int32(binary.BigEndian.Uint32(raw[:4]) ^ binary.BigEndian.Uint32(raw[4:8]) ^
		binary.BigEndian.Uint32(raw[8:12]) ^ binary.BigEndian.Uint32(raw[12:16]))
}

func (r *employeeRepository) ListPayable(
	ctx context.Context, businessID uuid.UUID, ids []uuid.UUID,
) ([]entity.Employee, error) {
	query := r.db.WithContext(ctx).
		Where("business_id = ? AND status = ?", businessID, entity.EmployeeStatusActive)
	if len(ids) > 0 {
		query = query.Where("id IN ?", ids)
	}

	var rows []entity.Employee
	err := query.Order("employee_code").Find(&rows).Error
	return rows, err
}

func (r *employeeRepository) AddDocument(ctx context.Context, doc *entity.EmployeeDocument) error {
	return r.db.WithContext(ctx).Create(doc).Error
}

func (r *employeeRepository) ListDocuments(ctx context.Context, employeeID uuid.UUID) ([]entity.EmployeeDocument, error) {
	var rows []entity.EmployeeDocument
	err := r.db.WithContext(ctx).
		Where("employee_id = ?", employeeID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

/*
DeleteDocument is keyed on both ids, so a document id belonging to another
employee cannot be deleted by guessing it.
*/
func (r *employeeRepository) DeleteDocument(ctx context.Context, employeeID, documentID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("employee_id = ? AND id = ?", employeeID, documentID).
		Delete(&entity.EmployeeDocument{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

/*
conn returns the caller's transaction when there is one and the pooled handle
otherwise — the same pattern orderRepository uses, so an employee and the user
account created alongside it commit or roll back together.
*/
func (r *employeeRepository) conn(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
