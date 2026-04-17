-- name: InsertInvitation :exec
INSERT INTO user_invitations (user_id, token, expires_at)
VALUES ($1, $2, now() + interval '24 hours');

-- name: GetValidInvitation :one
SELECT id, user_id FROM user_invitations
WHERE token = $1 AND expires_at > now() AND used_at IS NULL;

-- name: VerifyUser :exec
UPDATE users SET password_hash = $1, verified_at = now() WHERE id = $2;

-- name: MarkInvitationUsed :exec
UPDATE user_invitations SET used_at = now() WHERE id = $1;
