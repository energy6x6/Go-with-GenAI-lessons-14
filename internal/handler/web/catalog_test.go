package web_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"libratrack/internal/app"
	"libratrack/internal/auth"
	webhandler "libratrack/internal/handler/web"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"libratrack/internal/testutil"
)

const password = "CorrectPassword123!"

type fixture struct {
	router   http.Handler
	repos    repository.Repositories
	db       *gorm.DB
	sessions map[string]*http.Cookie
	users    map[string]model.User
}

func setup(t *testing.T) fixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", "web-tests-only-secret-with-at-least-32-bytes")
	t.Setenv("COOKIE_SECURE", "false")
	db := testutil.Database(t)
	repos := repository.NewRepositories(db)
	tokens, err := auth.NewTokensFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	router, err := app.NewRouter(repos, tokens)
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{router: router, repos: repos, db: db, sessions: map[string]*http.Cookie{}, users: map[string]model.User{}}
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{model.RoleReader, model.RoleLibrarian, model.RoleAdmin} {
		u := model.User{Email: role + "@example.com", Role: role, PasswordHash: hash}
		if err := repos.Users.Create(t.Context(), &u); err != nil {
			t.Fatal(err)
		}
		token, err := tokens.Issue(u.ID, u.Role)
		if err != nil {
			t.Fatal(err)
		}
		f.sessions[role] = &http.Cookie{Name: webhandler.SessionCookie, Value: token}
		f.users[role] = u
	}
	return f
}
func request(f fixture, method, path string, form url.Values, cookies []*http.Cookie, hx bool) *httptest.ResponseRecorder {
	var body string
	if form != nil {
		body = form.Encode()
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	if form != nil {
		req.Header.Set("X-CSRF-Token", form.Get("csrf_token"))
	}
	out := httptest.NewRecorder()
	f.router.ServeHTTP(out, req)
	return out
}
func session(t *testing.T, f fixture, role string) ([]*http.Cookie, string) {
	t.Helper()
	cookies := []*http.Cookie{}
	if role != "" {
		cookies = append(cookies, f.sessions[role])
	}
	out := request(f, "GET", "/catalog", nil, cookies, false)
	status(t, out, 200)
	for _, cookie := range out.Result().Cookies() {
		if cookie.Name == webhandler.CSRFCookie {
			cookies = append(cookies, cookie)
			return cookies, cookie.Value
		}
	}
	t.Fatal("missing CSRF cookie")
	return nil, ""
}
func status(t *testing.T, out *httptest.ResponseRecorder, want int) {
	t.Helper()
	if out.Code != want {
		t.Fatalf("want %d got %d: %s", want, out.Code, out.Body.String())
	}
}
func input(csrf, isbn string) url.Values {
	return url.Values{"title": {"Web test book"}, "isbn": {isbn}, "category_id": {"1"}, "author_ids": {"1", "2"}, "published_year": {"2026"}, "description": {"Created through a web form"}, "csrf_token": {csrf}}
}
func newBook(t *testing.T, f fixture, isbn string) model.Book {
	t.Helper()
	b, err := f.repos.Books.Create(t.Context(), model.BookInput{Title: "Delete test", ISBN: isbn, CategoryID: 1, AuthorIDs: []uint{1}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCatalogAndHTMXFragments(t *testing.T) {
	f := setup(t)
	cases := []struct {
		name, path   string
		hx           bool
		want, absent string
		status       int
	}{
		{"full layout", "/catalog", false, "<!DOCTYPE html>", "", 200},
		{"live search", "/web/books?q=Go", true, "Learning Go", "<!DOCTYPE", 200},
		{"search excludes unrelated books", "/web/books?q=Go", true, "The Go Programming Language", "Cosmos", 200},
		{"empty search", "/web/books?q=not-a-real-book", true, "Книг не знайдено", "<html", 200},
		{"category", "/web/books?category_id=3", true, "Cosmos", "Learning Go", 200},
		{"pagination", "/catalog?page=2", false, "Книг не знайдено", "book-1\"", 200},
		{"bad page", "/catalog?page=0", false, "Некоректна сторінка", "", 400},
		{"bad category", "/web/books?category_id=x", true, "Некоректна категорія", "", 400},
		{"detail", "/books/1", false, "ISBN", "", 200},
		{"bad id", "/books/abc", false, "Некоректний ідентифікатор", "", 400},
		{"missing id", "/books/999999", false, "Книгу не знайдено", "", 404},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := request(f, "GET", tc.path, nil, nil, tc.hx)
			status(t, out, tc.status)
			body := out.Body.String()
			if !strings.Contains(body, tc.want) || tc.absent != "" && strings.Contains(body, tc.absent) {
				t.Fatal(body)
			}
			if !strings.HasPrefix(out.Header().Get("Content-Type"), "text/html") {
				t.Fatal("not HTML")
			}
		})
	}
	for _, role := range []string{"", model.RoleReader, model.RoleLibrarian, model.RoleAdmin} {
		t.Run("buttons_"+role, func(t *testing.T) {
			cookies, _ := session(t, f, role)
			out := request(f, "GET", "/catalog", nil, cookies, false)
			has := strings.Contains(out.Body.String(), "hx-delete=")
			if has != (role == model.RoleLibrarian || role == model.RoleAdmin) {
				t.Fatal("incorrect management visibility")
			}
		})
	}
}
func TestWebRBAC(t *testing.T) {
	f := setup(t)
	for _, role := range []string{"", model.RoleReader} {
		t.Run("denied_"+role, func(t *testing.T) {
			cookies, csrf := session(t, f, role)
			for _, path := range []string{"/books/new", "/books/1/edit", "/books/1/delete"} {
				status(t, request(f, "GET", path, nil, cookies, false), 403)
			}
			for _, tc := range []struct{ method, path string }{{"POST", "/web/books"}, {"POST", "/web/books/1"}, {"DELETE", "/web/books/1"}, {"POST", "/web/books/1/delete"}} {
				status(t, request(f, tc.method, tc.path, input(csrf, "denied"), cookies, true), 403)
			}
			if _, err := f.repos.Books.FindByID(t.Context(), 1); err != nil {
				t.Fatal("denied delete mutated data")
			}
		})
	}
	for _, role := range []string{model.RoleLibrarian, model.RoleAdmin} {
		t.Run("allowed_"+role, func(t *testing.T) {
			cookies, csrf := session(t, f, role)
			b := newBook(t, f, "delete-"+role)
			out := request(f, "DELETE", fmt.Sprintf("/web/books/%d", b.ID), url.Values{"csrf_token": {csrf}}, cookies, true)
			status(t, out, 200)
			if out.Body.Len() != 0 {
				t.Fatal("delete must return empty 200")
			}
			if _, err := f.repos.Books.FindByID(t.Context(), b.ID); err != repository.ErrNotFound {
				t.Fatal("book not deleted", err)
			}
			status(t, request(f, "DELETE", fmt.Sprintf("/web/books/%d", b.ID), url.Values{"csrf_token": {csrf}}, cookies, true), 404)
		})
	}
}
func TestWebCRUDAndValidation(t *testing.T) {
	f := setup(t)
	cookies, csrf := session(t, f, model.RoleLibrarian)
	status(t, request(f, "GET", "/books/new", nil, cookies, false), 200)
	form := input(csrf, "web-create-isbn")
	out := request(f, "POST", "/web/books", form, cookies, false)
	status(t, out, 303)
	books, err := f.repos.Books.List(t.Context(), repository.BookFilter{Search: "web-create-isbn", Page: 1, Limit: 20})
	if err != nil || len(books) != 1 {
		t.Fatal(books, err)
	}
	b := books[0]
	path := fmt.Sprintf("/web/books/%d", b.ID)
	status(t, request(f, "GET", fmt.Sprintf("/books/%d/edit", b.ID), nil, cookies, false), 200)
	form.Set("title", "Edited web title")
	status(t, request(f, "POST", path, form, cookies, false), 303)
	b, err = f.repos.Books.FindByID(t.Context(), b.ID)
	if err != nil || b.Title != "Edited web title" || len(b.Authors) != 2 {
		t.Fatal(b, err)
	}
	duplicate := request(f, "POST", "/web/books", form, cookies, false)
	status(t, duplicate, 409)
	if !strings.Contains(duplicate.Body.String(), "Edited web title") {
		t.Fatal("lost submitted fields")
	}
	cases := []struct {
		name   string
		change func(url.Values)
	}{
		{"empty title", func(v url.Values) { v.Set("title", "") }}, {"no author", func(v url.Values) { v.Del("author_ids") }},
		{"duplicate author", func(v url.Values) { v["author_ids"] = []string{"1", "1"} }}, {"bad year", func(v url.Values) { v.Set("published_year", "bad") }},
		{"missing category", func(v url.Values) { v.Set("category_id", "999999") }}, {"missing author", func(v url.Values) { v.Set("author_ids", "999999") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := input(csrf, "invalid-"+tc.name)
			tc.change(v)
			status(t, request(f, "POST", "/web/books", v, cookies, false), 422)
		})
	}
	status(t, request(f, "GET", fmt.Sprintf("/books/%d/delete", b.ID), nil, cookies, false), 200)
	status(t, request(f, "POST", path+"/delete", url.Values{"csrf_token": {csrf}}, cookies, false), 303)
}
func TestCSRFProtection(t *testing.T) {
	f := setup(t)
	cookies, csrf := session(t, f, model.RoleLibrarian)
	for _, value := range []string{"", "wrong", csrf + "x"} {
		v := input(value, "csrf-rejected")
		status(t, request(f, "POST", "/web/books", v, cookies, false), 403)
	}
	status(t, request(f, "POST", "/web/books", input(csrf, "missing-cookie"), []*http.Cookie{f.sessions[model.RoleLibrarian]}, false), 403)
	otherCookies, _ := session(t, f, model.RoleAdmin)
	// A valid CSRF token from a different JWT session cannot authorize this write.
	for _, cookie := range otherCookies {
		if cookie.Name == webhandler.CSRFCookie {
			cookie.Value = csrf
		}
	}
	status(t, request(f, "POST", "/web/books", input(csrf, "wrong-session"), otherCookies, false), 403)
	for _, path := range []string{"/login", "/register", "/logout"} {
		status(t, request(f, "POST", path, url.Values{}, nil, false), 403)
	}
	out := request(f, "DELETE", "/web/books/1", url.Values{}, cookies, true)
	status(t, out, 403)
	if out.Header().Get("HX-Retarget") != "#feedback" {
		t.Fatal("error would replace the row")
	}
}
func TestXSSAndLiteralSearch(t *testing.T) {
	f := setup(t)
	payload := `<script>alert("xss")</script><img src=x onerror=alert(1)>`
	b, err := f.repos.Books.Create(t.Context(), model.BookInput{Title: payload, ISBN: "xss-test", CategoryID: 1, AuthorIDs: []uint{1}, Description: &payload})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/catalog", "/web/books?q=xss-test", fmt.Sprintf("/books/%d", b.ID)} {
		out := request(f, "GET", path, nil, nil, true)
		status(t, out, 200)
		if bytes.Contains(out.Body.Bytes(), []byte(payload)) || !bytes.Contains(out.Body.Bytes(), []byte("&lt;script&gt;")) {
			t.Fatal("unescaped user input", out.Body.String())
		}
	}
	for _, q := range []string{"%", "_", "' OR 1=1 --"} {
		out := request(f, "GET", "/web/books?q="+url.QueryEscape(q), nil, nil, true)
		status(t, out, 200)
		if !strings.Contains(out.Body.String(), "Книг не знайдено") {
			t.Fatal("search treated literal input as SQL or wildcard")
		}
	}
}
func TestLoginCookiesLogoutAndRegistration(t *testing.T) {
	f := setup(t)
	cookies, csrf := session(t, f, "")
	bad := url.Values{"email": {"reader@example.com"}, "password": {"incorrect"}, "csrf_token": {csrf}}
	out := request(f, "POST", "/login", bad, cookies, false)
	status(t, out, 401)
	if strings.Contains(out.Body.String(), "incorrect") {
		t.Fatal("password echoed")
	}
	bad.Set("password", password)
	out = request(f, "POST", "/login", bad, cookies, false)
	status(t, out, 303)
	var sessionCookie *http.Cookie
	for _, cookie := range out.Result().Cookies() {
		if cookie.Name == webhandler.SessionCookie {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode || sessionCookie.Path != "/" || sessionCookie.MaxAge != 86400 {
		t.Fatal("bad session cookie")
	}
	out = request(f, "GET", "/catalog", nil, []*http.Cookie{sessionCookie}, false)
	status(t, out, 200)
	if !strings.Contains(out.Body.String(), "reader@example.com") {
		t.Fatal("session not authenticated")
	}
	var logoutCSRF *http.Cookie
	for _, cookie := range out.Result().Cookies() {
		if cookie.Name == webhandler.CSRFCookie {
			logoutCSRF = cookie
		}
	}
	out = request(f, "POST", "/logout", url.Values{"csrf_token": {logoutCSRF.Value}}, []*http.Cookie{sessionCookie, logoutCSRF}, false)
	status(t, out, 303)
	cleared := false
	for _, cookie := range out.Result().Cookies() {
		if cookie.Name == webhandler.SessionCookie && cookie.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("session not cleared")
	}
	registration := url.Values{"email": {"new-reader@example.com"}, "name": {"New Reader"}, "password": {password}, "role": {"admin"}, "csrf_token": {csrf}}
	status(t, request(f, "POST", "/register", registration, cookies, false), 303)
	user, err := f.repos.Users.FindByEmail(t.Context(), "new-reader@example.com")
	if err != nil || user.Role != model.RoleReader || user.PasswordHash == "" {
		t.Fatal("unsafe registration", err)
	}
	status(t, request(f, "POST", "/register", registration, cookies, false), 422)
}
func TestSecureCookieAndInvalidIdentity(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "true")
	f := setup(t)
	t.Setenv("COOKIE_SECURE", "true")
	tokens, err := auth.NewTokensFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	f.router, err = app.NewRouter(f.repos, tokens)
	if err != nil {
		t.Fatal(err)
	}
	cookies, csrf := session(t, f, "")
	out := request(f, "POST", "/login", url.Values{"email": {"reader@example.com"}, "password": {password}, "csrf_token": {csrf}}, cookies, false)
	status(t, out, 303)
	for _, cookie := range out.Result().Cookies() {
		if cookie.Name == webhandler.SessionCookie && !cookie.Secure {
			t.Fatal("Secure not set")
		}
	}
	f.sessions[model.RoleReader].Value = "invalid-jwt"
	out = request(f, "GET", "/catalog", nil, []*http.Cookie{f.sessions[model.RoleReader]}, false)
	status(t, out, 200)
	if strings.Contains(out.Body.String(), "reader@example.com") {
		t.Fatal("invalid token authenticated")
	}
	cookies, _ = session(t, f, model.RoleLibrarian)
	if _, err := f.repos.Users.SetRole(t.Context(), f.users[model.RoleLibrarian].ID, model.RoleReader); err != nil {
		t.Fatal(err)
	}
	status(t, request(f, "GET", "/books/new", nil, cookies, false), 403)
}
func TestAvailabilityDoesNotAddNPlusOneQueries(t *testing.T) {
	f := setup(t)
	db, count := testutil.CountQueries(f.db)
	repo := repository.NewBookRepository(db)
	books, err := repo.List(t.Context(), repository.BookFilter{Page: 1, Limit: 100, IncludeAvailability: true})
	if err != nil {
		t.Fatal(err)
	}
	if count.Load() != 4 || len(books) != 10 || books[0].AvailableCopies != 2 || books[0].TotalCopies != 3 {
		t.Fatalf("queries=%d books=%d first=%+v", count.Load(), len(books), books[0])
	}
	for i := 0; i < 5; i++ {
		newBook(t, f, fmt.Sprintf("stock-%d", i))
	}
	count.Store(0)
	books, err = repo.List(t.Context(), repository.BookFilter{Page: 1, Limit: 100, IncludeAvailability: true})
	if err != nil {
		t.Fatal(err)
	}
	if count.Load() != 4 || len(books) != 15 {
		t.Fatalf("queries=%d books=%d", count.Load(), len(books))
	}
}
