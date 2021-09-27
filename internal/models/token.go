package models

import "time"

type Token struct {
	ID           uint64    `json:"id"            db:"id"`
	AccessToken  string    `json:"access_token"  db:"access_token"`
	RefreshToken string    `json:"refresh_token" db:"refresh_token"`
	Login        string    `json:"login"         db:"login"`
	Exp          time.Time `json:"exp"           db:"exp"`
	UserID       uint64    `json:"user_id"       db:"user_id"`
}
