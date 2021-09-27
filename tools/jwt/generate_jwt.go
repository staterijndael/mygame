package jwt

import (
	"context"
	"errors"
	"github.com/dgrijalva/jwt-go"
	"mygame/internal/models"
	"mygame/tools/helpers"
	"time"
)

func GenerateTokens(ctx context.Context, userID uint64, login string, jwtSecretKey string, expirationTime time.Duration) (*models.Token, error) {
	refreshToken := helpers.GenerateRandomString(64)
	accessToken, err := CreateJWT([]byte(jwtSecretKey), &Claims{
		ID:    userID,
		Login: login,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(expirationTime).Unix(),
		},
	})
	if err != nil {
		return nil, errors.New("token creation error")
	}

	token := &models.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       userID,
		Exp:          time.Now().Add(expirationTime),
	}

	return token, nil
}
