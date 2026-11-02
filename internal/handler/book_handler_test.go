package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"libratrack/internal/app"
	"libratrack/internal/auth"
	"libratrack/internal/handler"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"libratrack/internal/testutil"
)

func request(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	return out
}
func checkStatus(t *testing.T, r *httptest.ResponseRecorder, want int) {
	t.Helper()
	if r.Code != want {
		t.Fatalf("want %d got %d: %s", want, r.Code, r.Body.String())
	}
}
func router(t *testing.T) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", "lesson10-tests-only-secret-32-bytes-long")
	t.Setenv("ALLOWED_ORIGIN", "http://example.com")
	db := testutil.Database(t)
	user := model.User{Email: "regression-admin@example.com", Role: model.RoleAdmin}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.NewTokensFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	token, err := tokens.Issue(user.ID, user.Role)
	if err != nil {
		t.Fatal(err)
	}
	r, err := app.NewRouter(repository.NewRepositories(db), tokens)
	if err != nil {
		t.Fatal(err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" && req.Method != "OPTIONS" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r.ServeHTTP(w, req)
	})
}

func TestHealthAndSwagger(t *testing.T) {
	r := router(t)
	checkStatus(t, request(r, "GET", "/health", ""), 200)
	checkStatus(t, request(r, "GET", "/swagger/index.html", ""), 200)
	doc := request(r, "GET", "/swagger/doc.json", "")
	checkStatus(t, doc, 200)
	if !bytes.Contains(doc.Body.Bytes(), []byte("author_ids")) {
		t.Fatal("Swagger does not describe normalized book requests")
	}
}
func TestCreateBookValidation(t *testing.T) {
	r := router(t)
	for _, body := range []string{"", "null", "[]", "{}", "{bad", `{"title":"x"}`, `{"title":"x","isbn":"1","category_id":1,"author_ids":[]}`, `{"title":"x","isbn":"1","category_id":1,"author_ids":[1,1]}`, `{"title":"x","isbn":"1","category_id":1,"author_ids":[1],"published_year":1.5}`} {
		checkStatus(t, request(r, "POST", "/api/v1/books", body), 422)
	}
}
func TestMalformedIDs(t *testing.T) {
	r := router(t)
	for _, id := range []string{"-1", "abc", "1.5", "4294967296"} {
		checkStatus(t, request(r, "GET", "/api/v1/books/"+id, ""), 400)
	}
}
func TestInvalidPagination(t *testing.T) {
	r := router(t)
	for _, query := range []string{"page=0", "page=", "limit=-1", "limit=x", "page=1&page=2"} {
		checkStatus(t, request(r, "GET", "/api/v1/books?"+query, ""), 400)
	}
}
func TestInvalidCategoryFilter(t *testing.T) {
	r := router(t)
	for _, query := range []string{"category_id=abc", "category_id=0", "category_id=1&category_id=2"} {
		checkStatus(t, request(r, "GET", "/api/v1/books?"+query, ""), 400)
	}
}
func TestMissingBookErrorEnvelope(t *testing.T) {
	out := request(router(t), "GET", "/api/v1/books/999999", "")
	checkStatus(t, out, 404)
	var body model.ErrorResponse
	if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "not_found" || body.Error.Message == "" {
		t.Fatal(body)
	}
}
func TestUnknownReferencesAre422(t *testing.T) {
	r := router(t)
	out := request(r, "POST", "/api/v1/books", `{"title":"x","isbn":"test-invalid-ref","category_id":1,"author_ids":[999999]}`)
	checkStatus(t, out, 422)
	if !bytes.Contains(out.Body.Bytes(), []byte("invalid_reference")) {
		t.Fatal(out.Body.String())
	}
	checkStatus(t, request(r, "GET", "/api/v1/authors/999999", ""), 404)
}
func TestCategoryDuplicateIs409(t *testing.T) {
	out := request(router(t), "POST", "/api/v1/categories", `{"name":"Programming"}`)
	checkStatus(t, out, 409)
}
func TestEmptyNamesAreRejected(t *testing.T) {
	r := router(t)
	checkStatus(t, request(r, "POST", "/api/v1/categories", `{"name":" "}`), 422)
	checkStatus(t, request(r, "POST", "/api/v1/authors", `{"full_name":" "}`), 422)
}
func TestCORSPreflight(t *testing.T) {
	r := router(t)
	req := httptest.NewRequest("OPTIONS", "/api/v1/books", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	out := httptest.NewRecorder()
	r.ServeHTTP(out, req)
	checkStatus(t, out, 204)
	if out.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("missing CORS")
	}
}
func TestEmptyPageIsJSONArray(t *testing.T) {
	out := request(router(t), "GET", "/api/v1/books?page=18446744073709551615", "")
	checkStatus(t, out, 200)
	if out.Body.String() != "[]" {
		t.Fatal(out.Body.String())
	}
}
func TestPutChecksIDBeforeBody(t *testing.T) {
	r := router(t)
	checkStatus(t, request(r, "PUT", "/api/v1/books/abc", `{broken`), 400)
	checkStatus(t, request(r, "PUT", "/api/v1/books/999999", `{broken`), 422)
}
func TestCopyValidation(t *testing.T) {
	r := router(t)
	checkStatus(t, request(r, "POST", "/api/v1/books/1/copies", `{"inventory_no":"bad-copy","status":"broken"}`), 422)
	checkStatus(t, request(r, "POST", "/api/v1/books/999999/copies", `{"inventory_no":"missing-book"}`), 404)
}

type failingBooks struct {
	err      error
	received context.Context
}

func (r *failingBooks) Create(ctx context.Context, _ model.BookInput) (model.Book, error) {
	r.received = ctx
	return model.Book{}, r.err
}
func (r *failingBooks) FindByID(ctx context.Context, _ uint) (model.Book, error) {
	r.received = ctx
	return model.Book{}, r.err
}
func (r *failingBooks) List(ctx context.Context, _ repository.BookFilter) ([]model.Book, error) {
	r.received = ctx
	return nil, r.err
}
func (r *failingBooks) Update(ctx context.Context, _ uint, _ model.BookInput) (model.Book, error) {
	r.received = ctx
	return model.Book{}, r.err
}
func (r *failingBooks) Delete(ctx context.Context, _ uint) error { r.received = ctx; return r.err }
func TestInternalErrorsDoNotLeak(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &failingBooks{err: errors.New("postgres://private:secret@host/db")}
	r := gin.New()
	h := handler.NewBookHandler(repo)
	r.GET("/books/:id", h.Get)
	out := request(r, "GET", "/books/1", "")
	checkStatus(t, out, 500)
	if bytes.Contains(out.Body.Bytes(), []byte("secret")) {
		t.Fatal("credentials leaked")
	}
}
func TestRequestContextReachesRepository(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &failingBooks{err: repository.ErrNotFound}
	r := gin.New()
	h := handler.NewBookHandler(repo)
	r.GET("/books/:id", h.Get)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/books/1", nil).WithContext(ctx)
	r.ServeHTTP(httptest.NewRecorder(), req)
	if repo.received != ctx {
		t.Fatal("request context lost")
	}
}
