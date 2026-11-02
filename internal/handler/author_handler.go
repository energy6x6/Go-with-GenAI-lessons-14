package handler

import (
	"github.com/gin-gonic/gin"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"strings"
)

type AuthorHandler struct{ repo repository.AuthorRepository }

func NewAuthorHandler(r repository.AuthorRepository) *AuthorHandler { return &AuthorHandler{r} }

// Create godoc
// @Summary Create an author
// @Tags authors
// @Accept json
// @Produce json
// @Param author body model.AuthorInput true "Author fields"
// @Success 201 {object} model.Author
// @Failure 422,500 {object} model.ErrorResponse
// @Router /authors [post]
// @Security BearerAuth
// @Failure 401,403 {object} model.ErrorResponse
func (h *AuthorHandler) Create(c *gin.Context) {
	in, ok := readInput[model.AuthorInput](c)
	if !ok {
		return
	}
	if strings.TrimSpace(in.FullName) == "" {
		Error(c, 422, "validation_error", "Author name is required")
		return
	}
	a, err := h.repo.Create(c.Request.Context(), in)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(201, a)
}

// Get godoc
// @Summary Get an author
// @Tags authors
// @Produce json
// @Param id path int true "Author ID"
// @Success 200 {object} model.Author
// @Failure 400,404,500 {object} model.ErrorResponse
// @Router /authors/{id} [get]
func (h *AuthorHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	a, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, a)
}

// List godoc
// @Summary List authors ordered by ID
// @Tags authors
// @Produce json
// @Success 200 {array} model.Author
// @Failure 500 {object} model.ErrorResponse
// @Router /authors [get]
func (h *AuthorHandler) List(c *gin.Context) {
	a, err := h.repo.List(c.Request.Context())
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, a)
}
