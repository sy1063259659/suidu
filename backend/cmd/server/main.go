package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sy1063259659/suidu/backend/internal/clipboard"
)

func main() {
	repository, cleanup := buildRepository()
	defer cleanup()

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	if err := router.SetTrustedProxies(nil); err != nil {
		panic(err)
	}
	clipboard.NewHandler(repository).RegisterRoutes(router.Group("/api"))

	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "suidu-api",
		})
	})

	port := os.Getenv("SUIDU_PORT")
	if port == "" {
		port = "8080"
	}

	if err := router.Run(":" + port); err != nil {
		panic(err)
	}
}

func buildRepository() (clipboard.Repository, func()) {
	databaseURL := os.Getenv("SUIDU_DATABASE_URL")
	if databaseURL == "" {
		return clipboard.NewMemoryRepository(), func() {}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repository, err := clipboard.NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		panic(err)
	}
	return repository, repository.Close
}
