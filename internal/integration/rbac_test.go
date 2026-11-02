package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"libratrack/internal/model"
	"libratrack/internal/repository"
)

func newRequest(method, path, body, authorization string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authorization)
	return req
}
func newRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

func TestRBAC_Matrix(t *testing.T) {
	for _, role := range []string{"anonymous", model.RoleReader, model.RoleLibrarian, model.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			f := setup(t)
			token := f.access[role]
			staff := role == model.RoleLibrarian || role == model.RoleAdmin
			protectedStatus, staffStatus, adminStatus := 200, 403, 403
			if role == "anonymous" {
				protectedStatus, staffStatus, adminStatus = 401, 401, 401
			}
			if staff {
				staffStatus = 201
			}
			if role == model.RoleAdmin {
				adminStatus = 200
			}
			updateStatus, deleteStatus := staffStatus, staffStatus
			if staff {
				updateStatus = 200
				deleteStatus = 204
			}
			cases := []struct {
				name, method, path, body string
				want                     int
			}{
				{"public catalog", "GET", "/books", "", 200},
				{"personal loans", "GET", "/me/loans", "", protectedStatus},
				{"create book", "POST", "/books", `{"title":"RBAC Book","isbn":"rbac-unique","category_id":1,"author_ids":[1]}`, staffStatus},
				{"create author", "POST", "/authors", `{"full_name":"RBAC Author"}`, staffStatus},
				{"create category", "POST", "/categories", `{"name":"RBAC Category"}`, staffStatus},
				{"create copy", "POST", "/books/1/copies", `{"inventory_no":"RBAC-COPY"}`, staffStatus},
				{"update book", "PUT", "/books/1", `{"title":"Updated Book","isbn":"rbac-update-unique","category_id":1,"author_ids":[1]}`, updateStatus},
				{"delete book", "DELETE", "/books/10", "", deleteStatus},
				{"list users", "GET", "/users", "", adminStatus},
				{"change role", "PUT", fmt.Sprintf("/users/%d/role", f.users[model.RoleReader].ID), `{"role":"reader"}`, adminStatus},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					w := call(f.router, tc.method, "/api/v1"+tc.path, tc.body, token)
					status(t, w, tc.want)
					if tc.want == 403 {
						errorBody := decode[model.ErrorResponse](t, w)
						if errorBody.Error.Code != "FORBIDDEN" {
							t.Fatal(errorBody)
						}
					}
				})
			}
			var copy model.Copy
			if err := f.db.Where("status = ?", "available").First(&copy).Error; err != nil {
				t.Fatal(err)
			}
			body := fmt.Sprintf(`{"copy_id":%d,"user_id":%d}`, copy.ID, f.users[model.RoleReader].ID)
			w := call(f.router, "POST", "/api/v1/loans", body, token)
			status(t, w, staffStatus)
			var loan model.Loan
			if staff {
				loan = decode[model.Loan](t, w)
			} else {
				var err error
				loan, err = repository.NewLoanRepository(f.db).Borrow(t.Context(), model.BorrowRequest{CopyID: copy.ID, UserID: f.users[model.RoleReader].ID}, time.Now().Add(14*24*time.Hour))
				if err != nil {
					t.Fatal(err)
				}
			}
			wantReturn := staffStatus
			if staff {
				wantReturn = 200
			}
			status(t, call(f.router, "POST", fmt.Sprintf("/api/v1/loans/%d/return", loan.ID), "", token), wantReturn)
			var count int64
			if err := f.db.Model(&model.Book{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 10 {
				t.Fatal("denied handler executed or staff writes failed", count)
			}
		})
	}
}

func TestLoansOwnershipAndLifecycle(t *testing.T) {
	f := setup(t)
	var copies []model.Copy
	if err := f.db.Where("status = ?", "available").Order("id").Limit(2).Find(&copies).Error; err != nil {
		t.Fatal(err)
	}
	if len(copies) != 2 {
		t.Fatal("missing copies")
	}
	var readerLoan model.Loan
	for i, role := range []string{model.RoleReader, model.RoleLibrarian} {
		body := fmt.Sprintf(`{"copy_id":%d,"user_id":%d}`, copies[i].ID, f.users[role].ID)
		w := call(f.router, "POST", "/api/v1/loans", body, f.access[model.RoleAdmin])
		status(t, w, 201)
		if i == 0 {
			readerLoan = decode[model.Loan](t, w)
		}
	}
	w := call(f.router, "GET", "/api/v1/me/loans", "", f.access[model.RoleReader])
	status(t, w, 200)
	loans := decode[[]model.Loan](t, w)
	if len(loans) != 1 || loans[0].UserID != f.users[model.RoleReader].ID {
		t.Fatal("another user's loans exposed", loans)
	}
	status(t, call(f.router, "GET", fmt.Sprintf("/api/v1/me/loans?user_id=%d", f.users[model.RoleLibrarian].ID), "", f.access[model.RoleReader]), 400)
	body := fmt.Sprintf(`{"copy_id":%d,"user_id":%d}`, copies[0].ID, f.users[model.RoleReader].ID)
	status(t, call(f.router, "POST", "/api/v1/loans", body, f.access[model.RoleAdmin]), 409)
	path := fmt.Sprintf("/api/v1/loans/%d/return", readerLoan.ID)
	status(t, call(f.router, "POST", path, "", f.access[model.RoleReader]), 403)
	status(t, call(f.router, "POST", path, "", f.access[model.RoleLibrarian]), 200)
	status(t, call(f.router, "POST", path, "", f.access[model.RoleAdmin]), 409)
	var copy model.Copy
	if err := f.db.First(&copy, copies[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if copy.Status != "available" {
		t.Fatal("return did not make copy available")
	}
	status(t, call(f.router, "POST", "/api/v1/loans", body, f.access[model.RoleLibrarian]), 201)
}
