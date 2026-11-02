package handler

import (
	"github.com/gin-gonic/gin"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

type UserHandler struct{ users repository.UserRepository }

func NewUserHandler(users repository.UserRepository) *UserHandler { return &UserHandler{users} }

// List godoc
// @Summary List users (admin only; password hashes are excluded)
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page, default 1"
// @Param limit query int false "Limit, maximum 100"
// @Success 200 {array} model.User
// @Failure 400,401,403,500 {object} model.ErrorResponse
// @Router /users [get]
func (h *UserHandler) List(c *gin.Context) {
	limit, offset, ok := boundedPage(c)
	if !ok {
		return
	}
	users, err := h.users.List(c.Request.Context(), limit, offset)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, users)
}

// SetRole godoc
// @Summary Change a user's role (admin only; previous tokens become invalid)
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param request body model.UpdateRoleRequest true "New role"
// @Success 200 {object} model.User
// @Failure 400,401,403,404,422,500 {object} model.ErrorResponse
// @Router /users/{id}/role [put]
func (h *UserHandler) SetRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := readInput[model.UpdateRoleRequest](c)
	if !ok {
		return
	}
	user, err := h.users.SetRole(c.Request.Context(), id, in.Role)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, user)
}

// UpdateProfile godoc
// @Summary Update your own display name (no role, ID, hash or email changes)
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body model.UpdateProfileRequest true "Editable profile fields"
// @Success 200 {object} model.User
// @Failure 401,422,500 {object} model.ErrorResponse
// @Router /me/profile [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	in, ok := readInput[model.UpdateProfileRequest](c)
	if !ok {
		return
	}
	id := c.MustGet("user_id").(uint)
	user, err := h.users.UpdateProfile(c.Request.Context(), id, in)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, user)
}
