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

type Repository struct {
	UserRepository UserRepository
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		UserRepository: NewUserRepository(db),
	}
}
