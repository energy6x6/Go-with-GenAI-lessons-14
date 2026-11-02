package app

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "libratrack/docs"
	"libratrack/internal/auth"
	"libratrack/internal/handler"
	webhandler "libratrack/internal/handler/web"
	"libratrack/internal/middleware"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"net/http"
	"time"
)

func NewRouter(repos repository.Repositories, tokens *auth.Tokens) (http.Handler, error) {
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		return nil, err
	}
	r.RedirectTrailingSlash = false
	r.HandleMethodNotAllowed = true
	r.Use(middleware.RecoveryMiddleware(), middleware.LoggingMiddleware(), middleware.CORSMiddleware())
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	b := handler.NewBookHandler(repos.Books)
	a := handler.NewAuthorHandler(repos.Authors)
	cat := handler.NewCategoryHandler(repos.Categories)
	copies := handler.NewCopyHandler(repos.Copies)
	authH, err := handler.NewAuthHandler(repos.Users, tokens)
	if err != nil {
		return nil, err
	}
	users := handler.NewUserHandler(repos.Users)
	loans := handler.NewLoanHandler(repos.Loans)
	public := r.Group("/api/v1")
	public.GET("/books", b.List)
	public.GET("/books/:id", b.Get)
	public.GET("/authors", a.List)
	public.GET("/authors/:id", a.Get)
	public.GET("/categories", cat.List)
	public.GET("/categories/:id", cat.Get)
	public.GET("/books/:id/copies", copies.List)
	public.GET("/books/:id/availability", copies.Availability)
	credentials := public.Group("/auth", middleware.AuthRateLimit(20, time.Minute))
	credentials.POST("/register", authH.Register)
	credentials.POST("/login", authH.Login)
	protected := r.Group("/api/v1", middleware.AuthMiddleware(tokens, repos.Users))
	protected.GET("/me/loans", loans.MyLoans)
	protected.PUT("/me/profile", users.UpdateProfile)
	staff := protected.Group("", middleware.RequireRole(model.RoleLibrarian, model.RoleAdmin))
	staff.POST("/books", b.Create)
	staff.PUT("/books/:id", b.Update)
	staff.DELETE("/books/:id", b.Delete)
	staff.POST("/authors", a.Create)
	staff.POST("/categories", cat.Create)
	staff.POST("/books/:id/copies", copies.Create)
	staff.POST("/loans", loans.Borrow)
	staff.POST("/loans/:id/return", loans.Return)
	admin := protected.Group("", middleware.RequireRole(model.RoleAdmin))
	admin.GET("/users", users.List)
	admin.PUT("/users/:id/role", users.SetRole)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	web, err := webhandler.New(repos, tokens)
	if err != nil {
		return nil, err
	}
	if err := web.Register(r); err != nil {
		return nil, err
	}
	r.NoRoute(func(c *gin.Context) { handler.Error(c, 404, "not_found", "Route not found") })
	r.NoMethod(func(c *gin.Context) { handler.Error(c, 405, "method_not_allowed", "Method not allowed") })
	return r, nil
}
