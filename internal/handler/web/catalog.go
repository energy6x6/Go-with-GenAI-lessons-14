package web

import (
	"crypto/rand"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"libratrack/internal/auth"
	"libratrack/internal/middleware"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	assets "libratrack/web"
)

type Handler struct {
	repos       repository.Repositories
	tokens      *auth.Tokens
	credentials *auth.Credentials
	renderer    *assets.Renderer
	csrfKey     []byte
	secure      bool
}

type BookRow struct {
	Book      model.Book
	CanManage bool
	CSRF      string
}
type Option struct {
	ID       uint
	Label    string
	Selected bool
}
type Form struct {
	ID                                                  uint
	Title, ISBN, PublishedYear, Description, CategoryID string
	AuthorIDs                                           []string
	Categories, Authors                                 []Option
	Delete                                              bool
}
type Page struct {
	Title, Error, CSRF, Query, CategoryID, Email, Name, Notice string
	User                                                       *model.User
	CanManage                                                  bool
	Rows                                                       []BookRow
	Categories                                                 []model.Category
	Book                                                       model.Book
	Form                                                       Form
	PageNumber                                                 uint64
	PreviousURL, NextURL                                       string
}

func New(repos repository.Repositories, tokens *auth.Tokens) (*Handler, error) {
	renderer, err := assets.NewRenderer()
	if err != nil {
		return nil, err
	}
	credentials, err := auth.NewCredentials(repos.Users)
	if err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	secure := false
	if raw := os.Getenv("COOKIE_SECURE"); raw != "" {
		secure, err = strconv.ParseBool(raw)
		if err != nil {
			return nil, errors.New("COOKIE_SECURE must be true or false")
		}
	}
	return &Handler{repos: repos, tokens: tokens, credentials: credentials, renderer: renderer, csrfKey: key, secure: secure}, nil
}

func (h *Handler) Register(r *gin.Engine) error {
	static, err := assets.StaticHandler()
	if err != nil {
		return err
	}
	r.GET("/static/*filepath", gin.WrapH(static))
	r.HEAD("/static/*filepath", gin.WrapH(static))
	r.GET("/", func(c *gin.Context) { c.Redirect(303, "/catalog") })
	pages := r.Group("", h.security())
	pages.GET("/catalog", h.Catalog)
	pages.GET("/web/books", h.Books)
	pages.GET("/books/:id", h.Detail)
	pages.GET("/login", h.LoginPage)
	pages.GET("/register", h.RegisterPage)
	pages.POST("/login", middleware.AuthRateLimit(20, time.Minute), h.Login)
	pages.POST("/register", middleware.AuthRateLimit(20, time.Minute), h.RegisterUser)
	pages.POST("/logout", h.Logout)
	managed := pages.Group("", h.requireStaff())
	managed.GET("/books/new", h.NewBook)
	managed.GET("/books/:id/edit", h.EditBook)
	managed.GET("/books/:id/delete", h.DeletePage)
	managed.POST("/web/books", h.SaveBook)
	managed.POST("/web/books/:id", h.SaveBook)
	managed.DELETE("/web/books/:id", h.DeleteBook)
	managed.POST("/web/books/:id/delete", h.DeleteBook)
	return nil
}

func (h *Handler) page(c *gin.Context, title string) Page {
	return Page{Title: title, CSRF: c.GetString("csrf"), User: currentUser(c), CanManage: staff(currentUser(c))}
}
func (h *Handler) render(c *gin.Context, status int, name string, data Page) {
	var b strings.Builder
	if err := h.renderer.Render(&b, name, data); err != nil {
		log.Printf("render %s failed: %v", name, err)
		c.Data(500, "text/html; charset=utf-8", []byte("Не вдалося відобразити сторінку."))
		return
	}
	c.Data(status, "text/html; charset=utf-8", []byte(b.String()))
}
func (h *Handler) fail(c *gin.Context, status int, message string) {
	data := h.page(c, "Помилка")
	data.Error = message
	name := "error"
	if c.GetHeader("HX-Request") == "true" {
		name = "error_fragment"
		c.Header("HX-Retarget", "#feedback")
		c.Header("HX-Reswap", "innerHTML")
	}
	h.render(c, status, name, data)
}
func (h *Handler) repositoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		h.fail(c, 404, "Книгу не знайдено.")
	case errors.Is(err, repository.ErrConflict):
		h.fail(c, 409, "Запис із таким ISBN уже існує.")
	case errors.Is(err, repository.ErrInvalidReference):
		h.fail(c, 409, "Операція суперечить пов’язаним даним. Перевірте авторів, категорію або історію видач.")
	case errors.Is(err, repository.ErrValidation):
		h.fail(c, 422, "Перевірте введені дані.")
	default:
		log.Printf("web repository failure (%T)", err)
		h.fail(c, 500, "Внутрішня помилка. Спробуйте пізніше.")
	}
}
func id(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 32)
	return uint(n), err == nil && n > 0
}
func (h *Handler) loadBook(c *gin.Context) (model.Book, bool) {
	n, ok := id(c)
	if !ok {
		h.fail(c, 400, "Некоректний ідентифікатор книги.")
		return model.Book{}, false
	}
	book, err := h.repos.Books.FindByID(c.Request.Context(), n)
	if err != nil {
		h.repositoryError(c, err)
		return book, false
	}
	return book, true
}

func (h *Handler) catalog(c *gin.Context, fragment bool) {
	data := h.page(c, "Каталог книг")
	data.Query, data.CategoryID = strings.TrimSpace(c.Query("q")), c.Query("category_id")
	if len(data.Query) > 200 {
		h.fail(c, 400, "Пошуковий запит задовгий.")
		return
	}
	page := uint64(1)
	if raw := c.Query("page"); raw != "" {
		n, err := strconv.ParseUint(raw, 10, 32)
		if err != nil || n == 0 {
			h.fail(c, 400, "Некоректна сторінка.")
			return
		}
		page = n
	}
	var category uint
	if data.CategoryID != "" {
		n, err := strconv.ParseUint(data.CategoryID, 10, 32)
		if err != nil || n == 0 {
			h.fail(c, 400, "Некоректна категорія.")
			return
		}
		category = uint(n)
	}
	books, err := h.repos.Books.List(c.Request.Context(), repository.BookFilter{Search: data.Query, CategoryID: category, Page: page, Limit: 20, IncludeAvailability: true})
	if err != nil {
		h.repositoryError(c, err)
		return
	}
	for _, b := range books {
		data.Rows = append(data.Rows, BookRow{Book: b, CanManage: data.CanManage, CSRF: data.CSRF})
	}
	if fragment {
		h.render(c, 200, "book_rows", data)
		return
	}
	data.Categories, err = h.repos.Categories.List(c.Request.Context())
	if err != nil {
		h.repositoryError(c, err)
		return
	}
	data.PageNumber = page
	pageURL := func(n uint64) string {
		q := url.Values{"page": {strconv.FormatUint(n, 10)}, "q": {data.Query}, "category_id": {data.CategoryID}}
		return "/catalog?" + q.Encode()
	}
	if page > 1 {
		data.PreviousURL = pageURL(page - 1)
	}
	if len(books) == 20 {
		data.NextURL = pageURL(page + 1)
	}
	if c.Query("saved") == "1" {
		data.Notice = "Книгу збережено."
	}
	if c.Query("deleted") == "1" {
		data.Notice = "Книгу видалено."
	}
	h.render(c, 200, "catalog", data)
}
func (h *Handler) Catalog(c *gin.Context) { h.catalog(c, false) }
func (h *Handler) Books(c *gin.Context)   { h.catalog(c, true) }
func (h *Handler) Detail(c *gin.Context) {
	book, ok := h.loadBook(c)
	if !ok {
		return
	}
	availability, err := h.repos.Copies.Availability(c.Request.Context(), book.ID)
	if err != nil {
		h.repositoryError(c, err)
		return
	}
	book.AvailableCopies, book.TotalCopies = availability.AvailableCopies, availability.TotalCopies
	data := h.page(c, book.Title)
	data.Book = book
	h.render(c, 200, "book_detail", data)
}

func (h *Handler) populateForm(c *gin.Context, form *Form) error {
	categories, err := h.repos.Categories.List(c.Request.Context())
	if err != nil {
		return err
	}
	for _, value := range categories {
		form.Categories = append(form.Categories, Option{ID: value.ID, Label: value.Name, Selected: form.CategoryID == strconv.FormatUint(uint64(value.ID), 10)})
	}
	authors, err := h.repos.Authors.List(c.Request.Context())
	if err != nil {
		return err
	}
	for _, value := range authors {
		selected := false
		for _, raw := range form.AuthorIDs {
			if raw == strconv.FormatUint(uint64(value.ID), 10) {
				selected = true
			}
		}
		form.Authors = append(form.Authors, Option{ID: value.ID, Label: value.FullName, Selected: selected})
	}
	return nil
}
func (h *Handler) form(c *gin.Context, status int, form Form, message string) {
	if err := h.populateForm(c, &form); err != nil {
		h.repositoryError(c, err)
		return
	}
	data := h.page(c, "Нова книга")
	if form.ID != 0 {
		data.Title = "Редагування книги"
	}
	if form.Delete {
		data.Title = "Видалення книги"
	}
	data.Form, data.Error = form, message
	h.render(c, status, "book_form", data)
}
func (h *Handler) NewBook(c *gin.Context) { h.form(c, 200, Form{}, "") }
func (h *Handler) existingForm(c *gin.Context, deletion bool) {
	book, ok := h.loadBook(c)
	if !ok {
		return
	}
	form := Form{ID: book.ID, Title: book.Title, ISBN: book.ISBN, CategoryID: strconv.FormatUint(uint64(book.CategoryID), 10), Delete: deletion}
	if book.PublishedYear != nil {
		form.PublishedYear = strconv.Itoa(*book.PublishedYear)
	}
	if book.Description != nil {
		form.Description = *book.Description
	}
	for _, a := range book.Authors {
		form.AuthorIDs = append(form.AuthorIDs, strconv.FormatUint(uint64(a.ID), 10))
	}
	h.form(c, 200, form, "")
}
func (h *Handler) EditBook(c *gin.Context)   { h.existingForm(c, false) }
func (h *Handler) DeletePage(c *gin.Context) { h.existingForm(c, true) }
func (h *Handler) SaveBook(c *gin.Context) {
	form := Form{Title: strings.TrimSpace(c.PostForm("title")), ISBN: strings.TrimSpace(c.PostForm("isbn")), CategoryID: c.PostForm("category_id"), AuthorIDs: c.PostFormArray("author_ids"), PublishedYear: c.PostForm("published_year"), Description: c.PostForm("description")}
	if c.Param("id") != "" {
		n, ok := id(c)
		if !ok {
			h.fail(c, 400, "Некоректний ідентифікатор книги.")
			return
		}
		form.ID = n
	}
	category, err := strconv.ParseUint(form.CategoryID, 10, 32)
	input := model.BookInput{Title: form.Title, ISBN: form.ISBN, CategoryID: uint(category)}
	valid := err == nil && category > 0
	for _, raw := range form.AuthorIDs {
		n, err := strconv.ParseUint(raw, 10, 32)
		if err != nil || n == 0 {
			valid = false
		}
		input.AuthorIDs = append(input.AuthorIDs, uint(n))
	}
	if form.PublishedYear != "" {
		year, err := strconv.Atoi(form.PublishedYear)
		if err != nil || year < 0 || year > time.Now().Year()+1 {
			valid = false
		}
		input.PublishedYear = &year
	}
	if form.Description != "" {
		input.Description = &form.Description
	}
	if !valid || !input.Valid() || len(form.Title) > 500 || len(form.ISBN) > 32 || len(form.Description) > 10000 {
		h.form(c, 422, form, "Перевірте назву, ISBN, категорію, авторів і рік видання.")
		return
	}
	if form.ID == 0 {
		_, err = h.repos.Books.Create(c.Request.Context(), input)
	} else {
		_, err = h.repos.Books.Update(c.Request.Context(), form.ID, input)
	}
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			h.form(c, 409, form, "Книга з таким ISBN уже існує.")
			return
		}
		if errors.Is(err, repository.ErrInvalidReference) {
			h.form(c, 422, form, "Оберіть наявну категорію й авторів.")
			return
		}
		h.repositoryError(c, err)
		return
	}
	c.Redirect(303, "/catalog?saved=1")
}
func (h *Handler) DeleteBook(c *gin.Context) {
	n, ok := id(c)
	if !ok {
		h.fail(c, 400, "Некоректний ідентифікатор книги.")
		return
	}
	if err := h.repos.Books.Delete(c.Request.Context(), n); err != nil {
		h.repositoryError(c, err)
		return
	}
	if c.Request.Method == http.MethodDelete {
		c.Status(200)
		return
	}
	c.Redirect(303, "/catalog?deleted=1")
}

func (h *Handler) LoginPage(c *gin.Context) { h.render(c, 200, "login", h.page(c, "Вхід")) }
func (h *Handler) RegisterPage(c *gin.Context) {
	h.render(c, 200, "register", h.page(c, "Реєстрація"))
}
func (h *Handler) Login(c *gin.Context) {
	email, password := strings.TrimSpace(c.PostForm("email")), c.PostForm("password")
	data := h.page(c, "Вхід")
	data.Email = email
	if len(email) > 254 || len(password) > 72 {
		data.Error = "Неправильна електронна адреса або пароль."
		h.render(c, 401, "login", data)
		return
	}
	user, err := h.credentials.Verify(c.Request.Context(), email, password)
	if err != nil {
		if !errors.Is(err, auth.ErrCredentials) {
			h.repositoryError(c, err)
			return
		}
		data.Error = "Неправильна електронна адреса або пароль."
		h.render(c, 401, "login", data)
		return
	}
	token, err := h.tokens.Issue(user.ID, user.Role)
	if err != nil {
		h.fail(c, 500, "Не вдалося виконати вхід.")
		return
	}
	h.cookie(c, SessionCookie, token, 86400)
	h.cookie(c, CSRFCookie, "", -1)
	c.Redirect(303, "/catalog")
}
func (h *Handler) RegisterUser(c *gin.Context) {
	data := h.page(c, "Реєстрація")
	data.Email, data.Name = strings.TrimSpace(c.PostForm("email")), strings.TrimSpace(c.PostForm("name"))
	password := c.PostForm("password")
	parsed, err := mail.ParseAddress(data.Email)
	if err != nil || parsed.Address != data.Email || len(data.Email) > 254 || len(data.Name) > 200 || len([]rune(password)) < 12 || len(password) > 72 {
		data.Error = "Укажіть email і пароль від 12 символів до 72 байтів."
		h.render(c, 422, "register", data)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		h.fail(c, 500, "Не вдалося зареєструвати користувача.")
		return
	}
	user := model.User{Email: data.Email, Name: &data.Name, Role: model.RoleReader, PasswordHash: hash}
	if err := h.repos.Users.Create(c.Request.Context(), &user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			data.Error = "Цю електронну адресу вже зареєстровано."
			h.render(c, 422, "register", data)
			return
		}
		h.repositoryError(c, err)
		return
	}
	c.Redirect(303, "/login")
}
func (h *Handler) Logout(c *gin.Context) {
	h.cookie(c, SessionCookie, "", -1)
	h.cookie(c, CSRFCookie, "", -1)
	c.Redirect(303, "/login")
}
