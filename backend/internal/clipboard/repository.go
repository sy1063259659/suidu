package clipboard

import (
	"path/filepath"
	"strings"
	"unicode"
)

func validateContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return ErrEmptyContent
	}
	if len(content) > MaxContentBytes {
		return ErrContentTooLarge
	}
	return nil
}

func normalizeMetadata(metadata ItemMetadata) (ItemMetadata, error) {
	metadata.Note = strings.ReplaceAll(metadata.Note, "\r\n", "\n")
	metadata.Note = strings.ReplaceAll(metadata.Note, "\r", "\n")
	metadata.Note = strings.TrimSpace(metadata.Note)
	if len([]rune(metadata.Note)) > MaxNoteRunes || strings.IndexFunc(metadata.Note, func(character rune) bool {
		return unicode.IsControl(character) && character != '\n' && character != '\t'
	}) >= 0 {
		return ItemMetadata{}, ErrInvalidMetadata
	}
	if len(metadata.Tags) > MaxTags {
		return ItemMetadata{}, ErrInvalidMetadata
	}
	normalized := make([]string, 0, len(metadata.Tags))
	seen := make(map[string]struct{}, len(metadata.Tags))
	for _, raw := range metadata.Tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if len([]rune(tag)) > MaxTagRunes || strings.IndexFunc(tag, unicode.IsControl) >= 0 {
			return ItemMetadata{}, ErrInvalidMetadata
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
	}
	metadata.Tags = normalized
	return metadata, nil
}

func normalizeSource(source string) string {
	if strings.TrimSpace(source) == "" {
		return "web"
	}
	return source
}

func validateAttachment(attachment Attachment) error {
	if attachment.Kind != KindImage && attachment.Kind != KindFile {
		return ErrInvalidFile
	}
	if strings.TrimSpace(attachment.FileName) == "" || strings.TrimSpace(attachment.StorageKey) == "" || attachment.SizeBytes < 0 {
		return ErrInvalidFile
	}
	return nil
}

func normalizeFileName(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimSpace(filepath.Base(value))
	if value == "" || value == "." {
		return "file"
	}
	runes := []rune(strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return -1
		}
		return character
	}, value))
	if len(runes) == 0 {
		return "file"
	}
	if len(runes) > 180 {
		value = string(runes[:180])
	} else {
		value = string(runes)
	}
	return value
}
