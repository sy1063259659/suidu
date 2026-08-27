package clipboard

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	router.GET("/clipboard/:id", h.get)
	router.POST("/clipboard", h.create)
	router.POST("/clipboard/files", h.upload)
	router.GET("/clipboard/:id/content", h.content)
	router.PATCH("/clipboard/:id", h.updateMetadata)
	router.DELETE("/clipboard/:id", h.delete)
	router.POST("/clipboard/:id/shares", h.createShare)
	router.GET("/shares", h.listShares)
	router.POST("/shares/:id/revoke", h.revokeShare)
	router.GET("/backup/export", h.exportBackup)
	router.POST("/backup/import", h.importBackup)
}

func (h *Handler) get(c *gin.Context) {
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
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to get clipboard item")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) RegisterPublicRoutes(router gin.IRouter) {
	router.GET("/public/shares/:token", h.publicShare)
	router.GET("/public/shares/:token/content", h.publicShareContent)
}

type createRequest struct {
	Content        string `json:"content"`
	Source         string `json:"source"`
	AllowDuplicate bool   `json:"allowDuplicate"`
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
	query := strings.TrimSpace(c.Query("q"))
	if utf8.RuneCountInString(query) > 200 {
		writeError(c, http.StatusBadRequest, "search query must be at most 200 characters")
		return
	}
	kind := Kind(c.Query("kind"))
	if kind != "" && kind != KindText && kind != KindImage && kind != KindFile {
		writeError(c, http.StatusBadRequest, "kind must be text, image, or file")
		return
	}
	favoriteOnly := false
	if raw := c.Query("favorite"); raw != "" {
		if raw != "true" && raw != "false" {
			writeError(c, http.StatusBadRequest, "favorite must be true or false")
			return
		}
		favoriteOnly = raw == "true"
	}
	createdFrom, createdBefore, ok := parseCreatedRange(c)
	if !ok {
		return
	}

	items, err := h.repo.List(c.Request.Context(), user.ID, ListFilter{
		Limit:         limit,
		Query:         query,
		Kind:          kind,
		FavoriteOnly:  favoriteOnly,
		CreatedFrom:   createdFrom,
		CreatedBefore: createdBefore,
	})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to list clipboard items")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type metadataRequest struct {
	Note     string   `json:"note"`
	Tags     []string `json:"tags"`
	Favorite bool     `json:"favorite"`
}

func (h *Handler) updateMetadata(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var request metadataRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "request body must be valid JSON")
		return
	}
	item, err := h.repo.UpdateMetadata(c.Request.Context(), user.ID, id, ItemMetadata{Note: request.Note, Tags: request.Tags, Favorite: request.Favorite})
	if errors.Is(err, ErrInvalidMetadata) {
		writeError(c, http.StatusBadRequest, "use a note with at most 500 characters and at most 10 tags with at most 24 characters each")
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "clipboard item not found")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to update clipboard metadata")
		return
	}
	c.JSON(http.StatusOK, item)
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
	if err := validateContent(request.Content); errors.Is(err, ErrEmptyContent) || errors.Is(err, ErrContentTooLarge) {
		writeError(c, http.StatusBadRequest, "content must not be empty and must be at most 1 MiB")
		return
	}
	if !request.AllowDuplicate {
		duplicate, duplicateErr := h.findDuplicate(c.Request.Context(), user.ID, KindText, textContentHash(request.Content))
		if duplicateErr == nil {
			writeDuplicate(c, duplicate)
			return
		}
		if !errors.Is(duplicateErr, ErrNotFound) {
			writeError(c, http.StatusInternalServerError, "failed to check duplicate clipboard item")
			return
		}
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
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(c, http.StatusRequestEntityTooLarge, h.fileLimitMessage())
			return
		}
		writeError(c, http.StatusBadRequest, "a multipart file is required")
		return
	}
	if header.Size > h.maxFileBytes {
		writeError(c, http.StatusRequestEntityTooLarge, h.fileLimitMessage())
		return
	}
	allowDuplicate := c.PostForm("allowDuplicate") == "true"
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
	digest := sha256.New()
	object, err := h.files.Put(c.Request.Context(), user.ID, filepath.Ext(header.Filename), io.TeeReader(io.MultiReader(bytes.NewReader(prefix), file), digest), h.maxFileBytes)
	if errors.Is(err, filestore.ErrTooLarge) {
		writeError(c, http.StatusRequestEntityTooLarge, h.fileLimitMessage())
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to store uploaded file")
		return
	}
	contentHash := hex.EncodeToString(digest.Sum(nil))
	if !allowDuplicate {
		duplicate, duplicateErr := h.findDuplicate(c.Request.Context(), user.ID, kind, contentHash)
		if duplicateErr == nil {
			_ = h.files.Delete(c.Request.Context(), object.Key)
			writeDuplicate(c, duplicate)
			return
		}
		if !errors.Is(duplicateErr, ErrNotFound) {
			_ = h.files.Delete(c.Request.Context(), object.Key)
			writeError(c, http.StatusInternalServerError, "failed to check duplicate clipboard item")
			return
		}
	}
	item, err := h.repo.CreateAttachment(c.Request.Context(), user.ID, Attachment{
		Kind: kind, FileName: header.Filename, MediaType: mediaType, SizeBytes: object.Size, StorageKey: object.Key, ContentHash: contentHash, Source: c.PostForm("source"),
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
	h.serveItemContent(c, item)
}

func (h *Handler) serveItemContent(c *gin.Context, item Item) {
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
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, item.FileName, item.CreatedAt, reader)
}

type createShareRequest struct {
	ExpiresInSeconds int64 `json:"expiresInSeconds"`
}

func (h *Handler) createShare(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var request createShareRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "request body must be valid JSON")
		return
	}
	if request.ExpiresInSeconds < int64(MinShareTTL/time.Second) || request.ExpiresInSeconds > int64(MaxShareTTL/time.Second) {
		writeError(c, http.StatusBadRequest, "share expiry must be between 5 minutes and 365 days")
		return
	}
	duration := time.Duration(request.ExpiresInSeconds) * time.Second
	token, err := newShareToken()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to create share token")
		return
	}
	share, err := h.repo.CreateShare(c.Request.Context(), user.ID, CreateShareInput{
		ItemID: id, Token: token, ExpiresAt: time.Now().UTC().Add(duration),
	})
	if errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "clipboard item not found")
		return
	}
	if errors.Is(err, ErrInvalidShare) {
		writeError(c, http.StatusBadRequest, "share expiry is invalid")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to create clipboard share")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, share)
}

func (h *Handler) listShares(c *gin.Context) {
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
	shares, err := h.repo.ListShares(c.Request.Context(), user.ID, limit)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to list clipboard shares")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"shares": shares})
}

func (h *Handler) revokeShare(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.repo.RevokeShare(c.Request.Context(), user.ID, id); errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "clipboard share not found")
		return
	} else if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to revoke clipboard share")
		return
	}
	c.Status(http.StatusNoContent)
}

type publicShareResponse struct {
	Item      Item      `json:"item"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

func (h *Handler) publicShare(c *gin.Context) {
	share, ok := h.resolvePublicShare(c)
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	item := share.Item
	item.Note = ""
	item.Tags = nil
	item.Favorite = false
	c.JSON(http.StatusOK, publicShareResponse{Item: item, ExpiresAt: share.ExpiresAt, CreatedAt: share.CreatedAt})
}

func (h *Handler) publicShareContent(c *gin.Context) {
	share, ok := h.resolvePublicShare(c)
	if !ok {
		return
	}
	h.serveItemContent(c, share.Item)
}

func (h *Handler) resolvePublicShare(c *gin.Context) (Share, bool) {
	token := c.Param("token")
	if !validShareToken(token) {
		writeError(c, http.StatusNotFound, "clipboard share not found")
		return Share{}, false
	}
	share, err := h.repo.GetPublicShare(c.Request.Context(), token)
	if errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "clipboard share not found")
		return Share{}, false
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to load clipboard share")
		return Share{}, false
	}
	return share, true
}

func newShareToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func validShareToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
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
	if item.StorageKey != "" {
		if err := h.files.Delete(c.Request.Context(), item.StorageKey); err != nil && !errors.Is(err, filestore.ErrNotFound) {
			writeError(c, http.StatusInternalServerError, "failed to delete clipboard source file")
			return
		}
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

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(c, http.StatusBadRequest, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func parseCreatedRange(c *gin.Context) (*time.Time, *time.Time, bool) {
	createdFrom, ok := parseRFC3339Query(c, "from")
	if !ok {
		return nil, nil, false
	}
	createdBefore, ok := parseRFC3339Query(c, "to")
	if !ok {
		return nil, nil, false
	}
	if createdFrom != nil && createdBefore != nil && !createdFrom.Before(*createdBefore) {
		writeError(c, http.StatusBadRequest, "from must be earlier than to")
		return nil, nil, false
	}
	return createdFrom, createdBefore, true
}

func parseRFC3339Query(c *gin.Context, key string) (*time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		writeError(c, http.StatusBadRequest, key+" must be a valid RFC3339 timestamp")
		return nil, false
	}
	parsed = parsed.UTC()
	return &parsed, true
}

func isSafeImageType(mediaType string) bool {
	switch strings.ToLower(mediaType) {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif", "image/bmp":
		return true
	default:
		return false
	}
}

func (h *Handler) fileLimitMessage() string {
	return fmt.Sprintf("file must be at most %.1f MiB", float64(h.maxFileBytes)/(1<<20))
}

func writeError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func (h *Handler) findDuplicate(ctx context.Context, userID int64, kind Kind, contentHash string) (Item, error) {
	item, err := h.repo.FindDuplicate(ctx, userID, kind, contentHash)
	if err == nil || !errors.Is(err, ErrNotFound) {
		return item, err
	}
	items, err := h.repo.List(ctx, userID, ListFilter{Kind: kind})
	if err != nil {
		return Item{}, err
	}
	for _, candidate := range items {
		candidateHash := candidate.ContentHash
		if candidateHash == "" && kind == KindText {
			candidateHash = textContentHash(candidate.Content)
		}
		if candidateHash == "" && candidate.StorageKey != "" {
			reader, openErr := h.files.Open(ctx, candidate.StorageKey)
			if openErr != nil {
				continue
			}
			digest := sha256.New()
			_, copyErr := io.Copy(digest, reader)
			_ = reader.Close()
			if copyErr == nil {
				candidateHash = hex.EncodeToString(digest.Sum(nil))
			}
		}
		if candidateHash != "" && candidateHash == contentHash {
			return candidate, nil
		}
	}
	return Item{}, ErrNotFound
}

func writeDuplicate(c *gin.Context, item Item) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusConflict, gin.H{"error": "duplicate clipboard item", "duplicate": item})
}
