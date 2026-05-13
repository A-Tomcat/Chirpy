package auth

import (
	"fmt"
	"net/http"
	"strings"
)

func GetAPIKey(headers http.Header) (string, error) {
	unclean := headers.Get("Authorization")
	if unclean == "" {
		return "", fmt.Errorf("No Authorization header exists.")
	}
	tokenString := strings.TrimPrefix(unclean, "ApiKey ")

	return tokenString, nil
}
