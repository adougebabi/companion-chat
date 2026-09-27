package core

import (
	"fmt"
	"strings"
	"time"
)

// messageTime is captured once at submission/publication. Null fields on old
// messages deliberately remain unknown rather than being inferred later.
type messageTime struct {
	zone   *string
	offset *int
	sentAt *time.Time
}

func parseMessageTime(payload map[string]any) (messageTime, error) {
	zoneRaw, hasZone := payload["sender_timezone"]
	offsetRaw, hasOffset := payload["sender_utc_offset_minutes"]
	sentRaw, hasSent := payload["sender_sent_at"]
	if !hasZone && !hasOffset && !hasSent {
		return messageTime{}, nil
	}
	zone, ok := zoneRaw.(string)
	zone = strings.TrimSpace(zone)
	if !hasZone || !hasOffset || !hasSent || !ok || zone == "" || len(zone) > 128 {
		return messageTime{}, fmt.Errorf("%w: sender time snapshot is incomplete", ErrInvalidArguments)
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return messageTime{}, fmt.Errorf("%w: sender timezone is invalid", ErrInvalidArguments)
	}
	offset, ok := intValueExact(offsetRaw)
	if !ok || offset < -840 || offset > 840 {
		return messageTime{}, fmt.Errorf("%w: sender UTC offset is invalid", ErrInvalidArguments)
	}
	sentText, ok := sentRaw.(string)
	if !ok {
		return messageTime{}, fmt.Errorf("%w: sender sent time is invalid", ErrInvalidArguments)
	}
	sentAt, err := time.Parse(time.RFC3339Nano, sentText)
	if err != nil {
		return messageTime{}, fmt.Errorf("%w: sender sent time is invalid", ErrInvalidArguments)
	}
	_, actual := sentAt.In(location).Zone()
	if actual != offset*60 {
		return messageTime{}, fmt.Errorf("%w: sender UTC offset does not match timezone", ErrInvalidArguments)
	}
	sentAt = sentAt.UTC()
	return messageTime{&zone, &offset, &sentAt}, nil
}

func intValueExact(value any) (int, bool) {
	switch n := value.(type) {
	case int:
		return n, true
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	}
	return 0, false
}

func messageTimeForZone(zone string, at time.Time) (messageTime, error) {
	location, err := time.LoadLocation(zone)
	if err != nil {
		return messageTime{}, err
	}
	_, seconds := at.In(location).Zone()
	offset := seconds / 60
	at = at.UTC()
	return messageTime{&zone, &offset, &at}, nil
}

func (snapshot messageTime) addTo(message map[string]any) {
	message["sender_timezone"] = snapshot.zone
	message["sender_utc_offset_minutes"] = snapshot.offset
	if snapshot.sentAt != nil {
		message["sender_sent_at"] = snapshot.sentAt.UTC().Format(time.RFC3339Nano)
	} else {
		message["sender_sent_at"] = nil
	}
}

func (snapshot messageTime) equal(other messageTime) bool {
	if snapshot.zone == nil || other.zone == nil {
		return snapshot.zone == nil && other.zone == nil
	}
	return *snapshot.zone == *other.zone && *snapshot.offset == *other.offset && snapshot.sentAt.Equal(*other.sentAt)
}
