package platform

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func TestEventEnvelopeRoundTrip(t *testing.T) {
	want := EventEnvelope{EventID: "event-1", EventType: "cognition.fact.created", SchemaVersion: "event.v1", AggregateType: "fluctlight", AggregateID: "fl-1", CausationID: "turn-1", CorrelationID: "turn:1", OccurredAt: time.Unix(10, 0).UTC(), Payload: json.RawMessage(`{"value":true}`)}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got EventEnvelope
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got.EventID != want.EventID || got.EventType != want.EventType || string(got.Payload) != string(want.Payload) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func TestRedisConsumerPublishesAndDeduplicates(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	// The database-backed portion is intentionally opt-in so normal unit runs
	// remain hermetic. CI/acceptance sets GO_CORE_TEST_DATABASE_URL for the
	// PostgreSQL transaction assertions.
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	group := "test-group-" + stableDigest(time.Now().UTC().String())
	eventID := "test-outbox-" + stableDigest(time.Now().UTC().String())
	_, err = pool.Exec(ctx, `INSERT INTO public.platform_outbox_events(id,kind,aggregate_type,aggregate_id,causation_id,correlation_id,idempotency_key,payload,attempt_policy) VALUES($1,'test.event','test',$1,'cause','corr',$1,'{}','{}')`, eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM public.platform_consumer_effects WHERE event_id=$1`, eventID)
	defer pool.Exec(ctx, `DELETE FROM public.platform_consumer_inbox WHERE event_id=$1`, eventID)
	defer pool.Exec(ctx, `DELETE FROM public.platform_outbox_events WHERE id=$1`, eventID)
	consumer := NewEventConsumer(pool, client, group, "test-consumer")
	consumer.StartID = "$"
	consumer.MinIdle = time.Hour
	if err := consumer.EnsureGroup(ctx); err != nil {
		t.Fatal(err)
	}
	publisher := NewOutboxPublisher(pool, client, "test-publisher")
	var targetPublished bool
	for attempt := 0; attempt < 20 && !targetPublished; attempt++ {
		if _, err := publisher.PublishOnce(ctx, 100); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT published_at IS NOT NULL FROM public.platform_outbox_events WHERE id=$1`, eventID).Scan(&targetPublished); err != nil {
			t.Fatal(err)
		}
	}
	if !targetPublished {
		t.Fatal("test outbox event was not published")
	}
	processed, err := consumer.ConsumeOnce(ctx, 10)
	if err != nil || processed < 1 {
		t.Fatalf("ConsumeOnce() = %d, %v", processed, err)
	}
	duplicate, _ := json.Marshal(EventEnvelope{EventID: eventID, EventType: "test.event", SchemaVersion: "event.v1", AggregateType: "test", AggregateID: eventID, CausationID: "cause", CorrelationID: "corr", OccurredAt: time.Now().UTC(), Payload: json.RawMessage(`{}`)})
	if err := client.XAdd(ctx, &redis.XAddArgs{Stream: EventStream, Values: map[string]any{"event": string(duplicate)}}).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := consumer.ConsumeOnce(ctx, 10); err != nil {
		t.Fatal(err)
	}
	var inboxCount, effectCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.platform_consumer_inbox WHERE consumer_group=$1 AND event_id=$2`, group, eventID).Scan(&inboxCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.platform_consumer_effects WHERE consumer_group=$1 AND event_id=$2`, group, eventID).Scan(&effectCount); err != nil {
		t.Fatal(err)
	}
	if inboxCount != 1 || effectCount != 1 {
		t.Fatalf("inbox/effect = %d/%d, want 1/1", inboxCount, effectCount)
	}
}

func TestRedisConsumerRecordsPoisonEvent(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	server, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	group := "poison-group-" + stableDigest(time.Now().UTC().String())
	consumer := NewEventConsumer(pool, client, group, "poison-consumer")
	consumer.StartID = "$"
	if err := consumer.EnsureGroup(ctx); err != nil {
		t.Fatal(err)
	}
	messageID, err := client.XAdd(ctx, &redis.XAddArgs{Stream: EventStream, Values: map[string]any{"event": "not-json"}}).Result()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := consumer.ConsumeOnce(ctx, 10); err != nil {
		t.Fatal(err)
	}
	var failureCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.platform_consumer_failures WHERE consumer_group=$1 AND stream_id=$2`, group, messageID).Scan(&failureCount); err != nil {
		t.Fatal(err)
	}
	if failureCount != 1 {
		t.Fatalf("poison failure count = %d, want 1", failureCount)
	}
}

func TestRedisConsumerReclaimsPendingDelivery(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	server, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	group := "reclaim-group-" + stableDigest(time.Now().UTC().String())
	if err := client.XGroupCreateMkStream(ctx, EventStream, group, "$").Err(); err != nil {
		t.Fatal(err)
	}
	eventID := "reclaim-event-" + stableDigest(time.Now().UTC().String())
	event, _ := json.Marshal(EventEnvelope{EventID: eventID, EventType: "reclaim.event", SchemaVersion: "event.v1", AggregateType: "test", AggregateID: eventID, CausationID: "cause", CorrelationID: "corr", OccurredAt: time.Now().UTC(), Payload: json.RawMessage(`{}`)})
	streamID, err := client.XAdd(ctx, &redis.XAddArgs{Stream: EventStream, Values: map[string]any{"event": string(event)}}).Result()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: group, Consumer: "dead-consumer", Streams: []string{EventStream, ">"}, Count: 1, Block: time.Millisecond}).Result(); err != nil {
		t.Fatal(err)
	}
	consumer := NewEventConsumer(pool, client, group, "live-consumer")
	consumer.MinIdle = 0
	if _, err := consumer.ConsumeOnce(ctx, 10); err != nil {
		t.Fatal(err)
	}
	var effectCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.platform_consumer_effects WHERE consumer_group=$1 AND event_id=$2`, group, eventID).Scan(&effectCount); err != nil {
		t.Fatal(err)
	}
	if effectCount != 1 {
		t.Fatalf("reclaimed effect count = %d, want 1 (stream %s)", effectCount, streamID)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM public.platform_consumer_effects WHERE event_id=$1`, eventID)
	_, _ = pool.Exec(ctx, `DELETE FROM public.platform_consumer_inbox WHERE event_id=$1`, eventID)
}
