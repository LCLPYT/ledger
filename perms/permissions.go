package perms

const (
	UserRead         = "user.read"
	UserCreateToken  = "user.create_token"
	RolesCreate      = "roles.create"
	RolesRead        = "roles.read"
	RolesManageUsers = "roles.manage_users"
	PermissionsList  = "permissions.list"
)

var All = []string{
	UserRead,
	UserCreateToken,
	RolesCreate,
	RolesRead,
	RolesManageUsers,
	PermissionsList,
}
