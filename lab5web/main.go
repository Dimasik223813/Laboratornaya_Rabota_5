package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"goapp/handlers"
	"goapp/middleware"
	"goapp/models"
	"goapp/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "goapp/docs"
)

// ... Swagger аннотации остаются без изменений ...

func main() {
	// ================= ENV =================
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found, using environment variables")
	}

	// ================= DB =================
	var db *gorm.DB
	maxRetries := 5
	retryDelay := 3 * time.Second

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)

	log.Printf("Connecting to database: host=%s port=%s user=%s dbname=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_NAME"),
	)

	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err == nil {
			break
		}
		log.Printf("Failed to connect (attempt %d/%d): %v", i+1, maxRetries, err)
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	if err != nil {
		log.Fatal("Failed to connect database after retries:", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get sql.DB:", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	log.Println("✅ Connected to DB")

	// UUID extension
	err = db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto";`).Error
	if err != nil {
		log.Fatal("failed to enable pgcrypto:", err)
	}

	// ================= MIGRATIONS =================
	err = db.AutoMigrate(
		&models.User{},
		&models.Article{},
		&models.RefreshToken{},
	)

	if err != nil {
		log.Fatal("failed to migrate:", err)
	}

	log.Println("Database migrated")

	// ================= REDIS CACHE =================
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	cacheTTL, _ := time.ParseDuration(os.Getenv("CACHE_TTL_DEFAULT") + "s")

	if cacheTTL == 0 {
		cacheTTL = 300 * time.Second
	}

	cacheService := services.NewCacheService(redisHost, redisPort, redisPassword, redisDB, cacheTTL)

	// ================= SERVICES =================
	authService := services.NewAuthService(db, cacheService)

	// ================= ROUTER =================
	r := gin.Default()

	// ================= CORS =================
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:4200",
			"http://localhost:8080",
		},
		AllowMethods: []string{
			"GET",
			"POST",
			"PUT",
			"DELETE",
			"OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
		},
		AllowCredentials: true,
	}))

	// ================= SWAGGER =================
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "development" || appEnv == "local" || appEnv == "" {
		r.GET("/api/docs/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
		log.Println("✅ Swagger UI доступен на http://localhost:8080/api/docs/index.html")
	}

	// ================= AUTH ROUTES =================
	auth := r.Group("/auth")
	{
		auth.POST("/register", handlers.RegisterHandler(db, cacheService))
		auth.POST("/login", handlers.LoginHandler(db, cacheService))
		auth.POST("/refresh", handlers.RefreshHandler(db, cacheService))

		auth.GET("/oauth/:provider", handlers.OAuthStartHandler())
		auth.GET("/oauth/:provider/callback", handlers.OAuthCallbackHandler(db, cacheService))

		auth.GET("/whoami",
			middleware.AuthMiddleware(authService),
			handlers.WhoAmIHandler(db, cacheService),
		)

		auth.POST("/logout",
			middleware.AuthMiddleware(authService),
			handlers.LogoutHandler(db, cacheService),
		)

		auth.POST("/logout-all",
			middleware.AuthMiddleware(authService),
			handlers.LogoutAllHandler(db, cacheService),
		)
	}

	// ================= ARTICLES ROUTES =================
	articles := r.Group("/api/articles")
	articles.Use(middleware.AuthMiddleware(authService))
	{
		articles.POST("/", handlers.CreateArticleHandler(db, cacheService))
		articles.GET("/", handlers.GetArticlesHandler(db, cacheService))
		articles.GET("/:id", handlers.GetArticleByIDHandler(db, cacheService))
		articles.PUT("/:id", handlers.UpdateArticleHandler(db, cacheService))
		articles.DELETE("/:id", handlers.DeleteArticleHandler(db, cacheService))
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		status := "ok"
		if !cacheService.IsEnabled() {
			status = "degraded (cache disabled)"
		}
		c.JSON(200, gin.H{
			"status": status,
			"env":    appEnv,
			"cache":  cacheService.IsEnabled(),
		})
	})

	// ================= START =================
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("🚀 Server running on port:", port)
	log.Println("📝 Health check: http://localhost:" + port + "/health")

	err = r.Run(":" + port)
	if err != nil {
		log.Fatal("failed to start server:", err)
	}
}
