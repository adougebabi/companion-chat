package platform

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const EventStream = "fluctlight:events:v1"

var DurableConsumerGroups = []string{"bff-notifications", "cache-projections", "integration-observers"}

type EventEnvelope struct {
	EventID           string          `json:"event_id"`
	EventType         string          `json:"event_type"`
	SchemaVersion     string          `json:"schema_version"`
	AggregateType     string          `json:"aggregate_type"`
	AggregateID       string          `json:"aggregate_id"`
	AggregateSequence int             `json:"aggregate_sequence"`
	FluctlightID      string          `json:"fluctlight_id,omitempty"`
	CausationID       string          `json:"causation_id"`
	CorrelationID     string          `json:"correlation_id"`
	OccurredAt        time.Time       `json:"occurred_at"`
	Payload           json.RawMessage `json:"payload"`
}

type OutboxPublisher struct {
	Pool         *pgxpool.Pool
	Redis        redis.UniversalClient
	Stream       string
	PublisherID  string
	Lease        time.Duration
	MaxAttempts  int
	MaxStreamLen int64
}

func NewOutboxPublisher(pool *pgxpool.Pool, client redis.UniversalClient, publisherID string) *OutboxPublisher {
	if publisherID == "" {
		publisherID = "go-worker"
	}
	return &OutboxPublisher{Pool: pool, Redis: client, Stream: EventStream, PublisherID: publisherID, Lease: 30 * time.Second, MaxAttempts: 5, MaxStreamLen: 10000}
}

type outboxRow struct {
	ID, Kind, AggregateType, AggregateID, CausationID, CorrelationID string
	FluctlightID                                                     *string
	Payload                                                          []byte
	OccurredAt                                                       time.Time
	AttemptCount                                                     int
}

func (p *OutboxPublisher) PublishOnce(ctx context.Context, limit int) (int, error) {
	if p == nil || p.Pool == nil || p.Redis == nil {
		return 0, errors.New("outbox publisher is unavailable")
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := p.claim(ctx, limit)
	if err != nil {
		return 0, err
	}
	published := 0
	for _, row := range rows {
		envelope := EventEnvelope{EventID: row.ID, EventType: row.Kind, SchemaVersion: "event.v1", AggregateType: row.AggregateType, AggregateID: row.AggregateID, AggregateSequence: payloadSequence(row.Payload), CausationID: row.CausationID, CorrelationID: row.CorrelationID, OccurredAt: row.OccurredAt, Payload: json.RawMessage(row.Payload)}
		if row.FluctlightID != nil {
			envelope.FluctlightID = *row.FluctlightID
		}
		encoded, marshalErr := json.Marshal(envelope)
		if marshalErr != nil {
			_ = p.fail(ctx, row, marshalErr)
			continue
		}
		_, publishErr := p.Redis.XAdd(ctx, &redis.XAddArgs{Stream: p.Stream, Values: map[string]any{"event": string(encoded), "event_id": row.ID, "event_type": row.Kind}}).Result()
		if publishErr != nil {
			_ = p.fail(ctx, row, publishErr)
			continue
		}
		if err := p.markPublished(ctx, row.ID); err != nil {
			return published, err
		}
		published++
	}
	if p.MaxStreamLen > 0 {
		if p.trimSafe(ctx) {
			_ = p.Redis.XTrimMaxLen(ctx, p.Stream, p.MaxStreamLen).Err()
		}
	}
	return published, nil
}

func (p *OutboxPublisher) trimSafe(ctx context.Context) bool {
	for _, group := range DurableConsumerGroups {
		pending, err := p.Redis.XPending(ctx, p.Stream, group).Result()
		if err != nil {
			return false
		}
		if pending != nil && pending.Count > 0 {
			return false
		}
	}
	return true
}

func (p *OutboxPublisher) claim(ctx context.Context, limit int) ([]outboxRow, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,kind,aggregate_type,aggregate_id,fluctlight_id,causation_id,correlation_id,payload,occurred_at,attempt_count FROM public.platform_outbox_events WHERE published_at IS NULL AND failed_at IS NULL AND available_at <= now() AND (claim_until IS NULL OR claim_until < now()) ORDER BY occurred_at,id LIMIT $1 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, err
	}
	claimed := make([]outboxRow, 0, limit)
	for rows.Next() {
		var row outboxRow
		if err := rows.Scan(&row.ID, &row.Kind, &row.AggregateType, &row.AggregateID, &row.FluctlightID, &row.CausationID, &row.CorrelationID, &row.Payload, &row.OccurredAt, &row.AttemptCount); err != nil {
			rows.Close()
			return nil, err
		}
		row.AttemptCount++
		claimed = append(claimed, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, row := range claimed {
		if _, err := tx.Exec(ctx, `UPDATE public.platform_outbox_events SET claim_owner=$2,claim_until=now()+$3::interval,attempt_count=$4 WHERE id=$1`, row.ID, p.PublisherID, fmt.Sprintf("%d seconds", int64(p.Lease/time.Second)), row.AttemptCount); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return claimed, nil
}

func (p *OutboxPublisher) markPublished(ctx context.Context, id string) error {
	command, err := p.Pool.Exec(ctx, `UPDATE public.platform_outbox_events SET published_at=now(),claim_owner=NULL,claim_until=NULL,last_error=NULL WHERE id=$1 AND claim_owner=$2`, id, p.PublisherID)
	if err == nil && command.RowsAffected() != 1 {
		return errors.New("outbox claim was lost before publish settlement")
	}
	return err
}

func (p *OutboxPublisher) fail(ctx context.Context, row outboxRow, cause error) error {
	message := strings.TrimSpace(cause.Error())
	if len(message) > 512 {
		message = message[:512]
	}
	if row.AttemptCount >= p.MaxAttempts {
		_, err := p.Pool.Exec(ctx, `UPDATE public.platform_outbox_events SET failed_at=now(),last_error=$2,claim_owner=NULL,claim_until=NULL WHERE id=$1 AND claim_owner=$3`, row.ID, message, p.PublisherID)
		return err
	}
	backoff := row.AttemptCount * row.AttemptCount
	_, err := p.Pool.Exec(ctx, `UPDATE public.platform_outbox_events SET available_at=now()+$2::interval,last_error=$3,claim_owner=NULL,claim_until=NULL WHERE id=$1 AND claim_owner=$4`, row.ID, fmt.Sprintf("%d seconds", backoff), message, p.PublisherID)
	return err
}

type EventConsumer struct {
	Pool        *pgxpool.Pool
	Redis       redis.UniversalClient
	Stream      string
	Group       string
	Consumer    string
	StartID     string
	MaxAttempts int
	MinIdle     time.Duration
}

func NewEventConsumer(pool *pgxpool.Pool, client redis.UniversalClient, group, consumer string) *EventConsumer {
	return &EventConsumer{Pool: pool, Redis: client, Stream: EventStream, Group: group, Consumer: consumer, StartID: "0", MaxAttempts: 5, MinIdle: 30 * time.Second}
}

func (c *EventConsumer) EnsureGroup(ctx context.Context) error {
	if c == nil || c.Redis == nil {
		return errors.New("event consumer is unavailable")
	}
	startID := c.StartID
	if startID == "" {
		startID = "0"
	}
	err := c.Redis.XGroupCreateMkStream(ctx, c.Stream, c.Group, startID).Err()
	if err != nil && !strings.Contains(strings.ToUpper(err.Error()), "BUSYGROUP") {
		return err
	}
	return nil
}

func (c *EventConsumer) ConsumeOnce(ctx context.Context, count int64) (int, error) {
	if count < 1 {
		count = 10
	}
	if count > 100 {
		count = 100
	}
	if err := c.EnsureGroup(ctx); err != nil {
		return 0, err
	}
	messages, _, err := c.Redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: c.Stream, Group: c.Group, Consumer: c.Consumer, MinIdle: c.MinIdle, Start: "0-0", Count: count}).Result()
	if err != nil {
		return 0, err
	}
	newMessages, err := c.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: c.Group, Consumer: c.Consumer, Streams: []string{c.Stream, ">"}, Count: count, Block: time.Millisecond}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}
	for _, stream := range newMessages {
		messages = append(messages, stream.Messages...)
	}
	processed := 0
	for _, message := range messages {
		if err := c.process(ctx, message); err != nil {
			poison, recordErr := c.recordProcessingFailure(ctx, message.ID, err)
			if recordErr != nil {
				return processed, recordErr
			}
			if poison {
				if ackErr := c.Redis.XAck(ctx, c.Stream, c.Group, message.ID).Err(); ackErr != nil {
					return processed, ackErr
				}
				continue
			}
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (c *EventConsumer) process(ctx context.Context, message redis.XMessage) error {
	eventValue, ok := message.Values["event"].(string)
	if !ok || eventValue == "" {
		return c.recordFailureAndAck(ctx, message.ID, "event_invalid")
	}
	var event EventEnvelope
	if err := json.Unmarshal([]byte(eventValue), &event); err != nil || event.EventID == "" || event.EventType == "" {
		return c.recordFailureAndAck(ctx, message.ID, "event_invalid")
	}
	tx, err := c.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	if event.AggregateSequence > 0 && event.AggregateType != "" && event.AggregateID != "" {
		var lastSequence int
		if err := tx.QueryRow(ctx, `SELECT last_sequence FROM public.platform_consumer_heads WHERE consumer_group=$1 AND aggregate_type=$2 AND aggregate_id=$3 FOR UPDATE`, c.Group, event.AggregateType, event.AggregateID).Scan(&lastSequence); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		} else if err == nil && lastSequence > 0 && event.AggregateSequence > lastSequence+1 {
			return fmt.Errorf("consumer aggregate sequence gap: have %d, received %d", lastSequence, event.AggregateSequence)
		}
	}
	defer tx.Rollback(ctx)
	var inboxID int64
	err = tx.QueryRow(ctx, `INSERT INTO public.platform_consumer_inbox(consumer_group,event_id,result) VALUES($1,$2,$3) ON CONFLICT(consumer_group,event_id) DO NOTHING RETURNING id`, c.Group, event.EventID, []byte(eventValue)).Scan(&inboxID)
	if errors.Is(err, pgx.ErrNoRows) {
		return c.Redis.XAck(ctx, c.Stream, c.Group, message.ID).Err()
	}
	if err != nil {
		return err
	}
	effectID := "consumer_effect_" + stableDigest(c.Group+":"+event.EventID)
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_consumer_effects(id,consumer_group,event_id,effect_type,aggregate_type,aggregate_id,aggregate_sequence,correlation_id,fluctlight_id,payload_digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,encode(digest($10,'sha256'),'hex')) ON CONFLICT(id) DO NOTHING`, effectID, c.Group, event.EventID, consumerEffectType(c.Group), event.AggregateType, event.AggregateID, event.AggregateSequence, event.CorrelationID, nullable(event.FluctlightID), eventValue); err != nil {
		return err
	}
	if event.AggregateType != "" && event.AggregateID != "" {
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_consumer_heads(consumer_group,aggregate_type,aggregate_id,last_sequence) VALUES($1,$2,$3,$4) ON CONFLICT(consumer_group,aggregate_type,aggregate_id) DO UPDATE SET last_sequence=GREATEST(public.platform_consumer_heads.last_sequence,excluded.last_sequence),updated_at=now()`, c.Group, event.AggregateType, event.AggregateID, event.AggregateSequence); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return c.Redis.XAck(ctx, c.Stream, c.Group, message.ID).Err()
}

func consumerEffectType(group string) string {
	switch group {
	case "bff-notifications":
		return "notification"
	case "cache-projections":
		return "projection"
	default:
		return "observed"
	}
}

func (c *EventConsumer) recordFailureAndAck(ctx context.Context, streamID, code string) error {
	_, err := c.Pool.Exec(ctx, `INSERT INTO public.platform_consumer_failures(id,consumer_group,event_id,stream_id,attempt,max_attempts,status,error_code,details) VALUES($1,$2,$3,$4,1,$5,'poison',$6,'{}') ON CONFLICT(id) DO NOTHING`, "consumer_failure_"+stableDigest(c.Group+":"+streamID), c.Group, streamID, streamID, c.MaxAttempts, code)
	if err != nil {
		return err
	}
	return c.Redis.XAck(ctx, c.Stream, c.Group, streamID).Err()
}

func (c *EventConsumer) recordProcessingFailure(ctx context.Context, streamID string, cause error) (bool, error) {
	message := strings.TrimSpace(cause.Error())
	if len(message) > 512 {
		message = message[:512]
	}
	id := "consumer_failure_" + stableDigest(c.Group+":"+streamID)
	var attempt int
	if err := c.Pool.QueryRow(ctx, `SELECT attempt FROM public.platform_consumer_failures WHERE id=$1`, id).Scan(&attempt); errors.Is(err, pgx.ErrNoRows) {
		attempt = 1
		_, err = c.Pool.Exec(ctx, `INSERT INTO public.platform_consumer_failures(id,consumer_group,event_id,stream_id,attempt,max_attempts,status,error_code,details) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, c.Group, streamID, streamID, attempt, c.MaxAttempts, failureStatus(attempt, c.MaxAttempts), "consumer_processing_failed", jsonBytes(map[string]any{"error": message}))
		return attempt >= c.MaxAttempts, err
	} else if err != nil {
		return false, err
	}
	attempt++
	_, err := c.Pool.Exec(ctx, `UPDATE public.platform_consumer_failures SET attempt=$2,status=$3,error_code='consumer_processing_failed',details=$4 WHERE id=$1`, id, attempt, failureStatus(attempt, c.MaxAttempts), jsonBytes(map[string]any{"error": message}))
	return attempt >= c.MaxAttempts, err
}

func failureStatus(attempt, maxAttempts int) string {
	if attempt >= maxAttempts {
		return "poison"
	}
	return "retryable"
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func stableDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", digest[:])[:32]
}

func payloadSequence(payload []byte) int {
	var value map[string]any
	if json.Unmarshal(payload, &value) != nil {
		return 0
	}
	for _, key := range []string{"aggregate_sequence", "sequence", "revision"} {
		if raw, ok := value[key]; ok {
			switch number := raw.(type) {
			case float64:
				if number > 0 && number == float64(int(number)) {
					return int(number)
				}
			case json.Number:
				parsed, err := number.Int64()
				if err == nil && parsed > 0 {
					return int(parsed)
				}
			}
		}
	}
	return 0
}

func jsonBytes(value any) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}
