package auth

import (
	"crypto/rand"
	"encoding/hex"
)

func MakeRefreshToken() string {
	random := make([]byte, 32)
	rand.Read(random)
	stringToken := hex.EncodeToString(random)
	return stringToken
}
