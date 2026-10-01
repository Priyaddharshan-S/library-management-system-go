package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"library-system/internal/middleware"
	"library-system/internal/models"
	"library-system/internal/service"
)

type ReservationController struct {
	reservationService *service.ReservationService
}

func NewReservationController(reservationService *service.ReservationService) *ReservationController {
	return &ReservationController{reservationService: reservationService}
}

type reserveRequest struct {
	BookID int `json:"book_id" binding:"required"`
}

// Reserve creates a reservation for the CURRENTLY LOGGED-IN user (taken
// from the JWT via context), not an arbitrary user_id in the body — so a
// member can only ever reserve on their own behalf.
func (ctl *ReservationController) Reserve(c *gin.Context) {
	var req reserveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetInt(middleware.ContextUserIDKey)

	res, err := ctl.reservationService.Reserve(c.Request.Context(), req.BookID, userID)
	if err != nil {
		writeReservationServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, res)
}

// Cancel lets a member cancel their own reservation, or an admin cancel any.
func (ctl *ReservationController) Cancel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reservation id"})
		return
	}

	userID := c.GetInt(middleware.ContextUserIDKey)
	role, _ := c.Get(middleware.ContextRoleKey)
	isAdmin := role == models.RoleAdmin

	if err := ctl.reservationService.Cancel(c.Request.Context(), id, userID, isAdmin); err != nil {
		writeReservationServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (ctl *ReservationController) MyReservations(c *gin.Context) {
	userID := c.GetInt(middleware.ContextUserIDKey)

	reservations, err := ctl.reservationService.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch reservations"})
		return
	}
	c.JSON(http.StatusOK, reservations)
}

// ListByBook is for circulation desk staff to see who's waiting for a book.
func (ctl *ReservationController) ListByBook(c *gin.Context) {
	bookID, err := strconv.Atoi(c.Param("bookId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid book id"})
		return
	}

	reservations, err := ctl.reservationService.ListByBook(c.Request.Context(), bookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch reservations"})
		return
	}
	c.JSON(http.StatusOK, reservations)
}

func writeReservationServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrBookNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "book not found"})
	case errors.Is(err, service.ErrAlreadyReserved):
		c.JSON(http.StatusConflict, gin.H{"error": "you already have an active reservation for this book"})
	case errors.Is(err, service.ErrReservationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "reservation not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected error"})
	}
}
