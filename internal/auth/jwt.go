package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"libratrack/internal/model"
)

type Claims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type Tokens struct{ secret []byte }

// NewTokensFromEnv validates configuration once. There is no fallback key.
func NewTokensFromEnv() (*Tokens, error) {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	return &Tokens{secret: []byte(secret)}, nil
}

func (t *Tokens) Issue(userID uint, role string) (string, error) {
	if userID == 0 || !model.ValidRole(role) {
		return "", errors.New("invalid identity")
	}
	claims := Claims{UserID: userID, Role: role, RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

func (t *Tokens) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(_ *jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid || claims.UserID == 0 || !model.ValidRole(claims.Role) {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
