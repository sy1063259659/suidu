package clipboard

import (
	"bytes"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sy1063259659/suidu/backend/internal/auth"
	"github.com/sy1063259659/suidu/backend/internal/filestore"
)

const (
	defaultLimit = 50
	maxLimit     = 100
	MaxFileBytes = 100 << 20
)

type Handler struct {
	repo         Repository
	files        filestore.Store
	maxFileBytes int64
}

func NewHandler(repo Repository, files filestore.Store, maxFileBytes int64) *Handler {
	if maxFileBytes < 1 {
		maxFileBytes = MaxFileBytes
	}
	return &Handler{repo: repo, files: files, maxFileBytes: maxFileBytes}
}

func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.GET("/clipboard", h.list)
	router.POST("/clipboard", h.create)
	router.POST("/clipboard/files", h.upload)
	router.GET("/clipboard/:id/content", h.content)
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

	item, err := h.repo.CreateText(c.Request.Context(), user.ID, request.Content, request.Source)
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

func (h *Handler) upload(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxFileBytes+(1<<20))
	header, err := c.FormFile("file")
	if err != nil {
		writeError(c, http.StatusBadRequest, "a multipart file is required")
		return
	}
	if header.Size > h.maxFileBytes {
		writeError(c, http.StatusRequestEntityTooLarge, "file must be at most 100 MiB")
		return
	}
	file, err := header.Open()
	if err != nil {
		writeError(c, http.StatusBadRequest, "failed to read uploaded file")
		return
	}
	defer file.Close()

	prefix := make([]byte, 512)
	read, err := io.ReadFull(file, prefix)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		writeError(c, http.StatusBadRequest, "failed to read uploaded file")
		return
	}
	prefix = prefix[:read]
	mediaType := http.DetectContentType(prefix)
	kind := KindFile
	if isSafeImageType(mediaType) {
		kind = KindImage
	}
	object, err := h.files.Put(c.Request.Context(), user.ID, filepath.Ext(header.Filename), io.MultiReader(bytes.NewReader(prefix), file), h.maxFileBytes)
	if errors.Is(err, filestore.ErrTooLarge) {
		writeError(c, http.StatusRequestEntityTooLarge, "file must be at most 100 MiB")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to store uploaded file")
		return
	}
	item, err := h.repo.CreateAttachment(c.Request.Context(), user.ID, Attachment{
		Kind: kind, FileName: header.Filename, MediaType: mediaType, SizeBytes: object.Size, StorageKey: object.Key, Source: c.PostForm("source"),
	})
	if err != nil {
		_ = h.files.Delete(c.Request.Context(), object.Key)
		writeError(c, http.StatusInternalServerError, "failed to create clipboard item")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) content(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.repo.Get(c.Request.Context(), user.ID, id)
	if errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "clipboard attachment not found")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to load clipboard attachment")
		return
	}
	if item.StorageKey == "" {
		writeError(c, http.StatusNotFound, "clipboard attachment not found")
		return
	}
	reader, err := h.files.Open(c.Request.Context(), item.StorageKey)
	if errors.Is(err, filestore.ErrNotFound) {
		writeError(c, http.StatusNotFound, "clipboard attachment not found")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to open clipboard attachment")
		return
	}
	defer reader.Close()

	mediaType := item.MediaType
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	disposition := "attachment"
	if item.Kind == KindImage && c.Query("download") == "" {
		disposition = "inline"
	}
	c.Header("Content-Type", mediaType)
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": item.FileName}))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, item.FileName, item.CreatedAt, reader)
}

func (h *Handler) delete(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.repo.Get(c.Request.Context(), user.ID, id)
	if errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "clipboard item not found")
		return
	} else if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to load clipboard item")
		return
	}
	if err := h.repo.Delete(c.Request.Context(), user.ID, id); errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "clipboard item not found")
		return
	} else if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to delete clipboard item")
		return
	}
	if item.StorageKey != "" {
		if err := h.files.Delete(c.Request.Context(), item.StorageKey); err != nil && !errors.Is(err, filestore.ErrNotFound) {
			log.Printf("delete clipboard storage object %q: %v", item.StorageKey, err)
		}
	}
	c.Status(http.StatusNoContent)
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(c, http.StatusBadRequest, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func isSafeImageType(mediaType string) bool {
	switch strings.ToLower(mediaType) {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif", "image/bmp":
		return true
	default:
		return false
	}
}

func writeError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}
