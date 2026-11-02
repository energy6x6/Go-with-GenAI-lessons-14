package handler

import (
	"github.com/gin-gonic/gin"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"strings"
)

type CategoryHandler struct{ repo repository.CategoryRepository }

func NewCategoryHandler(r repository.CategoryRepository) *CategoryHandler { return &CategoryHandler{r} }

// Create godoc
// @Summary Create a unique category
// @Tags categories
// @Accept json
// @Produce json
// @Param category body model.CategoryInput true "Category fields"
// @Success 201 {object} model.Category
// @Failure 409,422,500 {object} model.ErrorResponse
// @Router /categories [post]
// @Security BearerAuth
// @Failure 401,403 {object} model.ErrorResponse
func (h *CategoryHandler) Create(c *gin.Context) {
	in, ok := readInput[model.CategoryInput](c)
	if !ok {
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		Error(c, 422, "validation_error", "Category name is required")
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
// @Summary Get a category
// @Tags categories
// @Produce json
// @Param id path int true "Category ID"
// @Success 200 {object} model.Category
// @Failure 400,404,500 {object} model.ErrorResponse
// @Router /categories/{id} [get]
func (h *CategoryHandler) Get(c *gin.Context) {
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
// @Summary List categories ordered by ID
// @Tags categories
// @Produce json
// @Success 200 {array} model.Category
// @Failure 500 {object} model.ErrorResponse
// @Router /categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	a, err := h.repo.List(c.Request.Context())
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, a)
}
