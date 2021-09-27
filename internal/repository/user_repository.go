package repository

import (
	"context"
	"errors"
	"github.com/jmoiron/sqlx"
	"mygame/internal/models"
)

type User struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *User {
	return &User{
		db: db,
	}
}

func (u *User) IsExistByLogin(ctx context.Context, login string) bool {
	var id uint64

	err := u.db.GetContext(ctx, &id, "SELECT id FROM users WHERE login = $1", login)
	if err != nil {
		return false
	}

	if id == 0 {
		return false
	}

	return true
}

func (u *User) GetUserByCredentials(ctx context.Context, credentials *models.Credentials) (uint64, error) {
	var id uint64

	err := u.db.GetContext(ctx, &id, "SELECT id FROM users WHERE login = $1 AND password = $2", credentials.Login, credentials.Password)
	if err != nil {
		return 0, errors.New("login or password incorrect")
	}

	if id == 0 {
		return 0, errors.New("user not found")
	}

	return id, nil
}

func (u *User) GetUserByUserID(ctx context.Context, userID uint64) (*models.User, error) {
	var user models.User

	err := u.db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = $1", userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	return &user, nil
}

func (u *User) CreateUser(ctx context.Context, user *models.User) (uint64, error) {
	var id uint64

	err := u.db.QueryRowContext(ctx, "INSERT INTO users (login, password, photo) VALUES ($1,$2,$3) RETURNING id",
		user.Login,
		user.Password,
		user.Photo,
	).Scan(&id)
	if err != nil {
		return 0, errors.New("user creation error")
	}

	return id, nil
}

func (u *User) GetUserIDByLogin(ctx context.Context, login string) (uint64, error) {
	var id uint64

	err := u.db.GetContext(ctx, &id, "SELECT id FROM users WHERE login = $1", login)
	if err != nil {
		return 0, err
	}

	if id == 0 {
		return 0, errors.New("user not found")
	}

	return id, nil
}
