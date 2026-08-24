package filestore

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func newKey(userID int64, extension string) (string, error) {
	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	return fmt.Sprintf("%d/%04d/%02d/%s%s", userID, now.Year(), now.Month(), hex.EncodeToString(identifier), safeExtension(extension)), nil
}

func safeExtension(value string) string {
	extension := strings.ToLower(filepath.Ext(value))
	if len(extension) < 2 || len(extension) > 11 {
		return ""
	}
	for _, character := range extension[1:] {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return ""
		}
	}
	return extension
}
