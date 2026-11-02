package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"libratrack/internal/auth"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

type AuthHandler struct {
	users       repository.UserRepository
	tokens      *auth.Tokens
	credentials *auth.Credentials
}

func NewAuthHandler(users repository.UserRepository, tokens *auth.Tokens) (*AuthHandler, error) {
	credentials, err := auth.NewCredentials(users)
	if err != nil {
		return nil, err
	}
	return &AuthHandler{users: users, tokens: tokens, credentials: credentials}, nil
}

// Register godoc
// @Summary Register a reader (role cannot be supplied by the client)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body model.RegisterRequest true "Password: 12 or more characters, at most 72 bytes"
// @Success 201 {object} model.User
// @Failure 422,429,500 {object} model.ErrorResponse
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	in, ok := readInput[model.RegisterRequest](c)
	if !ok {
		return
	}
	if len([]byte(in.Password)) > 72 {
		Error(c, 422, "validation_error", "Password must not exceed 72 bytes")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		Error(c, 500, "internal_server_error", "Internal server error")
		return
	}
	user := model.User{Email: in.Email, Name: in.Name, Role: model.RoleReader, PasswordHash: hash}
	if err := h.users.Create(c.Request.Context(), &user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			Error(c, 422, "validation_error", "email already registered")
		} else {
			repositoryError(c, err)
		}
		return
	}
	c.JSON(201, user)
}

// Login godoc
// @Summary Log in and receive an HS256 access token valid for 24 hours
// @Tags auth
// @Accept json
// @Produce json
// @Param request body model.LoginRequest true "Credentials"
// @Success 200 {object} map[string]interface{}
// @Failure 401,422,429,500 {object} model.ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	in, ok := readInput[model.LoginRequest](c)
	if !ok {
		return
	}
	user, err := h.credentials.Verify(c.Request.Context(), in.Email, in.Password)
	if err != nil {
		if errors.Is(err, auth.ErrCredentials) {
			Error(c, 401, "UNAUTHENTICATED", "invalid credentials")
		} else {
			repositoryError(c, err)
		}
		return
	}
	token, err := h.tokens.Issue(user.ID, user.Role)
	if err != nil {
		Error(c, 500, "internal_server_error", "Internal server error")
		return
	}
	c.JSON(200, gin.H{"token": token, "token_type": "Bearer", "expires_in": 86400})
}
