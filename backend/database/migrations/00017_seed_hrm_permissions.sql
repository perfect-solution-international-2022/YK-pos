-- Adds the two HRM permissions and grants them.
--
-- Only two, deliberately: hrm.view and hrm.manage are exactly the names already
-- present in the frontend's PERMISSIONS list
-- (pos-frontend/src/lib/types/index.ts), and that list is the contract. A finer
-- split (hrm.payroll, hrm.leave_approve) would be unreachable in the UI until
-- both sides ship it, so the endpoints gate on these two: reads on hrm.view,
-- every write — employees, payroll, leave decisions, settings — on hrm.manage.
--
-- Follows 00015's shape: idempotent inserts, grants resolved by join on the
-- generated permissions.name rather than hardcoded UUIDs, and an explicit grant
-- to owner because 00015's CROSS JOIN only covered the rows that existed then.

-- +goose Up
INSERT INTO permissions (id, resource, action, description)
VALUES
    (gen_random_uuid(), 'hrm', 'view',   'View employees, attendance, leave, payroll, and HR reports'),
    (gen_random_uuid(), 'hrm', 'manage', 'Manage employees, attendance, leave decisions, payroll, and HR settings')
ON CONFLICT (resource, action) DO NOTHING;

-- +goose StatementBegin
WITH grants (role_name, permission_name) AS (
    VALUES
        -- Owner is covered by the CROSS JOIN below.
        ('manager', 'hrm.view'),
        ('manager', 'hrm.manage')
        -- Cashier and warehouse staff get neither: an HR file holds salary,
        -- bank details and a NIC, which is not till-side data.
)
INSERT INTO role_permissions (id, role_id, permission_id, effect)
SELECT gen_random_uuid(), r.id, p.id, 'allow'
FROM grants g
JOIN roles r ON r.name = g.role_name AND r.business_id IS NULL AND r.is_system
JOIN permissions p ON p.name = g.permission_name
ON CONFLICT (role_id, permission_id) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO role_permissions (id, role_id, permission_id, effect)
SELECT gen_random_uuid(), r.id, p.id, 'allow'
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'owner'
  AND r.business_id IS NULL
  AND r.is_system
  AND p.resource = 'hrm'
ON CONFLICT (role_id, permission_id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE resource = 'hrm');

DELETE FROM permissions WHERE resource = 'hrm';
