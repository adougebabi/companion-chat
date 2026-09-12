package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRawHistoryNormalizationIsBoundedAndSourceStrict(t *testing.T) {
	if got := normalizeRawHistoryLimit(0); got != defaultRawHistoryLimit {
		t.Fatalf("default limit=%d", got)
	}
	if got := normalizeRawHistoryLimit(maxRawHistoryLimit + 1); got != maxRawHistoryLimit {
		t.Fatalf("max limit=%d", got)
	}
	refs, err := normalizeRawHistorySourceRefs([]string{" message:a ", "fact:b", "message:a", "outcome:c"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"message:a", "fact:b", "outcome:c"}; !reflect.DeepEqual(refs, want) {
		t.Fatalf("refs=%v want=%v", refs, want)
	}
	for _, invalid := range [][]string{nil, {"memory:a"}, {""}} {
		if _, err := normalizeRawHistorySourceRefs(invalid); err == nil {
			t.Fatalf("invalid refs accepted: %v", invalid)
		}
	}
}

func TestRawHistoryNormalizationKeepsUniqueChronologicalEvents(t *testing.T) {
	base := time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC)
	events := []RawHistoryEvent{
		{SourceRef: "fact:b", OccurredAt: base.Add(2 * time.Hour)},
		{SourceRef: "message:a", OccurredAt: base},
		{SourceRef: "fact:b", OccurredAt: base.Add(2 * time.Hour)},
		{SourceRef: "outcome:c", OccurredAt: base.Add(time.Hour)},
	}
	got := normalizeRawHistoryEvents(events, 3)
	refs := make([]string, 0, len(got))
	for _, event := range got {
		refs = append(refs, event.SourceRef)
	}
	if want := []string{"message:a", "outcome:c", "fact:b"}; !reflect.DeepEqual(refs, want) {
		t.Fatalf("chronological refs=%v want=%v", refs, want)
	}
}

func TestRawHistoryNormalizationBoundsThousandEvents(t *testing.T) {
	base := time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC)
	events := make([]RawHistoryEvent, 0, 1000)
	for index := 999; index >= 0; index-- {
		events = append(events, RawHistoryEvent{
			SourceRef:  "fact:" + string(rune(0x1000+index)),
			OccurredAt: base.Add(time.Duration(index) * time.Second),
		})
	}
	got := normalizeRawHistoryEvents(events, defaultRawHistoryLimit)
	if len(got) != defaultRawHistoryLimit {
		t.Fatalf("bounded events=%d want=%d", len(got), defaultRawHistoryLimit)
	}
	for index := 1; index < len(got); index++ {
		if got[index].OccurredAt.Before(got[index-1].OccurredAt) {
			t.Fatalf("events are not chronological at %d", index)
		}
	}
}

// Authored in S02 and executed only by the S12 isolated-PostgreSQL gate.
func TestPostgresRawHistoryReaderCombinesAuthoritiesAndSearchesMessages(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "raw-owner", "raw-fluctlight", "raw-conversation"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	factID, actionID, outcomeID := "raw-fact", "raw-action", "raw-outcome"
	for _, seed := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'raw history')`, []any{conversationID, ownerID}},
		{`INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,2)`, []any{conversationID}},
		{`INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, []any{conversationID, ownerID, fluctlightID}},
		{`INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,2,1)`, []any{fluctlightID}},
		{`INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES($1,$2,1,'conversation.turn',$3,'raw-turn','turn:raw-turn','raw-fact',now()-interval '3 minutes','processed',now()-interval '2 minutes')`, []any{factID, fluctlightID, jsonBytes(map[string]any{"actor_id": ownerID, "conversation_id": conversationID, "turn_id": "raw-turn", "text": "flight tomorrow"})}},
		{`INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,turn_id,source_fact_id,correlation_id,created_at) VALUES('raw-message',$1,1,$2,'user','flight tomorrow','[]','raw-message','raw-turn',$3,'turn:raw-turn',now()-interval '3 minutes')`, []any{conversationID, ownerID, factID}},
		{`INSERT INTO public.cognition_frozen_actions(id,decision_id,inbox_id,fluctlight_id,action_type,payload,state_revision,provider_request_id,status,frozen_at,completed_at) VALUES($1,'raw-decision',$2,$3,'reply','{}',0,'raw-provider','completed',now()-interval '2 minutes',now()-interval '1 minute')`, []any{actionID, factID, fluctlightID}},
		{`INSERT INTO public.cognition_action_outcomes(id,fluctlight_id,action_id,call_id,capability_name,status,success_boundary,expected,observed,goal_refs,intention_refs,evidence_refs,context_references,revision,request_digest,occurred_at) VALUES($1,$2,$3,'raw-call','scene_event','completed','life_context_committed','{}','{"scene":"balcony"}','[]','[]',$4,'{}',1,$5,now()-interval '1 minute')`, []any{outcomeID, fluctlightID, actionID, jsonBytes([]string{factID}), strings.Repeat("a", 64)}},
	} {
		if _, err := repository.Pool().Exec(ctx, seed.query, seed.args...); err != nil {
			t.Fatal(err)
		}
	}

	reader := NewRawHistoryReader(repository)
	events, err := reader.Recent(ctx, RawHistoryQuery{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].SourceRef != "fact:"+factID || events[1].SourceRef != "message:raw-message" || events[2].SourceRef != "outcome:"+outcomeID {
		t.Fatalf("raw events=%#v", events)
	}
	search, err := reader.Search(ctx, RawHistorySearchQuery{AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, Query: "flight", Limit: 5})
	if err != nil || len(search) != 1 || search[0].SourceRef != "message:raw-message" {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	if _, err := reader.Recent(ctx, RawHistoryQuery{AuthorizationActorID: "foreign-owner", FluctlightID: fluctlightID, ConversationID: conversationID}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("foreign raw history err=%v", err)
	}
}
