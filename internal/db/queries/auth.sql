-- ApiKeyPermissions
SELECT DISTINCT rp.permission
FROM api_key_roles kr
JOIN role_permissions rp ON rp.role_name = kr.role_name
WHERE kr.api_key_id = $1;

-- ApiKeyScopes
SELECT permission
FROM api_key_scopes
WHERE api_key_id = $1;
