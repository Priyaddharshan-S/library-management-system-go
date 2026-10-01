package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"library-system/internal/config"
	"library-system/internal/controller"
	"library-system/internal/middleware"
	"library-system/internal/models"
	"library-system/internal/notification"
	"library-system/internal/repository"
	"library-system/internal/service"
)

func main() {
	cfg := config.Load()

	// Give the initial connection attempt a reasonable timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("unable to create connection pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("unable to reach database: %v", err)
	}
	log.Println("Connected to database successfully")

	// Wiring: repository -> service -> controller (dependency flows inward).
	userRepo := repository.NewPostgresUserRepository(pool)
	authService := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTExpiry)
	authController := controller.NewAuthController(authService)

	bookRepo := repository.NewPostgresBookRepository(pool)
	bookService := service.NewBookService(bookRepo)
	bookController := controller.NewBookController(bookService)

	reservationRepo := repository.NewPostgresReservationRepository(pool)
	reservationService := service.NewReservationService(reservationRepo, bookRepo)
	reservationController := controller.NewReservationController(reservationService)

	// Choose the email delivery mechanism based on config. Nothing in the
	// service layer needs to know which one is in use.
	var notifier notification.Sender
	if cfg.SMTPHost == "" {
		notifier = notification.NewConsoleSender()
		log.Println("SMTP not configured — reservation-ready emails will be printed to the console instead of sent")
	} else {
		notifier = notification.NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
		log.Printf("Using SMTP server %s:%s for email delivery", cfg.SMTPHost, cfg.SMTPPort)
	}

	loanRepo := repository.NewPostgresLoanRepository(pool)
	loanService := service.NewLoanService(loanRepo, bookRepo, userRepo, reservationRepo, notifier, cfg.LoanDays)
	loanController := controller.NewLoanController(loanService)

	userService := service.NewUserService(userRepo, loanRepo)
	userController := controller.NewUserController(userService)

	reportRepo := repository.NewPostgresReportRepository(pool)
	reportService := service.NewReportService(reportRepo)
	reportController := controller.NewReportController(reportService)

	router := gin.Default()

	// The frontend (a plain HTML file, opened directly in a browser) runs
	// on a different origin than the API, so without CORS headers the
	// browser would block every fetch() call. "*" is fine for local
	// development; a real deployment would restrict this to a known origin.
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	router.GET("/health", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "error",
				"database": "unreachable",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"database": "connected",
		})
	})

	auth := router.Group("/auth")
	{
		auth.POST("/register", authController.Register)
		auth.POST("/login", authController.Login)
	}

	// A minimal protected route to prove the JWT middleware works end to end.
	router.GET("/me", middleware.RequireAuth(authService), func(c *gin.Context) {
		userID, _ := c.Get(middleware.ContextUserIDKey)
		role, _ := c.Get(middleware.ContextRoleKey)
		c.JSON(http.StatusOK, gin.H{"user_id": userID, "role": role})
	})

	// Any logged-in user (admin or member) can view/search books.
	// Only admins can create, update, or delete books.
	books := router.Group("/books")
	books.Use(middleware.RequireAuth(authService))
	{
		books.GET("", bookController.List) // supports ?q=searchterm
		books.GET("/:id", bookController.GetByID)
		books.POST("", middleware.RequireRole(models.RoleAdmin), bookController.Create)
		books.PUT("/:id", middleware.RequireRole(models.RoleAdmin), bookController.Update)
		books.DELETE("/:id", middleware.RequireRole(models.RoleAdmin), bookController.Delete)
	}

	// Circulation desk operations: admin/staff only.
	loans := router.Group("/loans")
	loans.Use(middleware.RequireAuth(authService), middleware.RequireRole(models.RoleAdmin))
	{
		loans.POST("/borrow", loanController.Borrow)
		loans.POST("/:id/return", loanController.Return)
		loans.GET("/active", loanController.ListActive)
		loans.GET("/overdue", loanController.ListOverdue)
	}

	// Any logged-in user can reserve/cancel/view their own reservations.
	// Viewing a book's full reservation queue is admin-only.
	reservations := router.Group("/reservations")
	reservations.Use(middleware.RequireAuth(authService))
	{
		reservations.POST("", reservationController.Reserve)
		reservations.DELETE("/:id", reservationController.Cancel)
		reservations.GET("/mine", reservationController.MyReservations)
		reservations.GET("/book/:bookId", middleware.RequireRole(models.RoleAdmin), reservationController.ListByBook)
	}

	// Reports: admin only.
	reports := router.Group("/reports")
	reports.Use(middleware.RequireAuth(authService), middleware.RequireRole(models.RoleAdmin))
	{
		reports.GET("/summary", reportController.Summary)
	}

	// Patron management: admin only (used by the Circulation Desk and Patrons views).
	users := router.Group("/users")
	users.Use(middleware.RequireAuth(authService), middleware.RequireRole(models.RoleAdmin))
	{
		users.GET("", userController.List)
		users.GET("/:id", userController.GetByID)
		users.GET("/:id/loans", userController.GetLoanHistory)
	}

	log.Printf("Starting server on port %s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
