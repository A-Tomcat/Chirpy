package auth

import (
	"github.com/alexedwards/argon2id"
)

func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return hash, err
	}

	return hash, nil
}

func CheckPasswordHash(password string, hash string) (bool, error) {
	if v, err := argon2id.ComparePasswordAndHash(password, hash); err != nil {
		return false, err
	} else {
		return v, nil
	}
}
