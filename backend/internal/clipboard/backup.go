package clipboard

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sy1063259659/suidu/backend/internal/auth"
	"github.com/sy1063259659/suidu/backend/internal/filestore"
)

const (
	backupVersion           = 1
	maxBackupArchiveBytes   = int64(1 << 30)
	maxBackupManifestBytes  = int64(10 << 20)
	maxBackupItems          = 10000
	maxBackupExpandedBytes  = int64(5 << 30)
	backupManifestEntryName = "manifest.json"
)

type backupManifest struct {
	Version    int          `json:"version"`
	ExportedAt time.Time    `json:"exportedAt"`
	Items      []backupItem `json:"items"`
}

type backupItem struct {
	OriginalID     int64     `json:"originalId"`
	Kind           Kind      `json:"kind"`
	Content        string    `json:"content,omitempty"`
	FileName       string    `json:"fileName,omitempty"`
	MediaType      string    `json:"mediaType,omitempty"`
	SizeBytes      int64     `json:"sizeBytes,omitempty"`
	ContentHash    string    `json:"contentHash"`
	AttachmentPath string    `json:"attachmentPath,omitempty"`
	Note           string    `json:"note,omitempty"`
	Source         string    `json:"source"`
	CreatedAt      time.Time `json:"createdAt"`
	Tags           []string  `json:"tags,omitempty"`
	Favorite       bool      `json:"favorite,omitempty"`
}

type importBackupResponse struct {
	Total             int `json:"total"`
	Imported          int `json:"imported"`
	SkippedDuplicates int `json:"skippedDuplicates"`
}

func (h *Handler) exportBackup(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	items, err := h.repo.List(c.Request.Context(), user.ID, ListFilter{})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "failed to load clipboard backup data")
		return
	}
	for _, item := range items {
		if item.StorageKey == "" {
			continue
		}
		reader, openErr := h.files.Open(c.Request.Context(), item.StorageKey)
		if openErr != nil {
			writeError(c, http.StatusInternalServerError, "a clipboard attachment is unavailable")
			return
		}
		_ = reader.Close()
	}

	filename := "suidu-backup-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	archive := zip.NewWriter(c.Writer)
	manifest := backupManifest{Version: backupVersion, ExportedAt: time.Now().UTC(), Items: make([]backupItem, 0, len(items))}
	for index, item := range items {
		exported := backupItem{
			OriginalID: item.ID, Kind: item.Kind, Content: item.Content, FileName: item.FileName,
			MediaType: item.MediaType, SizeBytes: item.SizeBytes, Note: item.Note, Source: item.Source,
			CreatedAt: item.CreatedAt, Tags: item.Tags, Favorite: item.Favorite,
		}
		if item.Kind == KindText {
			exported.ContentHash = textContentHash(item.Content)
		} else {
			exported.AttachmentPath = fmt.Sprintf("files/%06d-%s", index+1, normalizeFileName(item.FileName))
			reader, openErr := h.files.Open(c.Request.Context(), item.StorageKey)
			if openErr != nil {
				_ = archive.Close()
				return
			}
			header := &zip.FileHeader{Name: exported.AttachmentPath, Method: zip.Deflate}
			header.SetModTime(item.CreatedAt)
			entry, createErr := archive.CreateHeader(header)
			if createErr != nil {
				_ = reader.Close()
				_ = archive.Close()
				return
			}
			digest := sha256.New()
			_, copyErr := io.Copy(io.MultiWriter(entry, digest), reader)
			_ = reader.Close()
			if copyErr != nil {
				_ = archive.Close()
				return
			}
			exported.ContentHash = hex.EncodeToString(digest.Sum(nil))
		}
		manifest.Items = append(manifest.Items, exported)
	}
	manifestEntry, err := archive.Create(backupManifestEntryName)
	if err == nil {
		encoder := json.NewEncoder(manifestEntry)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(manifest)
	}
	if err != nil {
		_ = archive.Close()
		return
	}
	_ = archive.Close()
}

func (h *Handler) importBackup(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBackupArchiveBytes+(1<<20))
	header, err := c.FormFile("backup")
	if err != nil || header.Size < 1 {
		writeError(c, http.StatusBadRequest, "a Suidu backup ZIP is required")
		return
	}
	if header.Size > maxBackupArchiveBytes {
		writeError(c, http.StatusRequestEntityTooLarge, "backup archive must be at most 1 GiB")
		return
	}
	file, err := header.Open()
	if err != nil {
		writeError(c, http.StatusBadRequest, "failed to read backup archive")
		return
	}
	defer file.Close()
	archive, err := zip.NewReader(file, header.Size)
	if err != nil {
		writeError(c, http.StatusBadRequest, "backup archive must be a valid ZIP")
		return
	}
	manifest, entries, err := readBackupArchive(archive)
	if err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	response := importBackupResponse{Total: len(manifest.Items)}
	for _, item := range manifest.Items {
		duplicate, duplicateErr := h.findDuplicate(c.Request.Context(), user.ID, item.Kind, item.ContentHash)
		if duplicateErr == nil {
			_ = duplicate
			response.SkippedDuplicates++
			continue
		}
		if !errors.Is(duplicateErr, ErrNotFound) {
			writeError(c, http.StatusInternalServerError, "failed to check imported clipboard item")
			return
		}

		input := ImportItem{
			Kind: item.Kind, Content: item.Content, Source: item.Source, CreatedAt: item.CreatedAt,
			Metadata: ItemMetadata{Note: item.Note, Tags: item.Tags, Favorite: item.Favorite},
		}
		if item.Kind != KindText {
			entry := entries[item.AttachmentPath]
			reader, openErr := entry.Open()
			if openErr != nil {
				writeError(c, http.StatusBadRequest, "failed to read a backup attachment")
				return
			}
			object, putErr := h.files.Put(c.Request.Context(), user.ID, filepath.Ext(item.FileName), reader, h.maxFileBytes)
			_ = reader.Close()
			if putErr != nil {
				if errors.Is(putErr, filestore.ErrTooLarge) {
					writeError(c, http.StatusRequestEntityTooLarge, h.fileLimitMessage())
					return
				}
				writeError(c, http.StatusInternalServerError, "failed to restore a backup attachment")
				return
			}
			input.Attachment = Attachment{
				Kind: item.Kind, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: object.Size,
				StorageKey: object.Key, ContentHash: item.ContentHash, Source: item.Source,
			}
			if _, importErr := h.repo.Import(c.Request.Context(), user.ID, input); importErr != nil {
				_ = h.files.Delete(c.Request.Context(), object.Key)
				writeError(c, http.StatusInternalServerError, "failed to restore a backup item")
				return
			}
		} else if _, importErr := h.repo.Import(c.Request.Context(), user.ID, input); importErr != nil {
			writeError(c, http.StatusInternalServerError, "failed to restore a backup item")
			return
		}
		response.Imported++
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, response)
}

func readBackupArchive(archive *zip.Reader) (backupManifest, map[string]*zip.File, error) {
	entries := make(map[string]*zip.File, len(archive.File))
	var expandedBytes uint64
	for _, entry := range archive.File {
		if !validBackupEntryPath(entry.Name) {
			return backupManifest{}, nil, errors.New("backup archive contains an invalid path")
		}
		if _, exists := entries[entry.Name]; exists {
			return backupManifest{}, nil, errors.New("backup archive contains duplicate entries")
		}
		entries[entry.Name] = entry
		if entry.UncompressedSize64 > uint64(maxBackupExpandedBytes) || expandedBytes > uint64(maxBackupExpandedBytes)-entry.UncompressedSize64 {
			return backupManifest{}, nil, errors.New("backup archive expands beyond 5 GiB")
		}
		expandedBytes += entry.UncompressedSize64
	}
	manifestEntry := entries[backupManifestEntryName]
	if manifestEntry == nil || manifestEntry.UncompressedSize64 > uint64(maxBackupManifestBytes) {
		return backupManifest{}, nil, errors.New("backup manifest is missing or too large")
	}
	reader, err := manifestEntry.Open()
	if err != nil {
		return backupManifest{}, nil, errors.New("failed to read backup manifest")
	}
	decoder := json.NewDecoder(io.LimitReader(reader, maxBackupManifestBytes+1))
	decoder.DisallowUnknownFields()
	var manifest backupManifest
	err = decoder.Decode(&manifest)
	_ = reader.Close()
	if err != nil || manifest.Version != backupVersion || len(manifest.Items) > maxBackupItems {
		return backupManifest{}, nil, errors.New("backup manifest is invalid or unsupported")
	}
	for _, item := range manifest.Items {
		if err := validateBackupItem(item, entries); err != nil {
			return backupManifest{}, nil, err
		}
	}
	return manifest, entries, nil
}

func validateBackupItem(item backupItem, entries map[string]*zip.File) error {
	if item.Kind != KindText && item.Kind != KindImage && item.Kind != KindFile {
		return errors.New("backup contains an invalid item type")
	}
	if len(item.ContentHash) != sha256.Size*2 {
		return errors.New("backup contains an invalid content hash")
	}
	if _, err := hex.DecodeString(item.ContentHash); err != nil {
		return errors.New("backup contains an invalid content hash")
	}
	if item.CreatedAt.IsZero() {
		return errors.New("backup contains an invalid creation time")
	}
	if _, err := normalizeMetadata(ItemMetadata{Note: item.Note, Tags: item.Tags, Favorite: item.Favorite}); err != nil {
		return errors.New("backup contains invalid metadata")
	}
	if item.Kind == KindText {
		if validateContent(item.Content) != nil || textContentHash(item.Content) != item.ContentHash || item.AttachmentPath != "" {
			return errors.New("backup contains invalid text content")
		}
		return nil
	}
	if item.FileName == "" || !strings.HasPrefix(item.AttachmentPath, "files/") {
		return errors.New("backup attachment metadata is invalid")
	}
	entry := entries[item.AttachmentPath]
	if entry == nil || entry.FileInfo().IsDir() || entry.UncompressedSize64 > uint64(MaxFileBytes) || int64(entry.UncompressedSize64) != item.SizeBytes {
		return errors.New("backup attachment is missing or has an invalid size")
	}
	reader, err := entry.Open()
	if err != nil {
		return errors.New("failed to verify a backup attachment")
	}
	digest := sha256.New()
	_, copyErr := io.Copy(digest, reader)
	_ = reader.Close()
	if copyErr != nil || hex.EncodeToString(digest.Sum(nil)) != item.ContentHash {
		return errors.New("backup attachment integrity check failed")
	}
	return nil
}

func validBackupEntryPath(name string) bool {
	return name != "" && !strings.ContainsRune(name, '\x00') && path.Clean(name) == name && !path.IsAbs(name) && name != ".." && !strings.HasPrefix(name, "../")
}
