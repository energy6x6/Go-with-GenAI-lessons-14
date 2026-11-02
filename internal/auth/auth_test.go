package auth_test

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"libratrack/internal/auth"
)

func TestPasswordHashesAreSalted(t *testing.T) {
	a, err := auth.HashPassword("StrongPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	b, err := auth.HashPassword("StrongPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("equal passwords must have different salts")
	}
	if err := auth.CheckPassword(a, "StrongPassword123!"); err != nil {
		t.Fatal(err)
	}
	if err := auth.CheckPassword(a, "WrongPassword123!"); err == nil {
		t.Fatal("wrong password accepted")
	}
	cost, err := bcrypt.Cost([]byte(a))
	if err != nil || cost != 10 {
		t.Fatal("expected bcrypt cost 10")
	}
}

func TestSecretConfiguration(t *testing.T) {
	for _, secret := range []string{"", "weak", strings.Repeat("x", 31)} {
		t.Run("reject_"+string(rune(len(secret)+65)), func(t *testing.T) {
			t.Setenv("JWT_SECRET", secret)
			if _, err := auth.NewTokensFromEnv(); err == nil {
				t.Fatal("missing or short secret accepted")
			}
		})
	}
	t.Setenv("JWT_SECRET", strings.Repeat("x", 32))
	tokens, err := auth.NewTokensFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Issue(0, "reader"); err == nil {
		t.Fatal("zero user ID accepted")
	}
	if _, err := tokens.Issue(1, "boss"); err == nil {
		t.Fatal("unknown role accepted")
	}
}
