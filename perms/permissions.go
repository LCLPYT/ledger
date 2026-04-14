package perms

const (
	UserRead         = "user.read"
	UserCreateToken  = "user.create_token"
	UsersRead        = "users.read"
	UsersCreate      = "users.create"
	RolesCreate      = "roles.create"
	RolesRead        = "roles.read"
	RolesManageUsers = "roles.manage_users"
	CoinsRead        = "coins.read"
	CoinsWrite       = "coins.write"
)

var All = []string{
	UserRead,
	UserCreateToken,
	UsersRead,
	UsersCreate,
	RolesCreate,
	RolesRead,
	RolesManageUsers,
	CoinsRead,
	CoinsWrite,
}

var DefaultPermissions = []string{
	UserRead,
	UserCreateToken,
}
