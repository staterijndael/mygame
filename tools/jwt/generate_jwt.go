package jwt

import (
	"context"
	"errors"
	"github.com/dgrijalva/jwt-go"
	"time"
)

func GenerateTokens(ctx context.Context, userID uint64, login string, jwtSecretKey string, expirationTime time.Duration) (string, error) {
	accessToken, err := CreateJWT([]byte(jwtSecretKey), &Claims{
		ID:    userID,
		Login: login,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(expirationTime).Unix(),
		},
	})
	if err != nil {
		return "", errors.New("token creation error")
	}

	return accessToken, nil
}
