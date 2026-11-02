package handler

import (
	"github.com/gin-gonic/gin"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"strings"
)

type CopyHandler struct{ repo repository.CopyRepository }

func NewCopyHandler(r repository.CopyRepository) *CopyHandler { return &CopyHandler{r} }

// Create godoc
// @Summary Add a physical copy of a book
// @Tags copies
// @Accept json
// @Produce json
// @Param id path int true "Book ID"
// @Param copy body model.CopyInput true "Copy fields; status defaults to available"
// @Success 201 {object} model.Copy
// @Failure 400,404,409,422,500 {object} model.ErrorResponse
// @Router /books/{id}/copies [post]
// @Security BearerAuth
// @Failure 401,403 {object} model.ErrorResponse
func (h *CopyHandler) Create(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := readInput[model.CopyInput](c)
	if !ok {
		return
	}
	if in.Status == "" {
		in.Status = "available"
	}
	if strings.TrimSpace(in.InventoryNo) == "" || !model.ValidCopyStatus(in.Status) {
		Error(c, 422, "validation_error", "Inventory number and a valid status are required")
		return
	}
	a, err := h.repo.Create(c.Request.Context(), id, in)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(201, a)
}

// List godoc
// @Summary List physical copies of a book
// @Tags copies
// @Produce json
// @Param id path int true "Book ID"
// @Success 200 {array} model.Copy
// @Failure 400,404,500 {object} model.ErrorResponse
// @Router /books/{id}/copies [get]
func (h *CopyHandler) List(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	a, err := h.repo.List(c.Request.Context(), id)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, a)
}

// Availability godoc
// @Summary Count total and available copies of a book
// @Tags copies
// @Produce json
// @Param id path int true "Book ID"
// @Success 200 {object} model.Availability
// @Failure 400,404,500 {object} model.ErrorResponse
// @Router /books/{id}/availability [get]
func (h *CopyHandler) Availability(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	a, err := h.repo.Availability(c.Request.Context(), id)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, a)
}
