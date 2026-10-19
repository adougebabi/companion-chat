package core

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

var errOwnerReadCursor = errors.New("owner_read_cursor_invalid")

type ownerReadCursor struct {
	Scope    string     `json:"scope"`
	Time     time.Time  `json:"time,omitempty"`
	ID       string     `json:"id,omitempty"`
	Revision int        `json:"revision,omitempty"`
	Snapshot *time.Time `json:"snapshot,omitempty"`
}

func decodeOwnerReadCursor(raw, scope string) (ownerReadCursor, error) {
	c := ownerReadCursor{Scope: scope}
	if raw == "" {
		return c, nil
	}
	if len(raw) > 2048 {
		return c, errOwnerReadCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(b, &c) != nil || c.Scope != scope {
		return c, errOwnerReadCursor
	}
	return c, nil
}
func encodeOwnerReadCursor(c ownerReadCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}
