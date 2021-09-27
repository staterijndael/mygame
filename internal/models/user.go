package models

import "errors"

type User struct {
	Login    string `json:"login"    db:"login"`
	Password string `json:"password" db:"password"`
	Photo    string `json:"photo"    db:"photo"`
}

func (u *User) Validate() error {
	if len(u.Login) < 3 {
		return errors.New("логин не может быть меньше 3 символов")
	}
	if len(u.Login) > 32 {
		return errors.New("логин не может быть больше 32 символов")
	}
	if len(u.Password) > 64 {
		return errors.New("пароль не может быть больше 64 символов")
	}
	if len(u.Password) < 8 {
		return errors.New("пароль не может быть меньше 8 символов")
	}

	return nil
}
