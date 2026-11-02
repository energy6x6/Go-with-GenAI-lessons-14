package model

const (
	RoleReader    = "reader"
	RoleLibrarian = "librarian"
	RoleAdmin     = "admin"
)

func ValidRole(role string) bool {
	return role == RoleReader || role == RoleLibrarian || role == RoleAdmin
}

type RegisterRequest struct {
	Email    string  `json:"email" binding:"required,email,max=254"`
	Password string  `json:"password" binding:"required,min=12"`
	Name     *string `json:"name" binding:"omitempty,max=120"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required"`
}

type UpdateProfileRequest struct {
	Name *string `json:"name" binding:"omitempty,max=120"`
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=reader librarian admin"`
}
