package httpserver

import (
	"net/http"

	"goal_tracker_server/internal/config"

	"goal_tracker_server/internal/httpserver/handlers"
	"goal_tracker_server/internal/httpserver/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

func NewRouter(cfg config.Config, mongoClient *mongo.Client) *gin.Engine {
	authHandler := handlers.NewAuthHandler(mongoClient, cfg)
	mediaHandler := handlers.NewMediaHandler(mongoClient, cfg)
	quoteHandler := handlers.NewQuoteHandler(mongoClient, cfg)
	rewardHandler := handlers.NewRewardHandler(mongoClient, cfg)
	taskHandler := handlers.NewTaskHandler(mongoClient, cfg)
	notificationHandler := handlers.NewNotificationHandler(mongoClient, cfg)

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * 60 * 60,
	}))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	r.POST("/api/auth/signup", authHandler.Signup)
	r.POST("/api/auth/login", authHandler.Login)

	// Uploaded file streaming.
	// Stored as direct filenames inside cfg.UploadDir.
	r.Static("/uploads", cfg.UploadDir)

	api := r.Group("/api")
	api.Use(middleware.JWTAuth(cfg))
	{
		api.POST("/users/fcm-token", notificationHandler.RegisterFcmToken)

		api.POST("/audios/upload", mediaHandler.UploadAudio)
		api.GET("/audios", mediaHandler.ListAudios)
		api.DELETE("/audios/:id", mediaHandler.DeleteAudio)

		api.POST("/videos/upload", mediaHandler.UploadVideo)
		api.GET("/videos", mediaHandler.ListVideos)
		api.DELETE("/videos/:id", mediaHandler.DeleteVideo)

		// Quotes
		api.POST("/quotes", quoteHandler.Create)
		api.GET("/quotes", quoteHandler.List)
		api.PATCH("/quotes/:id", quoteHandler.Update)
		api.DELETE("/quotes/:id", quoteHandler.Delete)
		api.GET("/quotes/today", quoteHandler.TodayQuote)

		// Rewards
		api.POST("/rewards", rewardHandler.Create)
		api.GET("/rewards", rewardHandler.List)
		api.PATCH("/rewards/:id", rewardHandler.Update)
		api.DELETE("/rewards/:id", rewardHandler.Delete)

		// Tasks
		api.POST("/tasks", taskHandler.Create)
		api.GET("/tasks", taskHandler.List)
		api.PATCH("/tasks/:id", taskHandler.Update)
		api.DELETE("/tasks/:id", taskHandler.Delete)
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "not found",
		})
	})

	return r
}

