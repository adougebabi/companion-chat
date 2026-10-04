package core

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

const runtimeSummaryMaxRunes = 2048
const runtimeSummaryTail = 4

type runtimeSummaryState struct {
	Revision, Covered       int
	Summary, Digest, Status string
	LastBusiness            *time.Time
}

func readRuntimeSummaryState(ctx context.Context, q lifeContextQuerier, owner, conversation string) (runtimeSummaryState, error) {
	var state runtimeSummaryState
	err := q.QueryRow(ctx, `SELECT revision,covered_through,summary,source_digest,status,last_business_at FROM public.conversation_runtime_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2`, owner, conversation).Scan(&state.Revision, &state.Covered, &state.Summary, &state.Digest, &state.Status, &state.LastBusiness)
	if errors.Is(err, pgx.ErrNoRows) {
		return runtimeSummaryState{Status: "active"}, nil
	}
	return state, err
}
func (a *App) enqueueRuntimeSummaryTx(ctx context.Context, tx pgx.Tx, owner, conversation, sourceID string) error {
	var sourceSeq int
	var sourceAt time.Time
	if err := tx.QueryRow(ctx, `SELECT sequence,COALESCE(sender_sent_at,created_at) FROM public.conversation_messages WHERE id=$1 AND conversation_id=$2 AND author_actor_id=$3 AND kind='assistant'`, sourceID, conversation, owner).Scan(&sourceSeq, &sourceAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_runtime_summaries(owner_fluctlight_id,conversation_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, owner, conversation); err != nil {
		return err
	}
	state, err := readRuntimeSummaryState(ctx, tx, owner, conversation)
	if err != nil {
		return err
	}
	if state.Status == "invalidated" {
		state.Covered = 0
		state.Summary = ""
	}
	eligible := sourceSeq - runtimeSummaryTail
	if eligible <= state.Covered {
		return nil
	}
	messages, err := readConversationSummaryMessages(ctx, tx, conversation, state.Covered+1, eligible, conversationSummaryMaxMessages)
	if err != nil {
		return err
	}
	// A publication is a closed turn. Retain the tail and never end a summary
	// between an incoming request and its committed assistant/Tool settlement.
	last := -1
	tokens := 0
	for index, msg := range messages {
		tokens += estimateConversationSummaryTokens(msg)
		if msg.Kind == "assistant" {
			last = index
			if tokens >= conversationSummaryTargetTokens {
				break
			}
		}
	}
	if last < 0 {
		return nil
	}
	messages = messages[:last+1]
	refs, digest := conversationSummarySourceRefs(messages), conversationSummarySourceDigest(messages)
	from, to := messages[0].Sequence, messages[len(messages)-1].Sequence
	identity := stableDigest(fmt.Sprintf("%s:%s:%d:%d:%d:%s", owner, conversation, state.Revision, from, to, digest))
	intentID := "conversation_summary_intent:" + identity
	// Existing immutable-window workflows own execution. No new timer engine.
	interval, _, err := runtimeSummarySettingsWith(ctx, tx)
	if err != nil {
		return err
	}
	due := sourceAt.Add(interval)
	if state.LastBusiness != nil {
		due = state.LastBusiness.Add(interval)
	}
	if tokens >= conversationSummaryTargetTokens || len(messages) >= conversationSummaryMaxMessages {
		due = a.now().UTC()
	}
	payload := map[string]any{"intent_id": intentID, "fluctlight_id": owner, "conversation_id": conversation, "source_message_id": sourceID, "source_sequence": sourceSeq, "from_sequence": from, "to_sequence": to, "source_digest": digest, "source_message_refs": refs, "runtime_revision": state.Revision, "runtime_covered": state.Covered, "runtime_digest": state.Digest}
	// A longer queue window supersedes an unstarted predecessor. A running
	// worker still must pass source/revision CAS; it cannot advance this cursor.
	if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='superseded',last_error='runtime_summary_window_replaced' WHERE intent_type='conversation.summary' AND payload->>'fluctlight_id'=$1 AND payload->>'conversation_id'=$2 AND status IN ('pending','retry') AND intent_id<>$3`, owner, conversation, intentID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at) VALUES($1,$2,'lifecycle','conversation.summary',$3,$4) ON CONFLICT(intent_id) DO NOTHING`, intentID, "conversation_summary:"+identity, jsonBytes(payload), due)
	return err
}
func (a *App) processRuntimeSummary(ctx context.Context, work conversationSummaryWork) (map[string]any, error) {
	if err := a.validateConversationSummaryWorkIdentity(ctx, work); err != nil {
		return nil, err
	}
	var payloadRaw []byte
	var due time.Time
	var intentStatus string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT payload,next_attempt_at,status FROM public.platform_workflow_intents WHERE intent_id=$1`, work.IntentID).Scan(&payloadRaw, &due, &intentStatus); err != nil {
		return nil, err
	}
	payload := decodeObject(payloadRaw)
	var err error
	work.Messages, err = readConversationSummaryMessages(ctx, a.DB.Pool(), work.ConversationID, work.FromSequence, work.ToSequence, conversationSummaryMaxMessages)
	if err != nil {
		return nil, err
	}
	if err := validateRuntimeSummarySources(work); err != nil {
		return nil, err
	}
	state, err := readRuntimeSummaryState(ctx, a.DB.Pool(), work.FluctlightID, work.ConversationID)
	if err != nil {
		return nil, err
	}
	if state.Status == "invalidated" {
		state.Covered = 0
		state.Summary = ""
	}
	if state.Covered >= work.ToSequence && state.Status == "active" {
		return map[string]any{"status": "active", "revision": state.Revision, "to_sequence": state.Covered, "summary": state.Summary, "replayed": true}, nil
	}
	if intentStatus == "superseded" {
		return map[string]any{"status": "superseded"}, nil
	}
	if state.Revision != intValue(payload["runtime_revision"]) || state.Covered != intValue(payload["runtime_covered"]) || work.FromSequence <= state.Covered {
		return nil, errors.New("runtime_summary_revision_stale")
	}
	if a.now().Before(due) {
		return map[string]any{"status": "pending", "next_due_at": formatInstant(due)}, nil
	}
	work.Messages, err = readConversationSummaryMessages(ctx, a.DB.Pool(), work.ConversationID, work.FromSequence, work.ToSequence, conversationSummaryMaxMessages)
	if err != nil {
		return nil, err
	}
	if err := validateRuntimeSummarySources(work); err != nil {
		return nil, err
	}
	if a.Provider == nil {
		return nil, errors.New("conversation_summary_provider_unavailable")
	}
	actorFacts, err := readActorFactsWith(ctx, a.DB.Pool(), work.FluctlightID, "", a.now(), false, 16)
	if err != nil {
		return nil, err
	}
	assignment, err := a.Provider.assignment(ctx, "reflection")
	if err != nil {
		return nil, err
	}
	var ownerActorID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, work.FluctlightID).Scan(&ownerActorID); err != nil {
		return nil, err
	}
	_, outputBudget, err := runtimeSummarySettingsWith(ctx, a.DB.Pool())
	if err != nil {
		return nil, err
	}
	response, err := a.RunConversationSummaryTask(WithProviderCorrelation(ctx, "conversation-summary:"+work.IntentID), ConversationSummaryTaskInput{Messages: work.Messages, PreviousSummary: state.Summary, ActorFacts: actorFacts, MaxRunes: outputBudget, OwnerActorID: ownerActorID, FluctlightID: work.FluctlightID})
	if err != nil {
		return nil, err
	}
	liveAssignment, err := a.Provider.assignment(ctx, "reflection")
	if err != nil {
		return nil, err
	}
	if liveAssignment.EndpointID != assignment.EndpointID || liveAssignment.ModelID != assignment.ModelID {
		return nil, errors.New("conversation_summary_provider_assignment_stale")
	}
	if len([]rune(response.Summary)) > outputBudget {
		return nil, errors.New("runtime_summary_output_budget_exceeded")
	}
	var result map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT id FROM public.conversations WHERE id=$1 FOR UPDATE`, work.ConversationID); err != nil {
			return err
		}
		live, err := readRuntimeSummaryState(ctx, tx, work.FluctlightID, work.ConversationID)
		if err != nil {
			return err
		}
		if live.Revision != state.Revision || live.Status != state.Status {
			return errors.New("runtime_summary_revision_stale")
		}
		currentMessages, err := readConversationSummaryMessages(ctx, tx, work.ConversationID, work.FromSequence, work.ToSequence, conversationSummaryMaxMessages)
		if err != nil {
			return err
		}
		if conversationSummarySourceDigest(currentMessages) != work.SourceDigest {
			return errors.New("runtime_summary_source_drift")
		}
		liveActorFacts, err := readActorFactsWith(ctx, tx, work.FluctlightID, "", a.now(), false, 16)
		if err != nil {
			return err
		}
		if jsonString(liveActorFacts) != jsonString(actorFacts) {
			return errors.New("runtime_summary_actor_facts_stale")
		}
		if err := validateActorFactSnapshotTx(ctx, tx, work.FluctlightID, actorFacts, a.now().UTC()); err != nil {
			return err
		}
		_, liveBudget, err := runtimeSummarySettingsWith(ctx, tx)
		if err != nil {
			return err
		}
		if liveBudget != outputBudget {
			return errors.New("runtime_summary_policy_stale")
		}
		next := state.Revision + 1
		chain := stableDigest(state.Digest + "\x1f" + work.SourceDigest + "\x1f" + response.Summary)
		command, err := tx.Exec(ctx, `UPDATE public.conversation_runtime_summaries SET revision=$3,covered_through=$4,summary=$5,source_digest=$6,status='active',last_business_at=$7,updated_at=now() WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND revision=$8`, work.FluctlightID, work.ConversationID, next, work.ToSequence, response.Summary, chain, a.now().UTC(), state.Revision)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return errors.New("runtime_summary_revision_stale")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_runtime_summary_revisions(owner_fluctlight_id,conversation_id,revision,from_sequence,to_sequence,previous_revision,summary,source_digest,message_refs,model_id,provider_endpoint_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, work.FluctlightID, work.ConversationID, next, work.FromSequence, work.ToSequence, state.Revision, response.Summary, work.SourceDigest, jsonBytes(work.SourceMessageRefs), assignment.ModelID, assignment.EndpointID, a.now().UTC()); err != nil {
			return err
		}
		if err := attachActorFactSnapshotTx(ctx, tx, work.FluctlightID, actorFacts, "summary", "runtime_summary_"+stableDigest(work.FluctlightID+":"+work.ConversationID), next, a.now().UTC()); err != nil {
			return err
		}
		// Preserve bounded historical episode evidence; it is never the active
		// runtime summary authority and does not cause old raw messages to return.
		result = map[string]any{"summary_id": "runtime_summary_" + stableDigest(work.FluctlightID+":"+work.ConversationID), "status": "active", "revision": next, "from_sequence": 1, "to_sequence": work.ToSequence, "summary": response.Summary, "replayed": false}
		// Continue bounded batches when a burst was larger than one model input.
		if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed',completed_at=now() WHERE intent_id=$1`, work.IntentID); err != nil {
			return err
		}
		var latestSource string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.conversation_messages WHERE conversation_id=$1 AND author_actor_id=$2 AND kind='assistant' ORDER BY sequence DESC LIMIT 1`, work.ConversationID, work.FluctlightID).Scan(&latestSource); err != nil {
			return err
		}
		return a.enqueueRuntimeSummaryTx(ctx, tx, work.FluctlightID, work.ConversationID, latestSource)
	})
	return result, err
}
func (a *App) retrieveRuntimeSummary(ctx context.Context, q ConversationSummaryQuery, recent []map[string]any) (ConversationSummaryRetrievalResult, error) {
	result := ConversationSummaryRetrievalResult{Items: []map[string]any{}, Trace: ConversationSummaryRetrievalTrace{Budget: runtimeSummaryMaxRunes}}
	_, budget, budgetErr := runtimeSummarySettingsWith(ctx, a.DB.Pool())
	if budgetErr != nil {
		return result, budgetErr
	}
	result.Trace.Budget = budget
	if _, err := a.DB.GetFluctlight(ctx, q.FluctlightID, q.AuthorizationActorID); err != nil {
		return result, err
	}
	state, err := readRuntimeSummaryState(ctx, a.DB.Pool(), q.FluctlightID, q.ConversationID)
	if err != nil {
		return result, err
	}
	if state.Status != "active" || state.Covered == 0 || strings.TrimSpace(state.Summary) == "" {
		return result, nil
	}
	refs := make([]string, 0)
	for _, message := range recent {
		if intValue(message["sequence"]) <= state.Covered {
			refs = append(refs, "message:"+stringValue(message["id"]))
		}
	}
	result.Items = []map[string]any{{"summary_id": "runtime_summary_" + stableDigest(q.FluctlightID+":"+q.ConversationID), "summary": state.Summary, "from_sequence": 1, "to_sequence": state.Covered, "revision": state.Revision, "source_message_refs": refs, "required": true, "summary_semantics": "bounded cumulative summary; original history retained"}}
	result.Trace.CandidateCount = 1
	result.Trace.SelectedCount = 1
	result.Trace.BudgetUsed = len([]rune(state.Summary))
	return result, nil
}

func validateRuntimeSummarySources(work conversationSummaryWork) error {
	if len(work.Messages) == 0 || work.Messages[0].Sequence != work.FromSequence || work.Messages[len(work.Messages)-1].Sequence != work.ToSequence || work.Messages[len(work.Messages)-1].Kind != "assistant" {
		return errors.New("runtime_summary_source_window_stale")
	}
	previous := 0
	for _, message := range work.Messages {
		if message.Sequence <= previous {
			return errors.New("runtime_summary_source_order_invalid")
		}
		previous = message.Sequence
	}
	if conversationSummarySourceDigest(work.Messages) != work.SourceDigest || !sameStringSlice(conversationSummarySourceRefs(work.Messages), work.SourceMessageRefs) {
		return errors.New("runtime_summary_source_drift")
	}
	return nil
}

func runtimeSummarySettingsWith(ctx context.Context, q lifeContextQuerier) (time.Duration, int, error) {
	var raw string
	err := q.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.summary'`).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, err
	}
	settings := decodeObject([]byte(raw))
	seconds := intValue(settings["interval_seconds"])
	if seconds == 0 {
		seconds = 300
	}
	budget := intValue(settings["max_runes"])
	if budget == 0 {
		budget = runtimeSummaryMaxRunes
	}
	if seconds < 300 || seconds > 600 || budget < 512 || budget > 4096 {
		return 0, 0, ErrInvalidArguments
	}
	return time.Duration(seconds) * time.Second, budget, nil
}
func (a *App) rebuildRuntimeSummariesForBudgetTx(ctx context.Context, tx pgx.Tx, owner string, budget int) error {
	rows, err := tx.Query(ctx, `UPDATE public.conversation_runtime_summaries s SET status='invalidated',revision=revision+1 WHERE s.status='active' AND length(s.summary)>$2 AND EXISTS(SELECT 1 FROM public.fluctlights f WHERE f.id=s.owner_fluctlight_id AND f.created_by_actor_id=$1) RETURNING owner_fluctlight_id,conversation_id`, owner, budget)
	if err != nil {
		return err
	}
	scopes := [][2]string{}
	for rows.Next() {
		var f, c string
		if err := rows.Scan(&f, &c); err != nil {
			rows.Close()
			return err
		}
		scopes = append(scopes, [2]string{f, c})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, scope := range scopes {
		var source string
		err := tx.QueryRow(ctx, `SELECT id FROM public.conversation_messages WHERE conversation_id=$1 AND author_actor_id=$2 AND kind='assistant' ORDER BY sequence DESC LIMIT 1`, scope[1], scope[0]).Scan(&source)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if err := a.enqueueRuntimeSummaryTx(ctx, tx, scope[0], scope[1], source); err != nil {
			return err
		}
	}
	return nil
}

func invalidateRuntimeSummariesForActorTx(ctx context.Context, tx pgx.Tx, owner string, at time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE public.conversation_runtime_summaries SET status='invalidated',revision=revision+1 WHERE owner_fluctlight_id=$1 AND status='active' AND covered_through>0`, owner); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT s.conversation_id,(SELECT m.id FROM public.conversation_messages m WHERE m.conversation_id=s.conversation_id AND m.author_actor_id=s.owner_fluctlight_id AND m.kind='assistant' ORDER BY sequence DESC LIMIT 1) FROM public.conversation_runtime_summaries s WHERE s.owner_fluctlight_id=$1 AND s.status='invalidated'`, owner)
	if err != nil {
		return err
	}
	scopes := [][2]string{}
	for rows.Next() {
		var c string
		var m *string
		if err := rows.Scan(&c, &m); err != nil {
			rows.Close()
			return err
		}
		if m != nil {
			scopes = append(scopes, [2]string{c, *m})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	app := &App{Clock: func() time.Time { return at }}
	for _, scope := range scopes {
		if err := app.enqueueRuntimeSummaryTx(ctx, tx, owner, scope[0], scope[1]); err != nil {
			return err
		}
	}
	return nil
}
