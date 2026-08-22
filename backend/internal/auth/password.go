package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory  = 64 * 1024
	argonTime    = 3
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

func validatePassword(password string) error {
	if len(password) < MinPasswordLength || len(password) > MaxPasswordLength {
		return ErrWeakPassword
	}
	return nil
}

func hashPassword(password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encode := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads, encode(salt), encode(hash)), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" {
		return false
	}
	var memory, duration, threads uint32
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &memory, &duration, &threads); err != nil {
		return false
	}
	decode := base64.RawStdEncoding
	salt, err := decode.DecodeString(parts[3])
	if err != nil {
		return false
	}
	expected, err := decode.DecodeString(parts[4])
	if err != nil || len(expected) == 0 || memory == 0 || duration == 0 || threads == 0 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, duration, memory, uint8(threads), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
