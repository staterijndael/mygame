package models

import "errors"

type Credentials struct {
	Login    string `json:"login"    db:"login"`
	Password string `json:"password" db:"password"`
}

func (c *Credentials) Validate() error {
	if len(c.Login) < 3 {
		return errors.New("логин не может быть меньше 3 символов")
	}
	if len(c.Login) > 32 {
		return errors.New("логин не может быть больше 32 символов")
	}
	if len(c.Password) > 64 {
		return errors.New("пароль не может быть больше 64 символов")
	}
	if len(c.Password) < 8 {
		return errors.New("пароль не может быть меньше 8 символов")
	}

	return nil
}
