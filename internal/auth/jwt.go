package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeJWT(user uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		Subject:   user.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
	})
	signedToken, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func ValidateJWT(tokenString string, tokenSecret string) (uuid.UUID, error) {
	claim := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claim, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	sid, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, err
	}
	uid, err := uuid.Parse(sid)
	if err != nil {
		return uuid.Nil, err
	}
	return uid, nil
}
