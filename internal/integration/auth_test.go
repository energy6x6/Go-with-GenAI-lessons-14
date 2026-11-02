package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

func TestAuthentication_TableDriven(t *testing.T) {
	f := setup(t)
	cases := []struct {
		name, path, body string
		want             int
	}{
		{"register", "/auth/register", `{"email":"new@example.com","password":"SecurePassword123!"}`, 201},
		{"duplicate email", "/auth/register", `{"email":"reader@example.com","password":"SecurePassword123!"}`, 422},
		{"case-insensitive duplicate", "/auth/register", `{"email":"READER@example.com","password":"SecurePassword123!"}`, 422},
		{"valid login", "/auth/login", `{"email":"reader@example.com","password":"CorrectPassword123!"}`, 200},
		{"case-insensitive login", "/auth/login", `{"email":"READER@example.com","password":"CorrectPassword123!"}`, 200},
		{"wrong password", "/auth/login", `{"email":"reader@example.com","password":"WrongPassword123!"}`, 401},
		{"unknown email", "/auth/login", `{"email":"absent@example.com","password":"WrongPassword123!"}`, 401},
		{"legacy user cannot log in", "/auth/login", `{"email":"demo-reader@example.com","password":"WrongPassword123!"}`, 401},
		{"reject role assignment", "/auth/register", `{"email":"attack@example.com","password":"SecurePassword123!","role":"admin"}`, 422},
		{"reject hash assignment", "/auth/register", `{"email":"attack@example.com","password":"SecurePassword123!","password_hash":"evil"}`, 422},
		{"short password", "/auth/register", `{"email":"short@example.com","password":"short"}`, 422},
		{"bcrypt byte limit", "/auth/register", fmt.Sprintf(`{"email":"long@example.com","password":%q}`, strings.Repeat("x", 73)), 422},
		{"invalid JSON", "/auth/login", `{broken`, 422},
	}
	var wrong, unknown string
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := call(f.router, "POST", "/api/v1"+tc.path, tc.body, "")
			status(t, w, tc.want)
			if bytes.Contains(w.Body.Bytes(), []byte("password_hash")) || bytes.Contains(w.Body.Bytes(), []byte("$2a$")) {
				t.Fatal("password hash exposed")
			}
			if tc.want == 200 {
				result := decode[struct {
					Token string `json:"token"`
				}](t, w)
				claims, err := f.tokens.Parse(result.Token)
				if err != nil {
					t.Fatal(err)
				}
				if claims.UserID != f.users[model.RoleReader].ID || claims.Role != model.RoleReader {
					t.Fatal("wrong identity")
				}
				remaining := time.Until(claims.ExpiresAt.Time)
				if remaining < 24*time.Hour-3*time.Second || remaining > 24*time.Hour {
					t.Fatal("wrong TTL")
				}
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("token response may be cached")
				}
			}
			if tc.name == "wrong password" {
				wrong = w.Body.String()
			}
			if tc.name == "unknown email" {
				unknown = w.Body.String()
			}
		})
	}
	if wrong != unknown || !strings.Contains(wrong, "invalid credentials") {
		t.Fatal("account enumeration through error response")
	}
	user, err := repository.NewUserRepository(f.db).FindByEmail(t.Context(), "new@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != model.RoleReader || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("SecurePassword123!")) != nil {
		t.Fatal("registration did not persist reader with bcrypt")
	}
	cost, err := bcrypt.Cost([]byte(user.PasswordHash))
	if err != nil || cost != bcrypt.DefaultCost {
		t.Fatal("wrong bcrypt cost")
	}
}

func TestAuthMiddleware_RejectsInvalidTokens(t *testing.T) {
	f := setup(t)
	id := f.users[model.RoleReader].ID
	expired := standardClaims(id, model.RoleReader)
	expired["exp"] = time.Now().Add(-time.Minute).Unix()
	noExp := standardClaims(id, model.RoleReader)
	delete(noExp, "exp")
	cases := []struct{ name, header string }{
		{"missing", ""},
		{"wrong scheme", "Basic abc"},
		{"empty bearer", "Bearer "},
		{"garbage", "Bearer invalid"},
		{"expired", "Bearer " + signed(t, jwt.SigningMethodHS256, expired, []byte(testSecret))},
		{"missing expiration", "Bearer " + signed(t, jwt.SigningMethodHS256, noExp, []byte(testSecret))},
		{"wrong signature", "Bearer " + signed(t, jwt.SigningMethodHS256, standardClaims(id, model.RoleReader), []byte("different-key"))},
		{"wrong algorithm", "Bearer " + signed(t, jwt.SigningMethodHS384, standardClaims(id, model.RoleReader), []byte(testSecret))},
		{"unsigned", "Bearer " + signed(t, jwt.SigningMethodNone, standardClaims(id, model.RoleReader), jwt.UnsafeAllowNoneSignatureType)},
		{"zero identity", "Bearer " + signed(t, jwt.SigningMethodHS256, standardClaims(0, model.RoleReader), []byte(testSecret))},
		{"unknown role", "Bearer " + signed(t, jwt.SigningMethodHS256, standardClaims(id, "boss"), []byte(testSecret))},
		{"deleted user", "Bearer " + signed(t, jwt.SigningMethodHS256, standardClaims(999999, model.RoleReader), []byte(testSecret))},
		{"forged elevated role", "Bearer " + signed(t, jwt.SigningMethodHS256, standardClaims(id, model.RoleAdmin), []byte(testSecret))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := newRequest("GET", "/api/v1/me/loans", "", tc.header)
			w := newRecorder()
			f.router.ServeHTTP(w, req)
			status(t, w, 401)
			if w.Body.String() != `{"error":{"code":"UNAUTHENTICATED","message":"unauthenticated"}}` {
				t.Fatal("inconsistent auth error", w.Body.String())
			}
		})
	}
}

func TestRoleChangeInvalidatesOldToken(t *testing.T) {
	f := setup(t)
	id := f.users[model.RoleLibrarian].ID
	w := call(f.router, "PUT", fmt.Sprintf("/api/v1/users/%d/role", id), `{"role":"reader"}`, f.access[model.RoleAdmin])
	status(t, w, 200)
	status(t, call(f.router, "GET", "/api/v1/me/loans", "", f.access[model.RoleLibrarian]), 401)
	w = call(f.router, "POST", "/api/v1/auth/login", `{"email":"librarian@example.com","password":"CorrectPassword123!"}`, "")
	status(t, w, 200)
	result := decode[struct {
		Token string `json:"token"`
	}](t, w)
	status(t, call(f.router, "POST", "/api/v1/books", `{}`, result.Token), 403)
}

func TestProfileCannotEscalatePrivileges(t *testing.T) {
	f := setup(t)
	for _, body := range []string{`{"role":"admin"}`, `{"id":1}`, `{"password_hash":"evil"}`, `{"email":"admin@example.com"}`} {
		status(t, call(f.router, "PUT", "/api/v1/me/profile", body, f.access[model.RoleReader]), 422)
	}
	status(t, call(f.router, "PUT", "/api/v1/me/profile", `{"name":"Reader Name"}`, f.access[model.RoleReader]), 200)
	user, err := repository.NewUserRepository(f.db).FindByID(t.Context(), f.users[model.RoleReader].ID)
	if err != nil || user.Role != model.RoleReader || user.Name == nil || *user.Name != "Reader Name" {
		t.Fatal("profile update changed internal fields")
	}
}

func TestParameterizedEmailLookup(t *testing.T) {
	f := setup(t)
	_, err := repository.NewUserRepository(f.db).FindByEmail(t.Context(), `' OR '1'='1`)
	if err != repository.ErrNotFound {
		t.Fatal("injection input matched an account", err)
	}
	var count int64
	if err := f.db.Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatal("user data changed", count)
	}
}

func TestUsersResponseNeverContainsPasswordHash(t *testing.T) {
	f := setup(t)
	w := call(f.router, "GET", "/api/v1/users", "", f.access[model.RoleAdmin])
	status(t, w, 200)
	if bytes.Contains(w.Body.Bytes(), []byte("password")) || bytes.Contains(w.Body.Bytes(), []byte("$2")) {
		t.Fatal("sensitive field exposed")
	}
	raw, err := json.Marshal(f.users[model.RoleAdmin])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("PasswordHash")) || bytes.Contains(raw, []byte("$2")) {
		t.Fatal("model serialization exposed hash")
	}
}
