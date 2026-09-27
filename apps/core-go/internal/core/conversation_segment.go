package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	conversationSegmentIdleDelay     = 10 * time.Minute
	conversationSegmentSchemaVersion = "conversation-segment.v1"
	conversationSegmentPromptVersion = "conversation-segment.prompt.v1"
)

type conversationSegmentResponse struct {
	SchemaVersion string   `json:"schema_version"`
	Summary       string   `json:"summary"`
	EndingState   string   `json:"ending_state"`
	OpenThreads   []string `json:"open_threads"`
	CoreEvents    []string `json:"core_events"`
}

func validateConversationSegmentDay(messages []ConversationSummarySourceMessage, timezone, expectedDay string) error {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return errors.New("conversation_segment_timezone_invalid")
	}
	turnDay, segmentDay := "", ""
	for _, message := range messages {
		if message.Kind == "user" {
			turnDay = message.CreatedAt.In(location).Format("2006-01-02")
		}
		if message.Kind == "assistant" {
			if turnDay == "" {
				turnDay = message.CreatedAt.In(location).Format("2006-01-02")
			}
			if segmentDay == "" {
				segmentDay = turnDay
			} else if segmentDay != turnDay {
				return errors.New("conversation_segment_cross_day_window")
			}
		}
	}
	if segmentDay == "" || segmentDay != expectedDay {
		return errors.New("conversation_segment_local_date_stale")
	}
	return nil
}

func (a *App) enqueueConversationSegmentTx(ctx context.Context, tx pgx.Tx, fluctlightID, conversationID string, dueNow bool) error {
	// The conversation head serializes publication. A changed immutable window
	// supersedes an in-flight predecessor; rearming the same window is a no-op.
	var epoch, timezone string
	var idleSince time.Time
	if err := tx.QueryRow(ctx, `SELECT m.id,m.created_at,COALESCE(f.identity->>'timezone','Asia/Shanghai')
		FROM public.conversation_messages m JOIN public.fluctlights f ON f.id=$2
		WHERE m.conversation_id=$1 AND m.kind='user' ORDER BY m.sequence DESC LIMIT 1`, conversationID, fluctlightID).Scan(&epoch, &idleSince, &timezone); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return err
	}
	var covered int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(to_sequence),0) FROM public.conversation_summaries
		WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND status IN ('active','consolidated')`, fluctlightID, conversationID).Scan(&covered); err != nil {
		return err
	}
	// A source correction reopens the earliest invalidated segment. Its old
	// projection remains auditable and the same window receives a new revision.
	var invalidatedFrom *int
	if err := tx.QueryRow(ctx, `SELECT MIN(stale.from_sequence) FROM public.conversation_summaries stale
		WHERE stale.owner_fluctlight_id=$1 AND stale.conversation_id=$2 AND stale.status='invalidated'
		  AND NOT EXISTS (SELECT 1 FROM public.conversation_summaries live
		    WHERE live.owner_fluctlight_id=stale.owner_fluctlight_id AND live.conversation_id=stale.conversation_id
		      AND live.from_sequence=stale.from_sequence AND live.to_sequence=stale.to_sequence
		      AND live.revision>stale.revision AND live.status IN ('active','consolidated'))`, fluctlightID, conversationID).Scan(&invalidatedFrom); err != nil {
		return err
	}
	if invalidatedFrom != nil && *invalidatedFrom <= covered {
		covered = *invalidatedFrom - 1
	}
	var sourceID string
	var sourceSequence int
	if err := tx.QueryRow(ctx, `SELECT id,sequence FROM public.conversation_messages
		WHERE conversation_id=$1 AND kind='assistant' AND sequence>$2 ORDER BY sequence DESC LIMIT 1`, conversationID, covered).Scan(&sourceID, &sourceSequence); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	messages, err := readConversationSummaryMessages(ctx, tx, conversationID, covered+1, sourceSequence, conversationSummaryMaxMessages)
	if err != nil {
		return err
	}
	// A user turn owns the local date of its assistant continuation, even if
	// the reply crosses midnight. Never put two user-start dates in one segment.
	firstDay, turnDay := "", ""
	cut := 0
	tokens := 0
	for index, message := range messages {
		tokens += estimateConversationSummaryTokens(message)
		if message.Kind == "user" {
			turnDay = message.CreatedAt.In(location).Format("2006-01-02")
		}
		if message.Kind == "assistant" {
			if turnDay == "" {
				turnDay = message.CreatedAt.In(location).Format("2006-01-02")
			}
			if firstDay == "" {
				firstDay = turnDay
			} else if turnDay != firstDay {
				break
			}
			cut = index + 1
			if tokens >= conversationSummaryTargetTokens {
				break
			}
		}
	}
	if cut == 0 {
		return nil
	}
	chunk := messages[:cut]
	if firstDay == "" {
		firstDay = chunk[0].CreatedAt.In(location).Format("2006-01-02")
	}
	boundaryKind := "idle"
	clockIdentity := epoch
	if firstDay != idleSince.In(location).Format("2006-01-02") {
		// A new local day seals completed turns from the previous day even
		// while the conversation remains active. Its source identity cannot
		// depend on later user messages from the new day.
		boundaryKind = "day_rollover"
		clockIdentity = "day:" + firstDay + ":" + timezone
	}
	from, to := chunk[0].Sequence, chunk[len(chunk)-1].Sequence
	digest := conversationSummarySourceDigest(chunk)
	identity := stableDigest(strings.Join([]string{fluctlightID, conversationID, clockIdentity, fmt.Sprint(from), fmt.Sprint(to), digest}, "\x1f"))
	intentID := "conversation_segment_intent:" + identity
	due := idleSince.UTC().Add(conversationSegmentIdleDelay)
	if boundaryKind == "day_rollover" || (dueNow && !due.After(a.now().UTC())) {
		due = a.now().UTC()
	}
	payload := map[string]any{
		"intent_id": intentID, "fluctlight_id": fluctlightID, "conversation_id": conversationID,
		"source_message_id": chunk[len(chunk)-1].ID, "source_sequence": to,
		"from_sequence": from, "to_sequence": to, "source_digest": digest,
		"source_message_refs": conversationSummarySourceRefs(chunk),
		"idle_epoch":          epoch, "idle_since": idleSince.UTC().Format(time.RFC3339Nano),
		"local_date": firstDay, "timezone": timezone, "boundary_kind": boundaryKind,
	}
	if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='superseded'
		WHERE intent_type='conversation.segment' AND payload->>'conversation_id'=$1
		  AND intent_id<>$2 AND COALESCE(payload->>'boundary_kind','idle')='idle'
		  AND status IN ('pending','retry','started','running')`, conversationID, intentID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,next_attempt_at)
		VALUES($1,$2,'lifecycle','conversation.segment',$3,'pending',$4)
		ON CONFLICT(intent_id) DO UPDATE SET status='pending',next_attempt_at=excluded.next_attempt_at
		WHERE public.platform_workflow_intents.status='superseded'
		   OR (public.platform_workflow_intents.status='completed' AND EXISTS (
		     SELECT 1 FROM public.conversation_summaries s WHERE s.owner_fluctlight_id=$5 AND s.conversation_id=$6
		       AND s.from_sequence=$7 AND s.to_sequence=$8 AND s.source_digest=$9 AND s.status='invalidated'))`, intentID, "conversation_segment:"+identity, jsonBytes(payload), due, fluctlightID, conversationID, from, to, digest)
	return err
}

// EnsureConversationSegmentIntents catches old fixed-chunk backlog and any
// assistant publication whose post-commit dispatch was interrupted. The normal
// reply transaction creates the ten-minute intent immediately; this is repair.
func (a *App) EnsureConversationSegmentIntents(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT DISTINCT m.conversation_id,m.author_actor_id
		FROM public.conversation_messages m JOIN public.actors actor ON actor.id=m.author_actor_id AND actor.actor_type='fluctlight'
		WHERE m.kind='assistant'
		  AND EXISTS (SELECT 1 FROM public.conversation_messages u WHERE u.conversation_id=m.conversation_id AND u.kind='user')
		  AND NOT EXISTS (SELECT 1 FROM public.platform_workflow_intents i WHERE i.intent_type='conversation.segment'
		    AND i.payload->>'conversation_id'=m.conversation_id AND i.status IN ('pending','retry','started','running'))
		  AND (m.sequence>COALESCE((SELECT MAX(s.to_sequence) FROM public.conversation_summaries s
		    WHERE s.owner_fluctlight_id=m.author_actor_id AND s.conversation_id=m.conversation_id
		      AND s.status IN ('active','consolidated')),0)
		    OR EXISTS (SELECT 1 FROM public.conversation_summaries s WHERE s.owner_fluctlight_id=m.author_actor_id
		      AND s.conversation_id=m.conversation_id AND s.status='invalidated'
		      AND NOT EXISTS (SELECT 1 FROM public.conversation_summaries live WHERE live.owner_fluctlight_id=s.owner_fluctlight_id
		        AND live.conversation_id=s.conversation_id AND live.from_sequence=s.from_sequence AND live.to_sequence=s.to_sequence
		        AND live.revision>s.revision AND live.status IN ('active','consolidated'))))
		ORDER BY m.conversation_id,m.author_actor_id LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type target struct{ conversation, fluctlight string }
	targets := make([]target, 0)
	for rows.Next() {
		var item target
		if err := rows.Scan(&item.conversation, &item.fluctlight); err != nil {
			rows.Close()
			return 0, err
		}
		targets = append(targets, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, item := range targets {
		if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			return a.enqueueConversationSegmentTx(ctx, tx, item.fluctlight, item.conversation, true)
		}); err != nil {
			return 0, err
		}
	}
	return len(targets), nil
}

func (a *App) ProcessConversationSegmentIntent(ctx context.Context, intentID string) (map[string]any, error) {
	var raw []byte
	var status string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,payload FROM public.platform_workflow_intents
		WHERE intent_id=$1 AND intent_type='conversation.segment'`, intentID).Scan(&status, &raw); err != nil {
		return nil, err
	}
	if status == "superseded" || status == "completed" {
		return map[string]any{"status": status}, nil
	}
	payload := decodeObject(raw)
	work := conversationSummaryWork{
		IntentID: intentID, FluctlightID: stringValue(payload["fluctlight_id"]), ConversationID: stringValue(payload["conversation_id"]),
		SourceMessageID: stringValue(payload["source_message_id"]), SourceSequence: intValue(payload["source_sequence"]),
		FromSequence: intValue(payload["from_sequence"]), ToSequence: intValue(payload["to_sequence"]),
		SourceDigest: stringValue(payload["source_digest"]), SourceMessageRefs: conversationSummarySourceRefValues(payload["source_message_refs"]),
	}
	if work.FluctlightID == "" || work.ConversationID == "" || work.ToSequence < work.FromSequence || work.FromSequence < 1 {
		return nil, errors.New("conversation_segment_intent_invalid")
	}
	var currentEpoch string
	var currentAt time.Time
	if err := a.DB.Pool().QueryRow(ctx, `SELECT id,created_at FROM public.conversation_messages
		WHERE conversation_id=$1 AND kind='user' ORDER BY sequence DESC LIMIT 1`, work.ConversationID).Scan(&currentEpoch, &currentAt); err != nil {
		return nil, err
	}
	dayRollover := stringValue(payload["boundary_kind"]) == "day_rollover"
	if !dayRollover && currentEpoch != stringValue(payload["idle_epoch"]) {
		return map[string]any{"status": "superseded"}, nil
	}
	if !dayRollover && a.now().UTC().Before(currentAt.UTC().Add(conversationSegmentIdleDelay)) {
		return nil, errors.New("conversation_segment_not_idle")
	}
	messages, err := readConversationSummaryMessages(ctx, a.DB.Pool(), work.ConversationID, work.FromSequence, work.ToSequence, conversationSummaryMaxMessages)
	if err != nil {
		return nil, err
	}
	work.Messages = messages
	if err := validateConversationSummarySources(work); err != nil {
		return nil, err
	}
	if err := validateConversationSegmentDay(work.Messages, stringValue(payload["timezone"]), stringValue(payload["local_date"])); err != nil {
		return nil, err
	}
	if a.Provider == nil {
		return nil, errors.New("conversation_segment_provider_unavailable")
	}
	assignment, err := a.Provider.assignment(ctx, "reflection")
	if err != nil {
		return nil, err
	}
	response, err := a.RunConversationSegmentTask(WithProviderCorrelation(ctx, "conversation-segment:"+intentID), work.Messages)
	if err != nil {
		return nil, err
	}
	response.Summary = strings.TrimSpace(response.Summary)
	response.EndingState = strings.TrimSpace(response.EndingState)
	liveAssignment, err := a.Provider.assignment(ctx, "reflection")
	if err != nil {
		return nil, err
	}
	if liveAssignment.EndpointID != assignment.EndpointID || liveAssignment.ModelID != assignment.ModelID {
		return nil, errors.New("conversation_segment_provider_assignment_stale")
	}
	if response.SchemaVersion != conversationSegmentSchemaVersion || response.Summary == "" || len([]rune(response.Summary)) > conversationSummaryMaxRunes || response.EndingState == "" || len([]rune(response.EndingState)) > 1000 || len(response.OpenThreads) > 12 || len(response.CoreEvents) > 20 {
		return nil, errors.New("conversation_segment_response_invalid")
	}
	for _, values := range [][]string{response.OpenThreads, response.CoreEvents} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" || len([]rune(value)) > 500 {
				return nil, errors.New("conversation_segment_response_invalid")
			}
		}
	}
	return a.settleConversationSegment(ctx, work, payload, response, assignment)
}

func (a *App) settleConversationSegment(ctx context.Context, work conversationSummaryWork, payload map[string]any, response conversationSegmentResponse, assignment providerAssignment) (map[string]any, error) {
	result := map[string]any{"status": "superseded"}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var locked string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.conversations WHERE id=$1 FOR UPDATE`, work.ConversationID).Scan(&locked); err != nil {
			return err
		}
		var status, epoch string
		if err := tx.QueryRow(ctx, `SELECT status,payload->>'idle_epoch' FROM public.platform_workflow_intents WHERE intent_id=$1 FOR UPDATE`, work.IntentID).Scan(&status, &epoch); err != nil {
			return err
		}
		var currentEpoch string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.conversation_messages WHERE conversation_id=$1 AND kind='user' ORDER BY sequence DESC LIMIT 1`, work.ConversationID).Scan(&currentEpoch); err != nil {
			return err
		}
		if status == "superseded" || (stringValue(payload["boundary_kind"]) != "day_rollover" && currentEpoch != epoch) {
			return nil
		}
		messages, err := readConversationSummaryMessages(ctx, tx, work.ConversationID, work.FromSequence, work.ToSequence, conversationSummaryMaxMessages)
		if err != nil {
			return err
		}
		work.Messages = messages
		if err := validateConversationSummarySources(work); err != nil {
			return err
		}
		if err := validateConversationSegmentDay(messages, stringValue(payload["timezone"]), stringValue(payload["local_date"])); err != nil {
			return err
		}
		var overlapping bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_summaries s
			WHERE s.owner_fluctlight_id=$1 AND s.conversation_id=$2 AND s.status IN ('active','consolidated')
			  AND s.from_sequence<=$4 AND s.to_sequence>=$3
			  AND (s.from_sequence<>$3 OR s.to_sequence<>$4))`, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence).Scan(&overlapping); err != nil {
			return err
		}
		if overlapping {
			return errors.New("conversation_segment_projection_overlap")
		}
		var priorID string
		var priorRevision int
		err = tx.QueryRow(ctx, `SELECT id,revision FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2
			AND from_sequence=$3 AND to_sequence=$4 AND status IN ('active','invalidated') ORDER BY revision DESC LIMIT 1 FOR UPDATE`, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence).Scan(&priorID, &priorRevision)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if priorID != "" {
			if _, err := tx.Exec(ctx, `UPDATE public.conversation_summaries SET status='superseded' WHERE id=$1 AND status='active'`, priorID); err != nil {
				return err
			}
		}
		revision := priorRevision + 1
		started, ended := messages[0].CreatedAt.UTC(), messages[len(messages)-1].CreatedAt.UTC()
		localDate := stringValue(payload["local_date"])
		zone := stringValue(payload["timezone"])
		summaryID := "conversation_summary_" + stableDigest(work.IntentID+"\x1f"+assignment.EndpointID+"\x1f"+assignment.ModelID+"\x1f"+fmt.Sprint(revision))
		requestDigest := stableDigest(strings.Join([]string{work.SourceDigest, assignment.EndpointID, assignment.ModelID, conversationSegmentPromptVersion, conversationSegmentSchemaVersion}, "\x1f"))
		_, err = tx.Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,supersedes_summary_id,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key,started_at,ended_at,timezone,local_date,ending_state,open_threads,core_events)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,'active',$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25)
			ON CONFLICT (id) DO NOTHING`, summaryID, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence, jsonBytes(work.SourceMessageRefs), work.SourceDigest, response.Summary, revision, nullableString(priorID), assignment.EndpointID, assignment.ModelID, "provider:"+stableDigest(work.IntentID), conversationSegmentPromptVersion, conversationSegmentSchemaVersion, conversationSummaryPolicyVersion, requestDigest, "conversation-segment:"+work.IntentID+":"+fmt.Sprint(revision), started, ended, zone, localDate, response.EndingState, jsonBytes(response.OpenThreads), jsonBytes(response.CoreEvents))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed',completed_at=now() WHERE intent_id=$1`, work.IntentID); err != nil {
			return err
		}
		if err := a.enqueueConversationDailyMemoryTx(ctx, tx, work.FluctlightID, work.ConversationID, localDate, zone); err != nil {
			return err
		}
		if err := a.enqueueConversationSegmentTx(ctx, tx, work.FluctlightID, work.ConversationID, true); err != nil {
			return err
		}
		result = map[string]any{"status": "active", "summary_id": summaryID, "from_sequence": work.FromSequence, "to_sequence": work.ToSequence}
		return nil
	})
	return result, err
}
