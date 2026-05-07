package auth_test

import (
	"fmt"
	"main/internal/auth"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAddJWT(t *testing.T) {
	tokenSecret := "atomcat"
	userID := uuid.New()
	expi := time.Duration(10 * time.Second)
	t.Run(fmt.Sprintln("Testing MakeJWT"), func(t *testing.T) {
		_, err := auth.MakeJWT(userID, tokenSecret, expi)
		if err != nil {
			t.Errorf("Error: %v\n", err)
			return
		}

	})
}
func testValidateJWT(t *testing.T) {
	tokenSecret := "atomcat"
	userID := uuid.New()
	expi := time.Duration(10 * time.Second)
	token, err := auth.MakeJWT(userID, tokenSecret, expi)
	if err != nil {
		fmt.Printf("%v\n", err.Error())
		t.Fail()
		return
	}
	t.Run(fmt.Sprintln("Testing JWT"), func(t *testing.T) {
		Uid, err := auth.ValidateJWT(token, tokenSecret)
		if err != nil {
			t.Errorf("Error: %v\n", err)
			t.Fail()
			return
		}
		if Uid != userID {
			t.Errorf("UserID and validated ID do not match")
			t.Fail()
			return
		}
	})

}
