package auth_test

import (
	"fmt"
	"main/internal/auth"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMain(t *testing.T) {

	tokenSecret := "secret"
	userID := uuid.New()
	expi := 10 * time.Second
	tests := []struct {
		name    string
		tokenFn func() string
		secret  string
		wantErr bool
	}{
		{"valid token", func() string {
			token, _ := auth.MakeJWT(userID, tokenSecret, expi)
			return token
		}, "secret", false},
		{"expired token", func() string {
			token, _ := auth.MakeJWT(userID, tokenSecret, -expi)
			return token
		}, "secret", true},
		{"wrong secret", func() string {
			token, _ := auth.MakeJWT(userID, tokenSecret, expi)
			return token
		}, "other", true},
		{"malformed token", func() string {
			return "blurb"
		}, "secret", true},
		{"get token", func() string {
			token, _ := auth.MakeJWT(userID, tokenSecret, expi)
			head := http.Header{}
			head.Add("Authorization", fmt.Sprintf("Bearer %v", token))
			token, _ = auth.GetBearerToken(head)
			return token
		}, "secret", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := auth.ValidateJWT(tc.tokenFn(), tc.secret)
			if (err != nil) != tc.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

/*
func TestAddJWT(t *testing.T) {
	tokenSecret := "atomcat"
	userID := uuid.New()
	expi := 10 * time.Second
	t.Run("Testing MakeJWT", func(t *testing.T) {
		_, err := auth.MakeJWT(userID, tokenSecret, expi)
		if err != nil {
			t.Errorf("Error: %v\n", err)
			return
		}

	})
}
func TestValidateJWT(t *testing.T) {
	tokenSecret := "atomcat"
	userID := uuid.New()
	expi := 10 * time.Second
	token, err := auth.MakeJWT(userID, tokenSecret, expi)
	if err != nil {
		fmt.Printf("%v\n", err.Error())
		return
	}
	t.Run("Testing JWT", func(t *testing.T) {
		Uid, err := auth.ValidateJWT(token, tokenSecret)
		if err != nil {
			t.Errorf("Error: %v\n", err)
			return
		}
		if Uid != userID {
			t.Errorf("UserID and validated ID do not match")
			return
		}
	})

}
*/

/*
Ideas for more tests

The lesson explicitly asks for these scenarios. You've got the happy path — now think about the unhappy ones:
1. Expired token

Create a JWT with a negative or very tiny expiration (like -time.Hour or 1 * time.Nanosecond followed by a brief time.Sleep). Then call ValidateJWT and assert that it returns an error.

expired, _ := auth.MakeJWT(userID, secret, -time.Hour)
_, err := auth.ValidateJWT(expired, secret)
// expect err != nil

2. Wrong secret

Sign with one secret, validate with another. Should fail.

token, _ := auth.MakeJWT(userID, "secret-a", time.Hour)
_, err := auth.ValidateJWT(token, "secret-b")
// expect err != nil

3. Malformed token

Pass garbage like "not.a.real.token" to ValidateJWT. Should fail gracefully.
4. Table-driven tests (bonus)

Once you have several cases, consider consolidating them into a table. It's a very idiomatic Go pattern:

	tests := []struct {
		name      string
		tokenFn   func() string
		secret    string
		wantErr   bool
	}{
		{"valid token", func() string { /* ... * / }, "secret", false},
		{"expired token", func() string { /* ...  * / }, "secret", true},
		{"wrong secret", func() string { /* ... * / }, "other", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := auth.ValidateJWT(tc.tokenFn(), tc.secret)
			if (err != nil) != tc.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}

This scales beautifully when you have many cases.
*/
