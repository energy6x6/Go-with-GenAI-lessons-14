package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

type LoanHandler struct{ loans repository.LoanRepository }

func NewLoanHandler(loans repository.LoanRepository) *LoanHandler { return &LoanHandler{loans} }

// Borrow godoc
// @Summary Issue an available copy to a user (staff only, 14-day loan)
// @Tags loans
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body model.BorrowRequest true "Copy and borrower IDs"
// @Success 201 {object} model.Loan
// @Failure 401,403,404,409,422,500 {object} model.ErrorResponse
// @Router /loans [post]
func (h *LoanHandler) Borrow(c *gin.Context) {
	in, ok := readInput[model.BorrowRequest](c)
	if !ok {
		return
	}
	loan, err := h.loans.Borrow(c.Request.Context(), in, time.Now().UTC().Add(14*24*time.Hour))
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(201, loan)
}

// Return godoc
// @Summary Return an active loan (staff only)
// @Tags loans
// @Produce json
// @Security BearerAuth
// @Param id path int true "Loan ID"
// @Success 200 {object} model.Loan
// @Failure 400,401,403,404,409,500 {object} model.ErrorResponse
// @Router /loans/{id}/return [post]
func (h *LoanHandler) Return(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	loan, err := h.loans.Return(c.Request.Context(), id, time.Now().UTC())
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, loan)
}

// MyLoans godoc
// @Summary List only the authenticated user's loans
// @Tags loans
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page, default 1"
// @Param limit query int false "Limit, maximum 100"
// @Success 200 {array} model.Loan
// @Failure 400,401,500 {object} model.ErrorResponse
// @Router /me/loans [get]
func (h *LoanHandler) MyLoans(c *gin.Context) {
	if _, present := c.GetQuery("user_id"); present {
		Error(c, 400, "invalid_filter", "User identity comes from the access token")
		return
	}
	limit, offset, ok := boundedPage(c)
	if !ok {
		return
	}
	id := c.MustGet("user_id").(uint)
	loans, err := h.loans.ForUser(c.Request.Context(), id, limit, offset)
	if err != nil {
		repositoryError(c, err)
		return
	}
	c.JSON(200, loans)
}
