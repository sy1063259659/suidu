package clipboard

import "strings"

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
