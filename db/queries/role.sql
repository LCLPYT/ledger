-- name: ListRoles :many
SELECT id, name, created_at, protected FROM roles ORDER BY name;

-- name: CreateRole :one
INSERT INTO roles (name) VALUES ($1) RETURNING id, name, created_at, protected;

-- name: GetRoleProtected :one
SELECT protected FROM roles WHERE name = $1;

-- name: DeleteRole :exec
DELETE FROM roles WHERE name = $1;

-- name: RoleExists :one
SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1);

-- name: InitRole :exec
INSERT INTO roles (name, protected) VALUES ($1, TRUE) ON CONFLICT DO NOTHING;
