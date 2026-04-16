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
	PlayerRead       = "minecraft.player_read"
	PlayerWrite      = "minecraft.player_write"
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
	PlayerRead,
	PlayerWrite,
}

var DefaultPermissions = []string{
	UserRead,
	UserCreateToken,
}
