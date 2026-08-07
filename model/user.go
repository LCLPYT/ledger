package model

import (
	"time"

	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID           int64        `bun:"id,pk,autoincrement"`
	Name         string       `bun:"name,notnull,type:text"`
	Email        string       `bun:"email,unique,notnull,type:text"`
	CreatedAt    time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt    time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	DeletedAt    bun.NullTime `bun:"deleted_at,nullzero,soft_delete"`
	PasswordHash []byte       `bun:"password_hash,notnull"`
}
