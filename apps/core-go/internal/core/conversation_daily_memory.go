package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const conversationDailyMemorySchemaVersion = "conversation-daily-memory.v1"

type conversationDailyEpisodeResponse struct {
	SchemaVersion string `json:"schema_version"`
	Episode       string `json:"episode"`
}

type conversationDailySource struct {
	ID, Summary, EndingState, Fingerprint string
	Revision                              int
	StartedAt, EndedAt                    time.Time
	FromSequence, ToSequence              int
	SourceDigest                          string
	SourceRefs                            []string
	OpenThreads, CoreEvents               []any
}

func (a *App) enqueueConversationDailyMemoryTx(ctx context.Context, tx pgx.Tx, fluctlightID, conversationID, localDate, timezone string) error {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return err
	}
	day, err := time.ParseInLocation("2006-01-02", localDate, location)
	if err != nil {
		return err
	}
	due := day.AddDate(0, 0, 1).Add(conversationSegmentIdleDelay).UTC()
	if due.Before(a.now().UTC()) {
		due = a.now().UTC()
	}
	identity := stableDigest(fluctlightID + "\x1f" + conversationID + "\x1f" + localDate + "\x1f" + timezone)
	intentID := "conversation_daily_memory_intent:" + identity
	payload := map[string]any{"intent_id": intentID, "fluctlight_id": fluctlightID, "conversation_id": conversationID, "local_date": localDate, "timezone": timezone, "day_start_utc": day.UTC().Format(time.RFC3339Nano), "day_end_utc": day.AddDate(0, 0, 1).UTC().Format(time.RFC3339Nano)}
	_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,next_attempt_at)
		VALUES($1,$2,'lifecycle','conversation.daily_memory',$3,'pending',$4)
		ON CONFLICT(intent_id) DO UPDATE SET status=CASE WHEN public.platform_workflow_intents.status IN ('completed','failed','dead_letter') THEN 'retry' ELSE public.platform_workflow_intents.status END,
		 next_attempt_at=LEAST(COALESCE(public.platform_workflow_intents.next_attempt_at,excluded.next_attempt_at),excluded.next_attempt_at)`, intentID, "conversation_daily_memory:"+identity, jsonBytes(payload), due)
	return err
}

func readConversationDailySources(ctx context.Context, tx pgx.Tx, fluctlightID, conversationID, localDate, timezone string) ([]conversationDailySource, error) {
	rows, err := tx.Query(ctx, `SELECT id,summary,COALESCE(ending_state,''),revision,public.conversation_summary_source_fingerprint(id),
		COALESCE(started_at,created_at),COALESCE(ended_at,completed_at),from_sequence,to_sequence,source_digest,source_message_refs,open_threads,core_events
		FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND local_date=$3::date AND timezone=$4
		AND status IN ('active','consolidated') ORDER BY from_sequence,revision`, fluctlightID, conversationID, localDate, timezone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]conversationDailySource, 0)
	for rows.Next() {
		var item conversationDailySource
		var refs, openThreads, coreEvents []byte
		if err := rows.Scan(&item.ID, &item.Summary, &item.EndingState, &item.Revision, &item.Fingerprint, &item.StartedAt, &item.EndedAt, &item.FromSequence, &item.ToSequence, &item.SourceDigest, &refs, &openThreads, &coreEvents); err != nil {
			return nil, err
		}
		item.SourceRefs = conversationSummarySourceRefValues(decodeArray(refs))
		item.OpenThreads, item.CoreEvents = decodeArray(openThreads), decodeArray(coreEvents)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for _, item := range result {
		length := item.ToSequence - item.FromSequence + 1
		if length < 1 || length > conversationSummaryMaxMessages {
			return nil, errors.New("conversation_daily_memory_source_window_invalid")
		}
		messages, err := readConversationSummaryMessages(ctx, tx, conversationID, item.FromSequence, item.ToSequence, length)
		if err != nil {
			return nil, err
		}
		if len(messages) != length || !sameStringSlice(conversationSummarySourceRefs(messages), item.SourceRefs) || conversationSummarySourceDigest(messages) != item.SourceDigest {
			return nil, errors.New("conversation_daily_memory_source_drift")
		}
	}
	return result, nil
}

func dailySourceFingerprint(sources []conversationDailySource) string {
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		parts = append(parts, fmt.Sprintf("%s:%d:%s", source.ID, source.Revision, source.Fingerprint))
	}
	return stableDigest(strings.Join(parts, "\x1f"))
}

func (a *App) ProcessConversationDailyMemoryIntent(ctx context.Context, intentID string) (map[string]any, error) {
	var status string
	var raw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='conversation.daily_memory'`, intentID).Scan(&status, &raw); err != nil {
		return nil, err
	}
	payload := decodeObject(raw)
	fluctlightID, conversationID := stringValue(payload["fluctlight_id"]), stringValue(payload["conversation_id"])
	localDate, timezone := stringValue(payload["local_date"]), stringValue(payload["timezone"])
	if fluctlightID == "" || conversationID == "" || localDate == "" || timezone == "" {
		return nil, errors.New("conversation_daily_memory_intent_invalid")
	}
	if status == "completed" {
		return map[string]any{"status": "completed", "replayed": true}, nil
	}
	return a.runConversationDailyMemoryIntentWork(ctx, fluctlightID, func(runCtx context.Context) (map[string]any, error) {
		return a.processConversationDailyMemoryIntent(runCtx, intentID, fluctlightID)
	})
}

func (a *App) runConversationDailyMemoryIntentWork(ctx context.Context, owner string, work func(context.Context) (map[string]any, error)) (map[string]any, error) {
	return a.runBackgroundLogicalWork(ctx, owner, "conversation_daily_memory", work)
}

func (a *App) processConversationDailyMemoryIntent(ctx context.Context, intentID, expectedFluctlightID string) (map[string]any, error) {
	var status string
	var raw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='conversation.daily_memory'`, intentID).Scan(&status, &raw); err != nil {
		return nil, err
	}
	payload := decodeObject(raw)
	fluctlightID, conversationID := stringValue(payload["fluctlight_id"]), stringValue(payload["conversation_id"])
	localDate, timezone := stringValue(payload["local_date"]), stringValue(payload["timezone"])
	if fluctlightID == "" || fluctlightID != expectedFluctlightID || conversationID == "" || localDate == "" || timezone == "" {
		return nil, errors.New("conversation_daily_memory_intent_invalid")
	}
	if status == "completed" {
		return map[string]any{"status": "completed", "replayed": true}, nil
	}
	dayEnd, err := time.Parse(time.RFC3339Nano, stringValue(payload["day_end_utc"]))
	if err != nil {
		return nil, err
	}
	if a.now().UTC().Before(dayEnd.Add(conversationSegmentIdleDelay)) {
		return nil, errors.New("conversation_daily_memory_day_open")
	}
	var sources []conversationDailySource
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var err error
		sources, err = readConversationDailySources(ctx, tx, fluctlightID, conversationID, localDate, timezone)
		if err != nil {
			return err
		}
		var pending bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_type='conversation.segment'
			AND payload->>'conversation_id'=$1 AND payload->>'local_date'=$2 AND payload->>'timezone'=$3 AND status IN ('pending','retry','started','running'))`, conversationID, localDate, timezone).Scan(&pending); err != nil {
			return err
		}
		if pending {
			return errors.New("conversation_daily_memory_segment_pending")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return map[string]any{"status": "no_source"}, nil
	}
	if len(sources) > 64 {
		return nil, errors.New("conversation_daily_memory_source_limit")
	}
	var priorFingerprint string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT source_fingerprint FROM public.conversation_daily_memories WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND local_date=$3::date AND timezone=$4`, fluctlightID, conversationID, localDate, timezone).Scan(&priorFingerprint); err == nil && priorFingerprint == dailySourceFingerprint(sources) {
		return map[string]any{"status": "completed", "replayed": true}, nil
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if a.Provider == nil {
		return nil, errors.New("conversation_daily_memory_provider_unavailable")
	}
	response, err := a.RunConversationDailyEpisodeTask(WithProviderCorrelation(ctx, "conversation-daily:"+intentID+":"+dailySourceFingerprint(sources)), localDate, timezone, sources)
	if err != nil {
		return nil, err
	}
	if response.SchemaVersion != conversationDailyMemorySchemaVersion || strings.TrimSpace(response.Episode) == "" || len([]rune(response.Episode)) > conversationSummaryMaxRunes {
		return nil, errors.New("conversation_daily_memory_response_invalid")
	}
	var result map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var locked string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.conversations WHERE id=$1 FOR UPDATE`, conversationID).Scan(&locked); err != nil {
			return err
		}
		var liveStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1 FOR UPDATE`, intentID).Scan(&liveStatus); err != nil {
			return err
		}
		if liveStatus == "completed" {
			result = map[string]any{"status": "completed", "replayed": true}
			return nil
		}
		live, err := readConversationDailySources(ctx, tx, fluctlightID, conversationID, localDate, timezone)
		if err != nil {
			return err
		}
		fingerprint := dailySourceFingerprint(sources)
		if fingerprint != dailySourceFingerprint(live) {
			return errors.New("conversation_daily_memory_source_drift")
		}
		var ownerActorID string
		if err := tx.QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerActorID); err != nil {
			return err
		}
		evidence := make([]string, 0, len(sources))
		frozen := make(map[string]FrozenMemoryEvidenceSource, len(sources))
		for _, source := range sources {
			ref := "conversation_summary:" + source.ID
			evidence = append(evidence, ref)
			frozen[ref] = FrozenMemoryEvidenceSource{Kind: "conversation_summary", ID: source.ID, Revision: source.Revision, Fingerprint: source.Fingerprint}
		}
		command := PreparedMemoryMutation{
			SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate,
			OwnerFluctlightID: fluctlightID, OwnerActorID: ownerActorID, ActorID: fluctlightID,
			ConversationID: conversationID, ActorRefs: []string{ownerActorID, fluctlightID},
			EventRefs: []string{"conversation-day:" + localDate + "@" + timezone}, EvidenceRefs: evidence,
			FrozenEvidenceSources: frozen, Semantic: &MemorySemanticInput{Type: "episodic", Content: response.Episode, Confidence: 0.8, Importance: 0.5, EmotionalSignificance: 0.3},
			Visibility: "private", OccurredAt: sources[len(sources)-1].EndedAt, SourceFactID: intentID,
			ProposalID: "conversation_daily:" + stableDigest(intentID+":"+fingerprint), SemanticReason: "same-conversation local-day episode consolidation",
			IdempotencyKey: "conversation-daily:" + intentID + ":" + fingerprint,
		}
		var previousID string
		var previousRevision int
		err = tx.QueryRow(ctx, `SELECT d.memory_id,m.revision FROM public.conversation_daily_memories d JOIN public.memories m ON m.id=d.memory_id WHERE d.owner_fluctlight_id=$1 AND d.conversation_id=$2 AND d.local_date=$3::date AND d.timezone=$4 FOR UPDATE`, fluctlightID, conversationID, localDate, timezone).Scan(&previousID, &previousRevision)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if previousID != "" {
			command.Operation = MemoryRevise
			command.Target = &MemoryTarget{Ref: "conversation_daily:" + stableDigest(previousID), MemoryID: previousID, ExpectedRevision: previousRevision}
			command.ReplaceEvidence = true
		}
		command.RequestDigest = memoryCommandDigest(command)
		applied, err := a.applyMemoryCommandTx(ctx, tx, command)
		if err != nil {
			return err
		}
		if applied.Disposition != "applied" && applied.Disposition != "no_change" {
			return errors.New("conversation_daily_memory_not_applied")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_daily_memories(owner_fluctlight_id,conversation_id,local_date,timezone,memory_id,source_fingerprint)
			VALUES($1,$2,$3::date,$4,$5,$6)
			ON CONFLICT(owner_fluctlight_id,conversation_id,local_date,timezone) DO UPDATE SET memory_id=excluded.memory_id,source_fingerprint=excluded.source_fingerprint,updated_at=now()`, fluctlightID, conversationID, localDate, timezone, applied.MemoryID, fingerprint); err != nil {
			return err
		}
		for _, source := range sources {
			if _, err := tx.Exec(ctx, `UPDATE public.conversation_summaries SET status='consolidated',consolidated_into_memory_id=$2 WHERE id=$1 AND status IN ('active','consolidated') AND (status IS DISTINCT FROM 'consolidated' OR consolidated_into_memory_id IS DISTINCT FROM $2)`, source.ID, applied.MemoryID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed',completed_at=now() WHERE intent_id=$1`, intentID); err != nil {
			return err
		}
		result = map[string]any{"status": "completed", "memory_id": applied.MemoryID, "source_count": len(sources)}
		return nil
	})
	return result, err
}

// The covered message refs stay Core-only. They let final wire selection omit
// recent raw turns only when their daily memory actually fits the wire budget.
func (a *App) attachDailyMemoryMessageRefs(ctx context.Context, projection ContextProjection, memory *WorkingMemory) error {
	if memory == nil || projection.ConversationID == "" {
		return nil
	}
	recent := make(map[string]struct{}, len(projection.RecentMessages))
	for _, item := range projection.RecentMessages {
		if id := stringValue(item["id"]); id != "" {
			recent["message:"+id] = struct{}{}
		}
	}
	if len(recent) == 0 {
		return nil
	}
	byRef := make(map[string]map[string]any)
	for _, item := range append(append([]map[string]any(nil), projection.Memories...), projection.ResidentMemories...) {
		if ref := stringValue(item["ref"]); ref != "" {
			byRef[ref] = item
		}
	}
	cache := make(map[string][]string)
	attach := func(fragments []PromptFragment) error {
		for index := range fragments {
			row := byRef[stringValue(mapValue(fragments[index].Content)["ref"])]
			if row == nil {
				continue
			}
			for _, raw := range arrayValue(row["effective_source_refs"]) {
				ref := stringValue(raw)
				if !strings.HasPrefix(ref, "conversation_summary:") {
					continue
				}
				id := strings.TrimPrefix(ref, "conversation_summary:")
				messageRefs, cached := cache[id]
				if !cached {
					var encoded []byte
					err := a.DB.Pool().QueryRow(ctx, `SELECT source_message_refs FROM public.conversation_summaries WHERE id=$1 AND owner_fluctlight_id=$2 AND conversation_id=$3 AND status='consolidated'`, id, projection.FluctlightID, projection.ConversationID).Scan(&encoded)
					if errors.Is(err, pgx.ErrNoRows) {
						cache[id] = nil
						continue
					}
					if err != nil {
						return err
					}
					messageRefs = conversationSummarySourceRefValues(decodeArray(encoded))
					cache[id] = messageRefs
				}
				for _, messageRef := range messageRefs {
					if _, selected := recent[messageRef]; selected {
						fragments[index].SourceRefs = append(fragments[index].SourceRefs, messageRef)
					}
				}
			}
		}
		return nil
	}
	if err := attach(memory.Retrieved); err != nil {
		return err
	}
	return attach(memory.Resident)
}

// Legacy fixed chunks have no source date. Classify only from their verified
// raw window; a cross-day chunk is rebuilt as day-bounded segments.
func (a *App) ReconcileLegacyConversationSummaries(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_digest,source_message_refs
		FROM public.conversation_summaries WHERE status='active' AND local_date IS NULL ORDER BY completed_at,id LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type legacy struct {
		id, fluctlight, conversation, digest string
		from, to                             int
		refs                                 []string
	}
	items := make([]legacy, 0)
	for rows.Next() {
		var item legacy
		var encoded []byte
		if err := rows.Scan(&item.id, &item.fluctlight, &item.conversation, &item.from, &item.to, &item.digest, &encoded); err != nil {
			rows.Close()
			return 0, err
		}
		item.refs = conversationSummarySourceRefValues(decodeArray(encoded))
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, item := range items {
		if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			var locked string
			if err := tx.QueryRow(ctx, `SELECT id FROM public.conversations WHERE id=$1 FOR UPDATE`, item.conversation).Scan(&locked); err != nil {
				return err
			}
			var status string
			if err := tx.QueryRow(ctx, `SELECT status FROM public.conversation_summaries WHERE id=$1 FOR UPDATE`, item.id).Scan(&status); err != nil {
				return err
			}
			if status != "active" {
				return nil
			}
			window := item.to - item.from + 1
			if window < 1 || window > conversationSummaryMaxMessages {
				return errors.New("legacy_conversation_summary_window_invalid")
			}
			messages, err := readConversationSummaryMessages(ctx, tx, item.conversation, item.from, item.to, window)
			if err != nil {
				return err
			}
			if len(messages) != window || !sameStringSlice(conversationSummarySourceRefs(messages), item.refs) || conversationSummarySourceDigest(messages) != item.digest {
				return errors.New("legacy_conversation_summary_source_drift")
			}
			var zone string
			if err := tx.QueryRow(ctx, `SELECT COALESCE(identity->>'timezone','Asia/Shanghai') FROM public.fluctlights WHERE id=$1`, item.fluctlight).Scan(&zone); err != nil {
				return err
			}
			location, err := time.LoadLocation(zone)
			if err != nil {
				return err
			}
			day := ""
			crossesDay := false
			for _, message := range messages {
				if message.Kind != "user" {
					continue
				}
				current := message.CreatedAt.In(location).Format("2006-01-02")
				if day == "" {
					day = current
				} else if day != current {
					crossesDay = true
					break
				}
			}
			if day == "" {
				day = messages[0].CreatedAt.In(location).Format("2006-01-02")
			}
			if crossesDay {
				if _, err := tx.Exec(ctx, `UPDATE public.conversation_summaries SET status='invalidated' WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND from_sequence >= $3 AND status='active'`, item.fluctlight, item.conversation, item.from); err != nil {
					return err
				}
				return a.enqueueConversationSegmentTx(ctx, tx, item.fluctlight, item.conversation, true)
			}
			if _, err := tx.Exec(ctx, `UPDATE public.conversation_summaries SET started_at=$2,ended_at=$3,timezone=$4,local_date=$5::date WHERE id=$1 AND status='active'`, item.id, messages[0].CreatedAt.UTC(), messages[len(messages)-1].CreatedAt.UTC(), zone, day); err != nil {
				return err
			}
			return a.enqueueConversationDailyMemoryTx(ctx, tx, item.fluctlight, item.conversation, day, zone)
		}); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}
