package permissions

const (
	UserRead         = "user.read"
	UserCreateToken  = "user.create_token"
	RolesCreate      = "roles.create"
	RolesRead        = "roles.read"
	RolesManageUsers = "roles.manage_users"
)

var All = []string{
	UserRead,
	UserCreateToken,
	RolesCreate,
	RolesRead,
	RolesManageUsers,
}
