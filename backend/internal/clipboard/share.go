package clipboard

import (
	"strings"
	"time"
)

const (
	MinShareTTL       = 5 * time.Minute
	MaxShareTTL       = 365 * 24 * time.Hour
	shareTTLTolerance = time.Second
)

type Share struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"-"`
	Token     string     `json:"token"`
	Item      Item       `json:"item"`
	ExpiresAt time.Time  `json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type CreateShareInput struct {
	ItemID    int64
	Token     string
	ExpiresAt time.Time
}

func validateShareInput(input CreateShareInput, now time.Time) error {
	ttl := input.ExpiresAt.Sub(now)
	if input.ItemID < 1 || strings.TrimSpace(input.Token) == "" || ttl < MinShareTTL-shareTTLTolerance || ttl > MaxShareTTL {
		return ErrInvalidShare
	}
	return nil
}
