package models

import "time"

type MinecraftPlayer struct {
	ID        int64     `json:"id"`
	UUID      string    `json:"uuid"`
	Username  *string   `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type CoinTransaction struct {
	ID           int64     `json:"id"`
	PlayerID     int64     `json:"player_id"`
	Amount       int64     `json:"amount"`
	Source       string    `json:"source"`
	Description  *string   `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
	ActorUserID  *int64    `json:"actor_user_id"`
	ActorTokenID *int64    `json:"actor_token_id"`
}

type AwardCoinsRequest struct {
	Amount      int64   `json:"amount"      binding:"required,min=1"`
	Source      string  `json:"source"      binding:"required,oneof=minigame admin purchase system"`
	Description *string `json:"description"`
}

type SpendCoinsRequest struct {
	Amount      int64   `json:"amount"      binding:"required,min=1"`
	Source      string  `json:"source"      binding:"required,oneof=minigame admin purchase system"`
	Description *string `json:"description"`
}

type AdjustCoinsRequest struct {
	Amount      int64   `json:"amount"      binding:"required"`
	Source      string  `json:"source"      binding:"required,oneof=minigame admin purchase system"`
	Description *string `json:"description"`
}
