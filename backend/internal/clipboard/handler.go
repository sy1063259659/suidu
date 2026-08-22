package clipboard

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sy1063259659/suidu/backend/internal/auth"
)

const (
	defaultLimit = 50
	maxLimit     = 100
)

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.GET("/clipboard", h.list)
	router.POST("/clipboard", h.create)
	router.DELETE("/clipboard/:id", h.delete)
}

type createRequest struct {
	Content string `json:"content"`
	Source  string `json:"source"`
}

func (h *Handler) list(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	limit := defaultLimit
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeError(c, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > maxLimit {
			parsed = maxLimit
		}
		limit = parsed
	}

	items, err := h.repo.List(c.Request.Context(), user.ID, limit)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to list clipboard items")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) create(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	var request createRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "request body must be valid JSON")
		return
	}

	item, err := h.repo.Create(c.Request.Context(), user.ID, request.Content, request.Source)
	if errors.Is(err, ErrEmptyContent) || errors.Is(err, ErrContentTooLarge) {
		writeError(c, http.StatusBadRequest, "content must not be empty and must be at most 1 MiB")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to create clipboard item")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) delete(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(c, http.StatusBadRequest, "id must be a positive integer")
		return
	}

	if err := h.repo.Delete(c.Request.Context(), user.ID, id); errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "clipboard item not found")
		return
	} else if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to delete clipboard item")
		return
	}
	c.Status(http.StatusNoContent)
}

func writeError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}
