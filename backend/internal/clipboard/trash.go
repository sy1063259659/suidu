package clipboard

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sy1063259659/suidu/backend/internal/auth"
	"github.com/sy1063259659/suidu/backend/internal/filestore"
)

// Trash has no retention expiry. Listing defaults to 100 items, capped at 200.
const (
	defaultTrashLimit = 100
	maxTrashLimit     = 200
)

func (h *Handler) listTrash(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	limit := defaultTrashLimit
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeError(c, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = min(parsed, maxTrashLimit)
	}
	items, err := h.repo.ListTrash(c.Request.Context(), user.ID, limit)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to list trash")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) restore(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.repo.Restore(c.Request.Context(), user.ID, id)
	if errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "trashed clipboard item not found")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to restore clipboard item")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, item)
}

func (h *Handler) purge(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	err := h.repo.Purge(c.Request.Context(), user.ID, id, func(ctx context.Context, key string) error {
		err := h.files.Delete(ctx, key)
		if errors.Is(err, filestore.ErrNotFound) {
			return nil
		}
		return err
	})
	if errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "trashed clipboard item not found")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to permanently delete clipboard item")
		return
	}
	c.Status(http.StatusNoContent)
}
