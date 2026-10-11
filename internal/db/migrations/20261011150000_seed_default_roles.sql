-- +goose Up
INSERT INTO roles (name) VALUES ('admin'), ('user') ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_name, permission) VALUES ('admin', '*') ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM roles WHERE name IN ('admin', 'user');
