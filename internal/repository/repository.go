package repository

import (
	"context"
	"github.com/jmoiron/sqlx"
	"mygame/internal/models"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *models.User) (uint64, error)
	IsExistByLogin(ctx context.Context, login string) bool
	GetUserByCredentials(ctx context.Context, credentials *models.Credentials) (uint64, error)
	GetUserByUserID(ctx context.Context, userID uint64) (*models.User, error)
	GetUserIDByLogin(ctx context.Context, login string) (uint64, error)
}

type TokenRepository interface {
	CreateToken(ctx context.Context, token *models.Token) error
	GetTokenByAccessToken(ctx context.Context, accessToken string) (*models.Token, error)
	DeleteTokenByAccessToken(ctx context.Context, accessToken string) error
}

type Repository struct {
	UserRepository  UserRepository
	TokenRepository TokenRepository
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		UserRepository:  NewUserRepository(db),
		TokenRepository: NewTokenRepository(db),
	}
}
