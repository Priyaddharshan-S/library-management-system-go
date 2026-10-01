package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"library-system/internal/service"
)

type BookController struct {
	bookService *service.BookService
}

func NewBookController(bookService *service.BookService) *BookController {
	return &BookController{bookService: bookService}
}

type bookRequest struct {
	Title       string `json:"title" binding:"required"`
	Author      string `json:"author" binding:"required"`
	ISBN        string `json:"isbn" binding:"required"`
	TotalCopies int    `json:"total_copies" binding:"required,min=1"`
}

// List handles GET /books and GET /books?q=term (search) in one endpoint.
func (ctl *BookController) List(c *gin.Context) {
	term := c.Query("q")

	var books interface{}
	var err error
	if term != "" {
		books, err = ctl.bookService.Search(c.Request.Context(), term)
	} else {
		books, err = ctl.bookService.List(c.Request.Context())
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch books"})
		return
	}

	c.JSON(http.StatusOK, books)
}

func (ctl *BookController) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid book id"})
		return
	}

	book, err := ctl.bookService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrBookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "book not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch book"})
		return
	}

	c.JSON(http.StatusOK, book)
}

func (ctl *BookController) Create(c *gin.Context) {
	var req bookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	book, err := ctl.bookService.Create(c.Request.Context(), service.CreateBookInput{
		Title:       req.Title,
		Author:      req.Author,
		ISBN:        req.ISBN,
		TotalCopies: req.TotalCopies,
	})
	if err != nil {
		writeBookServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, book)
}

func (ctl *BookController) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid book id"})
		return
	}

	var req bookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	book, err := ctl.bookService.Update(c.Request.Context(), id, service.UpdateBookInput{
		Title:       req.Title,
		Author:      req.Author,
		ISBN:        req.ISBN,
		TotalCopies: req.TotalCopies,
	})
	if err != nil {
		writeBookServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, book)
}

func (ctl *BookController) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid book id"})
		return
	}

	if err := ctl.bookService.Delete(c.Request.Context(), id); err != nil {
		writeBookServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func writeBookServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrBookNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "book not found"})
	case errors.Is(err, service.ErrISBNTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "isbn already exists"})
	case errors.Is(err, service.ErrInvalidCopyCount):
		c.JSON(http.StatusBadRequest, gin.H{"error": "total_copies must be at least 1"})
	case errors.Is(err, service.ErrCannotReduceBelow):
		c.JSON(http.StatusConflict, gin.H{"error": "cannot reduce total_copies below the number currently on loan"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected error"})
	}
}
