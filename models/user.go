package models

import "time"

type User struct {
	ID       int64     `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Created  time.Time `json:"created"`
	Password string    `json:"-"`
}

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type TokenRequest struct {
	Name   string    `json:"name"`
	Scopes []string  `json:"scopes"`
	Expiry time.Time `json:"expiry"`
}

type AccessToken struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
	Scopes    []string  `json:"scopes"`
}
