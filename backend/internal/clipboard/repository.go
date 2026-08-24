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
