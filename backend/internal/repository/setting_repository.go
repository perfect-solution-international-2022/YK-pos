package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

//go:generate go run go.uber.org/mock/mockgen -source=setting_repository.go -destination=mocks/setting_repository_mock.go -package=mocks

/*
SettingRepository is a key/value store over the existing settings table, scoped
to a business.

It is generic on purpose: HRM's three settings groups (attendance, leave,
payroll) are JSON documents under reserved keys rather than three bespoke
tables, because they are read whole, written whole, and never queried by their
contents. A table per group would add three migrations and three repositories
to store what is, on every read path, one blob.
*/
type SettingRepository interface {
	// Get returns the raw JSON value for a key. ErrNotFound when the business
	// has never saved one, which callers turn into the module's defaults rather
	// than an error.
	Get(ctx context.Context, businessID uuid.UUID, key string) (*entity.Setting, error)
	// GetMany returns every stored setting whose key is in keys, in one query,
	// so a settings screen showing three groups costs one round trip.
	GetMany(ctx context.Context, businessID uuid.UUID, keys []string) ([]entity.Setting, error)
	// Put inserts or replaces the value for a key.
	Put(ctx context.Context, businessID uuid.UUID, key string, value datatypes.JSON) error
}

type settingRepository struct {
	db *gorm.DB
}

func NewSettingRepository(db *gorm.DB) SettingRepository {
	return &settingRepository{db: db}
}

func (r *settingRepository) Get(ctx context.Context, businessID uuid.UUID, key string) (*entity.Setting, error) {
	var row entity.Setting
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND branch_id IS NULL AND key = ?", businessID, key).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &row, nil
}

func (r *settingRepository) GetMany(
	ctx context.Context, businessID uuid.UUID, keys []string,
) ([]entity.Setting, error) {
	if len(keys) == 0 {
		return []entity.Setting{}, nil
	}
	var rows []entity.Setting
	err := r.db.WithContext(ctx).
		Where("business_id = ? AND branch_id IS NULL AND key IN ?", businessID, keys).
		Find(&rows).Error
	return rows, err
}

/*
Put upserts the value for a key.

An upsert rather than a read-then-write: two managers saving the settings screen
at the same moment would both miss the other's row on a preceding SELECT, and
one insert would then fail on the unique index.

The conflict target is spelled as the same COALESCE expression the index is
built over (settings_scope_key_key), which is why this is raw SQL: GORM's
clause.OnConflict can only name bare columns, and naming business_id/branch_id/
key directly would not match an expression index. The id is minted here for the
same reason — a raw INSERT skips the BeforeCreate hook that normally assigns it.

Branch-scoped settings are not written here: HRM settings are business-wide
policy, and a branch override would need a resolution order nothing asks for
yet.
*/
func (r *settingRepository) Put(
	ctx context.Context, businessID uuid.UUID, key string, value datatypes.JSON,
) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO settings (id, business_id, branch_id, key, value)
		 VALUES (?, ?, NULL, ?, ?)
		 ON CONFLICT (
		     COALESCE(business_id, '00000000-0000-0000-0000-000000000000'::uuid),
		     COALESCE(branch_id, '00000000-0000-0000-0000-000000000000'::uuid),
		     key
		 )
		 DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		id, businessID, key, value,
	).Error
}
