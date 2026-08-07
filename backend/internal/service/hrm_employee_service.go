package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/hash"
)

/*
EmployeeLogin is the optional account an employee is hired with.

The system's login identifier is the email address, as it is everywhere else in
this API — there is no separate username column on users, and inventing one here
would give the HR module a second, divergent way to sign in. Email is therefore
required whenever a login is requested.
*/
type EmployeeLogin struct {
	Email    string
	Password string
	RoleIDs  []uuid.UUID
}

/*
EmployeeTxRepos are the repositories a hire writes through, bound to one
transaction. Mirrors AuthService's TxRepos: creating an employee and the user
account that comes with them is a single atomic act, so a failure halfway must
not leave a login nobody owns.
*/
type EmployeeTxRepos struct {
	Users     repository.UserRepository
	Employees repository.EmployeeRepository
}

// DefaultEmployeeTxRepos is the production EmployeeServiceDeps.NewTxRepos.
func DefaultEmployeeTxRepos(tx *gorm.DB) EmployeeTxRepos {
	return EmployeeTxRepos{
		Users:     repository.NewUserRepository(tx),
		Employees: repository.NewEmployeeRepository(tx),
	}
}

type EmployeeServiceDeps struct {
	Employees    repository.EmployeeRepository
	Designations repository.DesignationRepository
	Shifts       repository.ShiftRepository
	Users        repository.UserRepository
	Roles        repository.RoleRepository
	Tx           *repository.TxManager
	NewTxRepos   func(*gorm.DB) EmployeeTxRepos
}

/*
EmployeeService owns staff records.

Two things it deliberately does not do. It never deletes the linked user account
when an employee is removed — losing an HR file must not silently lock somebody
out of the till, which stays an explicit action on the users endpoints. And it
never changes an existing account's password: that goes through the auth flow,
which knows about the token denylist and the audit trail.
*/
type EmployeeService interface {
	List(ctx context.Context, businessID uuid.UUID, query EmployeeQuery) ([]entity.Employee, int64, error)
	Get(ctx context.Context, businessID, id uuid.UUID) (*entity.Employee, error)
	// GetByUserID resolves the employee behind the authenticated user, for the
	// self-service paths where the caller is the employee.
	GetByUserID(ctx context.Context, businessID, userID uuid.UUID) (*entity.Employee, error)
	Create(ctx context.Context, businessID uuid.UUID, e *entity.Employee, login *EmployeeLogin) (*entity.Employee, error)
	Update(ctx context.Context, businessID, id uuid.UUID, apply func(*entity.Employee)) (*entity.Employee, error)
	Delete(ctx context.Context, businessID, id uuid.UUID) error

	AddDocument(ctx context.Context, businessID, employeeID uuid.UUID, doc *entity.EmployeeDocument) (*entity.EmployeeDocument, error)
	ListDocuments(ctx context.Context, businessID, employeeID uuid.UUID) ([]entity.EmployeeDocument, error)
	DeleteDocument(ctx context.Context, businessID, employeeID, documentID uuid.UUID) error
}

type employeeService struct {
	deps       EmployeeServiceDeps
	bcryptCost int
}

func NewEmployeeService(deps EmployeeServiceDeps, bcryptCost int) EmployeeService {
	return &employeeService{deps: deps, bcryptCost: bcryptCost}
}

func (s *employeeService) List(
	ctx context.Context, businessID uuid.UUID, query EmployeeQuery,
) ([]entity.Employee, int64, error) {
	rows, total, err := s.deps.Employees.List(ctx, businessID, query.toParams())
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "failed to list employees", err)
	}
	return rows, total, nil
}

func (s *employeeService) Get(ctx context.Context, businessID, id uuid.UUID) (*entity.Employee, error) {
	row, err := s.deps.Employees.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "employee not found", "failed to load employee")
	}
	return row, nil
}

func (s *employeeService) GetByUserID(ctx context.Context, businessID, userID uuid.UUID) (*entity.Employee, error) {
	row, err := s.deps.Employees.FindByUserID(ctx, businessID, userID)
	if err != nil {
		return nil, mapNotFound(err, "no employee record is linked to this account", "failed to load employee")
	}
	return row, nil
}

/*
Create hires an employee, optionally with a login.

Everything happens in one transaction: the designation and shift checks, the
duplicate checks, the user account, the code allocation and the insert. A hire
that fails at the last step must not leave behind a user account with a
password somebody has already been told.
*/
func (s *employeeService) Create(
	ctx context.Context, businessID uuid.UUID, e *entity.Employee, login *EmployeeLogin,
) (*entity.Employee, error) {
	e.BusinessID = businessID
	normalizeEmployee(e)

	if err := s.assertReferencesValid(ctx, businessID, e); err != nil {
		return nil, err
	}
	if err := s.assertIdentifiersFree(ctx, businessID, e, uuid.Nil); err != nil {
		return nil, err
	}
	if err := s.validateLogin(ctx, businessID, e, login); err != nil {
		return nil, err
	}

	err := s.deps.Tx.WithTransaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
		repos := s.deps.NewTxRepos(tx)

		if login != nil {
			user, createErr := s.createLoginUser(ctx, repos, businessID, e.FullName, login)
			if createErr != nil {
				return createErr
			}
			e.UserID = &user.ID
		}

		code, codeErr := repos.Employees.NextEmployeeCode(ctx, tx, businessID)
		if codeErr != nil {
			return apperror.Wrap(apperror.CodeDatabase, "failed to allocate an employee code", codeErr)
		}
		e.EmployeeCode = code

		if createErr := repos.Employees.Create(ctx, tx, e); createErr != nil {
			return apperror.Wrap(apperror.CodeDatabase, "failed to create employee", createErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Re-read so the response carries the preloaded designation and shift the
	// list and detail screens render, rather than bare ids.
	return s.Get(ctx, businessID, e.ID)
}

/*
Update replaces an employee's editable fields.

The stored row is loaded first and the edit applied to it, so employee_code,
user_id and business_id — none of which the request can carry — survive
untouched, and the response is the server's copy rather than the client's.
*/
func (s *employeeService) Update(
	ctx context.Context, businessID, id uuid.UUID, apply func(*entity.Employee),
) (*entity.Employee, error) {
	row, err := s.deps.Employees.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "employee not found", "failed to load employee")
	}

	apply(row)
	normalizeEmployee(row)

	if err := s.assertReferencesValid(ctx, businessID, row); err != nil {
		return nil, err
	}
	if err := s.assertIdentifiersFree(ctx, businessID, row, row.ID); err != nil {
		return nil, err
	}
	if err := s.deps.Employees.Update(ctx, row); err != nil {
		return nil, mapNotFound(err, "employee not found", "failed to update employee")
	}

	return s.Get(ctx, businessID, id)
}

/*
Delete soft-deletes the HR record and leaves the login alone.

Payslips reference the employee, so the row has to survive for a historical
payslip to still name who it paid — and a person's till account is a separate
decision from their HR file, made on the users endpoints.
*/
func (s *employeeService) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	if err := s.deps.Employees.Delete(ctx, businessID, id); err != nil {
		return mapNotFound(err, "employee not found", "failed to delete employee")
	}
	return nil
}

func (s *employeeService) AddDocument(
	ctx context.Context, businessID, employeeID uuid.UUID, doc *entity.EmployeeDocument,
) (*entity.EmployeeDocument, error) {
	// Scoped read first: a document must not be attachable to another tenant's
	// employee by guessing an id.
	if _, err := s.deps.Employees.FindByID(ctx, businessID, employeeID); err != nil {
		return nil, mapNotFound(err, "employee not found", "failed to load employee")
	}

	doc.EmployeeID = employeeID
	if err := s.deps.Employees.AddDocument(ctx, doc); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to store document", err)
	}
	return doc, nil
}

func (s *employeeService) ListDocuments(
	ctx context.Context, businessID, employeeID uuid.UUID,
) ([]entity.EmployeeDocument, error) {
	if _, err := s.deps.Employees.FindByID(ctx, businessID, employeeID); err != nil {
		return nil, mapNotFound(err, "employee not found", "failed to load employee")
	}

	docs, err := s.deps.Employees.ListDocuments(ctx, employeeID)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to load documents", err)
	}
	return docs, nil
}

func (s *employeeService) DeleteDocument(
	ctx context.Context, businessID, employeeID, documentID uuid.UUID,
) error {
	if _, err := s.deps.Employees.FindByID(ctx, businessID, employeeID); err != nil {
		return mapNotFound(err, "employee not found", "failed to load employee")
	}

	if err := s.deps.Employees.DeleteDocument(ctx, employeeID, documentID); err != nil {
		return mapNotFound(err, "document not found", "failed to delete document")
	}
	return nil
}

/*
createLoginUser provisions the account an employee signs in with, reusing the
same repositories and the same bcrypt cost the auth module uses — the HR module
has no password handling of its own, and should not grow one.
*/
func (s *employeeService) createLoginUser(
	ctx context.Context, repos EmployeeTxRepos, businessID uuid.UUID,
	fullName string, login *EmployeeLogin,
) (*entity.User, error) {
	hashed, err := hash.Hash(login.Password, s.bcryptCost)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to hash password", err)
	}

	user := entity.User{
		BusinessID:   businessID,
		Email:        login.Email,
		PasswordHash: hashed,
		FullName:     fullName,
		Status:       entity.UserStatusActive,
	}
	if err := repos.Users.Create(ctx, &user); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to create the login account", err)
	}
	if len(login.RoleIDs) > 0 {
		if err := repos.Users.AssignRoles(ctx, user.ID, login.RoleIDs, nil); err != nil {
			return nil, apperror.Wrap(apperror.CodeDatabase, "failed to assign roles", err)
		}
	}

	return &user, nil
}

/*
validateLogin checks everything about a requested account before the transaction
opens, so the common failures (a taken email, an unknown role) come back as a
clean 409/404 rather than as a rolled-back transaction.
*/
func (s *employeeService) validateLogin(
	ctx context.Context, businessID uuid.UUID, e *entity.Employee, login *EmployeeLogin,
) error {
	if login == nil {
		return nil
	}

	login.Email = strings.ToLower(strings.TrimSpace(login.Email))
	if login.Email == "" {
		// Fall back to the employee's own address: the two are the same person,
		// and requiring it twice on the form would be noise.
		if e.Email == nil || *e.Email == "" {
			return apperror.New(
				apperror.CodeValidationError,
				"an email address is required to create a login",
			)
		}
		login.Email = *e.Email
	}

	if _, err := s.deps.Users.FindByEmail(ctx, businessID, login.Email); err == nil {
		return apperror.New(apperror.CodeEmailAlreadyExists, "a user with this email already exists")
	} else if !errors.Is(err, repository.ErrNotFound) {
		return apperror.Wrap(apperror.CodeDatabase, "failed to check existing email", err)
	}

	for _, roleID := range login.RoleIDs {
		role, err := s.deps.Roles.FindByID(ctx, roleID)
		if err != nil {
			return mapNotFound(err, "role not found", "failed to validate role")
		}
		// A business-scoped role from another tenant reads as not found, the
		// same as one that does not exist. System roles (business_id NULL) are
		// available to everyone.
		if role.BusinessID != nil && *role.BusinessID != businessID {
			return apperror.New(apperror.CodeNotFound, "role not found")
		}
	}

	return nil
}

/*
assertReferencesValid checks that the designation and shift belong to this
business and are still in use. Left to the foreign keys, a title from another
tenant would be rejected as a driver error nothing above could explain, and an
inactive one would be accepted silently.
*/
func (s *employeeService) assertReferencesValid(
	ctx context.Context, businessID uuid.UUID, e *entity.Employee,
) error {
	designation, err := s.deps.Designations.FindByID(ctx, businessID, e.DesignationID)
	if err != nil {
		return mapNotFound(err, "designation not found", "failed to load designation")
	}
	if designation.Status != entity.HRMStatusActive {
		return apperror.New(apperror.CodeValidationError, "this designation is no longer active")
	}

	if e.ShiftID == nil {
		return nil
	}
	shift, err := s.deps.Shifts.FindByID(ctx, businessID, *e.ShiftID)
	if err != nil {
		return mapNotFound(err, "shift not found", "failed to load shift")
	}
	if shift.Status != entity.HRMStatusActive {
		return apperror.New(apperror.CodeValidationError, "this shift is no longer active")
	}
	return nil
}

/*
assertIdentifiersFree rejects an NIC or email already held by somebody else in
the business. Matching this employee's own id is expected — an edit that leaves
the identifiers alone finds itself — which is why selfID is passed in rather
than inferred.
*/
func (s *employeeService) assertIdentifiersFree(
	ctx context.Context, businessID uuid.UUID, e *entity.Employee, selfID uuid.UUID,
) error {
	existing, err := s.deps.Employees.FindByNIC(ctx, businessID, e.NIC)
	switch {
	case errors.Is(err, repository.ErrNotFound):
	case err != nil:
		return apperror.Wrap(apperror.CodeDatabase, "failed to check existing employee", err)
	case existing.ID != selfID:
		return apperror.New(apperror.CodeConflict, "an employee with this NIC already exists")
	}

	if e.Email == nil || *e.Email == "" {
		return nil
	}
	existing, err = s.deps.Employees.FindByEmail(ctx, businessID, *e.Email)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return nil
	case err != nil:
		return apperror.Wrap(apperror.CodeDatabase, "failed to check existing employee", err)
	case existing.ID != selfID:
		return apperror.New(apperror.CodeConflict, "an employee with this email already exists")
	}
	return nil
}

/*
normalizeEmployee applies the storage form of the free-text identifiers, so the
case-insensitive unique indexes and the duplicate checks above agree with each
other. An empty optional email becomes nil rather than "", because the partial
unique index only skips NULLs — two employees with "" would collide.
*/
func normalizeEmployee(e *entity.Employee) {
	e.FullName = strings.TrimSpace(e.FullName)
	e.NIC = strings.ToUpper(strings.TrimSpace(e.NIC))
	e.Phone = strings.TrimSpace(e.Phone)

	if e.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*e.Email))
		if email == "" {
			e.Email = nil
		} else {
			e.Email = &email
		}
	}

	if e.Status == "" {
		e.Status = entity.EmployeeStatusActive
	}
	if e.Gender == "" {
		e.Gender = entity.GenderOther
	}
	if e.EmploymentType == "" {
		e.EmploymentType = entity.EmploymentTypeFullTime
	}
}
