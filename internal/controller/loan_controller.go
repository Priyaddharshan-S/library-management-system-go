package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"library-system/internal/service"
)

type LoanController struct {
	loanService *service.LoanService
}

func NewLoanController(loanService *service.LoanService) *LoanController {
	return &LoanController{loanService: loanService}
}

type borrowRequest struct {
	BookID int `json:"book_id" binding:"required"`
	UserID int `json:"user_id" binding:"required"`
}

// Borrow is called by circulation desk staff (admin) checking a book out
// to a patron. book_id and user_id are both supplied in the request body.
func (ctl *LoanController) Borrow(c *gin.Context) {
	var req borrowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	loan, err := ctl.loanService.Borrow(c.Request.Context(), req.BookID, req.UserID)
	if err != nil {
		writeLoanServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, loan)
}

func (ctl *LoanController) Return(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid loan id"})
		return
	}

	loan, err := ctl.loanService.Return(c.Request.Context(), id)
	if err != nil {
		writeLoanServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, loan)
}

func (ctl *LoanController) ListActive(c *gin.Context) {
	loans, err := ctl.loanService.ListActive(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch active loans"})
		return
	}
	c.JSON(http.StatusOK, loans)
}

func (ctl *LoanController) ListOverdue(c *gin.Context) {
	loans, err := ctl.loanService.ListOverdue(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch overdue loans"})
		return
	}
	c.JSON(http.StatusOK, loans)
}

func writeLoanServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrBookNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "book not found"})
	case errors.Is(err, service.ErrNoCopiesAvailable):
		c.JSON(http.StatusConflict, gin.H{"error": "no copies available"})
	case errors.Is(err, service.ErrLoanNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "loan not found"})
	case errors.Is(err, service.ErrLoanAlreadyReturned):
		c.JSON(http.StatusConflict, gin.H{"error": "loan already returned"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected error"})
	}
}
