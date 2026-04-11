package models

import "time"

type Role struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type RoleWithMembers struct {
	Role
	Members []string `json:"members"`
}

type CreateRoleRequest struct {
	Name string `json:"name" binding:"required"`
}

type RoleUserRequest struct {
	UserID string `json:"user_id" binding:"required"`
}
