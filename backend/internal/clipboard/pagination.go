package clipboard

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// A timestamp and an int64 fit comfortably within this bound. Reject larger
// inputs before decoding so cursor parsing never allocates for an arbitrary URL.
const maxListCursorBytes = 128

var errInvalidListCursor = errors.New("invalid clipboard page cursor")

func encodeListCursor(item Item) string {
	payload := item.CreatedAt.UTC().Format(time.RFC3339Nano) + "\n" + strconv.FormatInt(item.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func decodeListCursor(raw string) (*ListCursor, error) {
	if raw == "" || len(raw) > maxListCursorBytes {
		return nil, errInvalidListCursor
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || base64.RawURLEncoding.EncodeToString(payload) != raw {
		return nil, errInvalidListCursor
	}
	timestamp, rawID, ok := strings.Cut(string(payload), "\n")
	if !ok {
		return nil, errInvalidListCursor
	}
	createdAt, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil || createdAt.Format(time.RFC3339Nano) != timestamp {
		return nil, errInvalidListCursor
	}
	// time.Parse accepts a 24-hour zone offset, which RFC3339 forbids.
	_, offset := createdAt.Zone()
	if offset <= -24*60*60 || offset >= 24*60*60 {
		return nil, errInvalidListCursor
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != rawID {
		return nil, errInvalidListCursor
	}
	return &ListCursor{CreatedAt: createdAt, ID: id}, nil
}
