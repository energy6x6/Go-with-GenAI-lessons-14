package handler

import (
	"github.com/gin-gonic/gin"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"strconv"
)

type BookHandler struct{ repo repository.BookRepository }

func NewBookHandler(repo repository.BookRepository) *BookHandler { return &BookHandler{repo} }

// Create godoc
// @Summary Create a book with existing authors and category
// @Tags books
// @Accept json
// @Produce json
// @Param book body model.BookInput true "Book fields"
// @Success 201 {object} model.Book
// @Failure 409,422,500 {object} model.ErrorResponse
// @Router /books [post]
// @Security BearerAuth
// @Failure 401,403 {object} model.ErrorResponse
func (h *BookHandler) Create(c *gin.Context) {
	in, ok := readInput[model.BookInput](c)
	if !ok {
		return
	}
	if !in.Valid() {
		Error(c, 422, "validation_error", "Title, ISBN, category and distinct author IDs are required")
		return
	}
	b, err := h.repo.Create(c.Request.Context(), in)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(201, b)
}

// Get godoc
// @Summary Get a book and its relations
// @Tags books
// @Produce json
// @Param id path int true "Book ID"
// @Success 200 {object} model.Book
// @Failure 400,404,500 {object} model.ErrorResponse
// @Router /books/{id} [get]
func (h *BookHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	b, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, b)
}

// Update godoc
// @Summary Replace book fields and author links; omitted optional fields become null
// @Tags books
// @Accept json
// @Produce json
// @Param id path int true "Book ID"
// @Param book body model.BookInput true "Replacement fields"
// @Success 200 {object} model.Book
// @Failure 400,404,409,422,500 {object} model.ErrorResponse
// @Router /books/{id} [put]
// @Security BearerAuth
// @Failure 401,403 {object} model.ErrorResponse
func (h *BookHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := readInput[model.BookInput](c)
	if !ok {
		return
	}
	if !in.Valid() {
		Error(c, 422, "validation_error", "Title, ISBN, category and distinct author IDs are required")
		return
	}
	b, err := h.repo.Update(c.Request.Context(), id, in)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, b)
}

// Delete godoc
// @Summary Delete a book, its copies and author links
// @Tags books
// @Param id path int true "Book ID"
// @Success 204 "No content"
// @Failure 400,404,500 {object} model.ErrorResponse
// @Router /books/{id} [delete]
// @Security BearerAuth
// @Failure 401,403 {object} model.ErrorResponse
func (h *BookHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		repositoryError(c, err)
		return
	}
	c.Status(204)
}

// List godoc
// @Summary List books with authors and categories, ordered by ID
// @Tags books
// @Produce json
// @Param q query string false "Title or ISBN search"
// @Param category_id query int false "Category ID"
// @Param page query int false "Page" default(1) minimum(1)
// @Param limit query int false "Page size, capped at 100" default(10) minimum(1)
// @Success 200 {array} model.Book
// @Failure 400,500 {object} model.ErrorResponse
// @Router /books [get]
func (h *BookHandler) List(c *gin.Context) {
	page, ok := pagination(c, "page", 1)
	if !ok {
		return
	}
	limit, ok := pagination(c, "limit", 10)
	if !ok {
		return
	}
	var categoryID uint
	if raw, present := c.Request.URL.Query()["category_id"]; present {
		if len(raw) != 1 {
			Error(c, 400, "invalid_filter", "Specify one category_id")
			return
		}
		n, err := strconv.ParseUint(raw[0], 10, 32)
		if err != nil || n == 0 {
			Error(c, 400, "invalid_filter", "Category ID must be a positive integer")
			return
		}
		categoryID = uint(n)
	}
	books, err := h.repo.List(c.Request.Context(), repository.BookFilter{Search: c.Query("q"), CategoryID: categoryID, Page: page, Limit: limit})
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, books)
}
