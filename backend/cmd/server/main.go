package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sy1063259659/suidu/backend/internal/auth"
	"github.com/sy1063259659/suidu/backend/internal/clipboard"
	"github.com/sy1063259659/suidu/backend/internal/filestore"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "admin" {
		if err := runAdminCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	runtime, cleanup := buildRuntime()
	defer cleanup()

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	if err := router.SetTrustedProxies(nil); err != nil {
		panic(err)
	}
	api := router.Group("/api")
	authHandler := auth.NewHandler(runtime.auth)
	authHandler.RegisterPublicRoutes(api)
	clipboardHandler := clipboard.NewHandler(runtime.repository, runtime.files, maxFileBytes())
	clipboardHandler.RegisterPublicRoutes(api)
	protected := api.Group("")
	protected.Use(runtime.auth.RequireAuth())
	authHandler.RegisterProtectedRoutes(protected)
	clipboardHandler.RegisterRoutes(protected)
	admin := protected.Group("/admin")
	admin.Use(auth.RequireRole(auth.RoleAdmin))
	authHandler.RegisterAdminRoutes(admin)

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

type appRuntime struct {
	repository clipboard.Repository
	files      filestore.Store
	auth       *auth.Service
}

func buildRuntime() (appRuntime, func()) {
	databaseURL := os.Getenv("SUIDU_DATABASE_URL")
	if databaseURL == "" {
		users := auth.NewMemoryStore()
		sessions := auth.NewMemorySessionStore()
		return appRuntime{
				repository: clipboard.NewMemoryRepository(),
				files:      filestore.NewMemoryStore(),
				auth:       auth.NewService(users, sessions, sessionTTL(), cookieSecure()),
			}, func() {
				_ = sessions.Close()
				users.Close()
			}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	users, err := auth.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		panic(err)
	}
	repository, err := clipboard.NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		users.Close()
		panic(err)
	}
	files, err := filestore.NewLocalStore(storageRoot())
	if err != nil {
		repository.Close()
		users.Close()
		panic(err)
	}
	if existing, err := users.ListUsers(ctx); err != nil {
		repository.Close()
		users.Close()
		panic(err)
	} else if len(existing) > 0 {
		if err := repository.AssignOrphanedItems(ctx, existing[0].ID); err != nil {
			repository.Close()
			users.Close()
			panic(err)
		}
	}

	redisAddr := strings.TrimSpace(os.Getenv("SUIDU_REDIS_ADDR"))
	if redisAddr == "" {
		repository.Close()
		users.Close()
		panic("SUIDU_REDIS_ADDR is required when SUIDU_DATABASE_URL is configured")
	}
	redisDB, err := strconv.Atoi(os.Getenv("SUIDU_REDIS_DB"))
	if err != nil {
		redisDB = 0
	}
	sessions, err := auth.NewRedisSessionStore(ctx, redisAddr, os.Getenv("SUIDU_REDIS_PASSWORD"), redisDB, os.Getenv("SUIDU_REDIS_KEY_PREFIX"))
	if err != nil {
		repository.Close()
		users.Close()
		panic(err)
	}
	service := auth.NewService(users, sessions, sessionTTL(), cookieSecure())
	return appRuntime{repository: repository, files: files, auth: service}, func() {
		_ = sessions.Close()
		repository.Close()
		users.Close()
	}
}

func storageRoot() string {
	if value := strings.TrimSpace(os.Getenv("SUIDU_STORAGE_ROOT")); value != "" {
		return value
	}
	return "/var/lib/suidu/files/suidu"
}

func maxFileBytes() int64 {
	if value := strings.TrimSpace(os.Getenv("SUIDU_MAX_FILE_BYTES")); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
			return parsed
		}
	}
	return clipboard.MaxFileBytes
}

func sessionTTL() time.Duration {
	if value := os.Getenv("SUIDU_SESSION_TTL"); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 7 * 24 * time.Hour
}

func cookieSecure() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("SUIDU_COOKIE_SECURE")))
	return value == "1" || value == "true" || value == "yes"
}

func runAdminCommand(args []string) error {
	if len(args) != 1 || (args[0] != "create-user" && args[0] != "reset-password") {
		return errors.New("usage: suidu-api admin <create-user|reset-password>")
	}
	databaseURL := strings.TrimSpace(os.Getenv("SUIDU_DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("SUIDU_DATABASE_URL is required for admin commands")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users, err := auth.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer users.Close()
	sessions := auth.NewMemorySessionStore()
	service := auth.NewService(users, sessions, sessionTTL(), false)

	reader := bufio.NewReader(os.Stdin)
	username, err := prompt(reader, "Username: ")
	if err != nil {
		return err
	}
	password, err := readSecret("Password: ")
	if err != nil {
		return err
	}
	if args[0] == "create-user" {
		confirmation, err := readSecret("Confirm password: ")
		if err != nil {
			return err
		}
		if password != confirmation {
			return errors.New("passwords do not match")
		}
		user, err := service.CreateUser(ctx, username, password, auth.RoleAdmin)
		if err != nil {
			return fmt.Errorf("create administrator: %w", err)
		}
		if err := users.AssignOrphanedClipboardItems(ctx, user.ID); err != nil {
			return fmt.Errorf("assign existing clipboard items: %w", err)
		}
		fmt.Printf("administrator created: %s (id=%d)\n", user.Username, user.ID)
		return nil
	}
	confirmation, err := readSecret("Confirm password: ")
	if err != nil {
		return err
	}
	if password != confirmation {
		return errors.New("passwords do not match")
	}
	record, err := users.FindByUsername(ctx, strings.ToLower(strings.TrimSpace(username)))
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}
	if err := service.ResetPassword(ctx, record.ID, password); err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	fmt.Printf("password reset: %s\n", record.Username)
	return nil
}

func prompt(reader *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	value, err := reader.ReadString('\n')
	return strings.TrimSpace(value), err
}

func readSecret(label string) (string, error) {
	fmt.Print(label)
	value, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return strings.TrimSpace(string(value)), err
}
