package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
	"libratrack/internal/app"
	"libratrack/internal/auth"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"libratrack/internal/testutil"
)

const testSecret = "integration-tests-only-strong-secret-key"
const testPassword = "CorrectPassword123!"

type fixture struct {
	db     *gorm.DB
	router http.Handler
	tokens *auth.Tokens
	users  map[string]model.User
	access map[string]string
}

func setup(t *testing.T) fixture {
	t.Helper()
	t.Setenv("JWT_SECRET", testSecret)
	gin.SetMode(gin.TestMode)
	db := testutil.Database(t)
	tokens, err := auth.NewTokensFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	repos := repository.NewRepositories(db)
	router, err := app.NewRouter(repos, tokens)
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{db: db, router: router, tokens: tokens, users: make(map[string]model.User), access: make(map[string]string)}
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{model.RoleReader, model.RoleLibrarian, model.RoleAdmin} {
		user := model.User{Email: role + "@example.com", Role: role, PasswordHash: hash}
		if err := repos.Users.Create(t.Context(), &user); err != nil {
			t.Fatal(err)
		}
		f.users[role] = user
		token, err := tokens.Issue(user.ID, role)
		if err != nil {
			t.Fatal(err)
		}
		f.access[role] = token
	}
	return f
}

func call(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
func status(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("want %d got %d: %s", want, w.Code, w.Body.String())
	}
}
func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func signed(t *testing.T, method jwt.SigningMethod, claims jwt.MapClaims, key any) string {
	t.Helper()
	raw, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func standardClaims(id uint, role string) jwt.MapClaims {
	return jwt.MapClaims{"user_id": id, "role": role, "exp": time.Now().Add(time.Hour).Unix()}
}
