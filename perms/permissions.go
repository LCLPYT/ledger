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
	PlayerDataRead   = "minecraft.player_data_read"
	PlayerDataWrite  = "minecraft.player_data_write"
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
	PlayerDataRead,
	PlayerDataWrite,
}

var DefaultPermissions = []string{
	UserRead,
	UserCreateToken,
}
