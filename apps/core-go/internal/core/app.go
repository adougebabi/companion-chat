package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type App struct {
	DB           *PostgresRepository
	Provider     *ProviderClient
	Capabilities *CapabilityRegistry
	Workflows    WorkflowRuntime
	SettingsKey  []byte
	ServiceKey   string
	Storage      *minio.Client
	S3Bucket     string
}

func (a *App) SetWorkflowRuntime(runtime WorkflowRuntime) {
	a.Workflows = runtime
}

func NewApp(repository *PostgresRepository, settingsKey, serviceKey, s3Endpoint, s3Region, s3Access, s3Secret, s3Bucket string, useSSL bool) (*App, error) {
	key, err := decodeSettingsKey(settingsKey)
	if err != nil {
		return nil, err
	}
	host := strings.TrimPrefix(strings.TrimPrefix(s3Endpoint, "http://"), "https://")
	storage, err := minio.New(host, &minio.Options{Creds: credentials.NewStaticV4(s3Access, s3Secret, ""), Secure: useSSL, Region: s3Region})
	if err != nil {
		return nil, fmt.Errorf("create object storage client: %w", err)
	}
	app := &App{
		DB:          repository,
		Provider:    &ProviderClient{DB: repository, SettingsKey: key, HTTP: &http.Client{Timeout: 15 * time.Minute}},
		SettingsKey: key,
		ServiceKey:  serviceKey,
		Storage:     storage,
		S3Bucket:    s3Bucket,
	}
	app.Capabilities = NewCapabilityRegistry(
		&imageCapabilityExecutor{app: app},
		&sceneCapabilityExecutor{app: app},
		&presenceCapabilityExecutor{app: app},
		&memoryCapabilityExecutor{app: app},
		&capabilityRequestExecutor{app: app},
	)
	return app, nil
}

func randomID(prefix string) string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		digest := sha256.Sum256([]byte(prefix + time.Now().UTC().String()))
		return prefix + hex.EncodeToString(digest[:])[:32]
	}
	return prefix + hex.EncodeToString(bytes)
}

// StableFluctlightID is the activation idempotency key. It is deliberately
// derived from the authenticated owner and request id, never from user input.
func StableFluctlightID(actorID, requestID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(actorID) + ":" + strings.TrimSpace(requestID)))
	return "fluctlight_" + hex.EncodeToString(digest[:])[:32]
}

func withTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // safe after a successful commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func jsonBytes(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}

func decodeObject(value []byte) map[string]any {
	result := make(map[string]any)
	_ = json.Unmarshal(value, &result)
	return result
}

func stringValue(value any) string {
	if result, ok := value.(string); ok {
		return strings.TrimSpace(result)
	}
	return ""
}

func mapValue(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return map[string]any{}
}

func arrayValue(value any) []any {
	switch result := value.(type) {
	case []any:
		return result
	case []map[string]any:
		items := make([]any, len(result))
		for index, item := range result {
			items[index] = item
		}
		return items
	case []string:
		items := make([]any, len(result))
		for index, item := range result {
			items[index] = item
		}
		return items
	}
	return []any{}
}

func (a *App) ResolveSession(ctx context.Context, token string) (string, error) {
	return a.DB.ResolveSession(ctx, token)
}

func (a *App) Login(ctx context.Context, password string) (string, string, error) {
	if len(password) < 6 {
		a.authAudit(ctx, "login", "", "failed", "password_invalid")
		return "", "", errors.New("authentication_failed")
	}
	var actorID, encodedHash string
	err := a.DB.Pool().QueryRow(ctx, `SELECT human_actor_id, credential_hash FROM public.owner_accounts LIMIT 1`).Scan(&actorID, &encodedHash)
	if err != nil || !verifyArgon2ID(encodedHash, password) {
		a.authAudit(ctx, "login", actorID, "failed", "authentication_failed")
		return "", "", errors.New("authentication_failed")
	}
	token := randomID("session_")
	sessionID := randomID("session_")
	_, err = a.DB.Pool().Exec(ctx, `INSERT INTO public.auth_sessions (id, token_hash, human_actor_id, expires_at, last_seen_at) VALUES ($1,$2,$3,now()+interval '14 days',now())`, sessionID, digestToken(token), actorID)
	if err != nil {
		a.authAudit(ctx, "login", actorID, "failed", "session_create_failed")
		return "", "", fmt.Errorf("create session: %w", err)
	}
	a.authAudit(ctx, "login", actorID, "success", "")
	return actorID, token, nil
}

func (a *App) SetupAvailable(ctx context.Context) (bool, error) {
	var exists bool
	err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.owner_accounts)`).Scan(&exists)
	return !exists, err
}

func (a *App) Setup(ctx context.Context, setupToken, password string) (string, string, error) {
	if len(password) < 6 {
		return "", "", errors.New("setup_unavailable")
	}
	var ownerExists bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.owner_accounts)`).Scan(&ownerExists); err != nil {
		return "", "", err
	}
	if ownerExists {
		return "", "", errors.New("setup_unavailable")
	}
	var tokenID string
	err := a.DB.Pool().QueryRow(ctx, `SELECT id FROM public.owner_setup_tokens WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now()`, digestToken(setupToken)).Scan(&tokenID)
	if err != nil {
		return "", "", errors.New("setup_unavailable")
	}
	hash, err := hashArgon2ID(password)
	if err != nil {
		return "", "", err
	}
	actorID := randomID("human_")
	token := randomID("session_")
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `LOCK TABLE public.owner_accounts IN SHARE ROW EXCLUSIVE MODE`); err != nil {
			return err
		}
		var ownerCount int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM public.owner_accounts`).Scan(&ownerCount); err != nil {
			return err
		}
		if ownerCount != 0 {
			return errors.New("setup_unavailable")
		}
		if err := tx.QueryRow(ctx, `SELECT id FROM public.owner_setup_tokens WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`, digestToken(setupToken)).Scan(&tokenID); err != nil {
			return errors.New("setup_unavailable")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.actors (id, actor_type) VALUES ($1,'human')`, actorID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.owner_accounts (human_actor_id, credential_hash, parameters, credential_revision, owner_key) VALUES ($1,$2,'argon2id-default',$3,'owner')`, actorID, hash, randomID("credential_")); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `UPDATE public.owner_setup_tokens SET consumed_at=now() WHERE id=$1 AND consumed_at IS NULL`, tokenID)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return errors.New("setup_unavailable")
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.auth_sessions (id, token_hash, human_actor_id, expires_at, last_seen_at) VALUES ($1,$2,$3,now()+interval '14 days',now())`, randomID("session_"), digestToken(token), actorID)
		return err
	})
	if err != nil {
		a.authAudit(ctx, "setup", actorID, "failed", "setup_unavailable")
		return "", "", err
	}
	a.authAudit(ctx, "setup", actorID, "success", "")
	return actorID, token, nil
}

func (a *App) authAudit(ctx context.Context, action, actorID, result, details string) {
	if a == nil || a.DB == nil {
		return
	}
	_, _ = a.DB.Pool().Exec(ctx, `INSERT INTO public.auth_audit_log(id,action,actor_id,result,details) VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO NOTHING`, randomID("auth_audit_"), action, nullableString(actorID), result, details)
}

func (a *App) AnalyzeDescription(ctx context.Context, description string) (map[string]any, error) {
	if strings.TrimSpace(description) == "" || len(description) > 12000 {
		return nil, errors.New("description_invalid")
	}
	messages := []map[string]any{
		{"role": "system", "content": "Return one JSON object with foundation.identity, foundation.personality, foundation.behavioral_policy, foundation.life_profile, foundation.initial_goals, foundation.initial_intentions, and provenance.foundation. Fill every semantic field with a concrete value. initial_goals must be an array of objects with description, importance (0..1), and urgency (0..1). initial_intentions must be an array of objects with action, goal_index (zero-based index into initial_goals), and confidence (0..1). Every intention must reference a valid goal_index, and intentions may be fewer than or equal to the number of goals. Do not return markdown or a flat identity/life_profile object outside the foundation envelope."},
		{"role": "user", "content": description},
	}
	result, err := a.Provider.Structured(ctx, "initialization", messages)
	if err != nil {
		return nil, err
	}
	foundation, ok := result["foundation"].(map[string]any)
	if !ok {
		// Some OpenAI-compatible providers omit the outer envelope while still
		// returning the complete typed foundation object. Normalize that wire
		// variation without inventing any semantic field or default value.
		if _, hasIdentity := result["identity"].(map[string]any); hasIdentity {
			if _, hasPersonality := result["personality"].(map[string]any); hasPersonality {
				if _, hasPolicy := result["behavioral_policy"].(map[string]any); hasPolicy {
					if _, hasLifeProfile := result["life_profile"].(map[string]any); hasLifeProfile {
						foundation = result
						result = map[string]any{"foundation": foundation}
						ok = true
					}
				}
			}
		}
	}
	if !ok {
		return nil, errors.New("initialization_foundation_invalid")
	}
	normalizeFoundationCollections(foundation)
	if !validFoundation(foundation) {
		return nil, errors.New("initialization_foundation_invalid")
	}
	return result, nil
}

func validFoundation(value map[string]any) bool {
	for _, key := range []string{"identity", "personality", "behavioral_policy", "life_profile"} {
		if child, ok := value[key].(map[string]any); !ok || len(child) == 0 {
			return false
		}
	}
	if timezone := stringValue(mapValue(value["identity"])["timezone"]); timezone != "" {
		if _, err := time.LoadLocation(canonicalTimezone(timezone)); err != nil {
			return false
		}
	}
	for _, key := range []string{"initial_goals", "initial_intentions"} {
		if raw, exists := value[key]; exists && raw != nil {
			items, ok := raw.([]any)
			if !ok {
				return false
			}
			for index, entry := range items {
				item := mapValue(entry)
				if len(item) == 0 {
					return false
				}
				if key == "initial_goals" {
					if strings.TrimSpace(stringValue(item["description"])) == "" {
						return false
					}
					for _, field := range []string{"importance", "urgency"} {
						if rawValue, present := item[field]; present {
							value, ok := numberFloat(rawValue)
							if !ok || value < 0 || value > 1 {
								return false
							}
						}
					}
				} else {
					if strings.TrimSpace(stringValue(item["action"])) == "" {
						return false
					}
					if rawGoal, present := item["goal_index"]; present {
						goalIndex := intValue(rawGoal)
						if goalIndex < 0 || goalIndex >= len(arrayValue(value["initial_goals"])) {
							return false
						}
					} else if index >= len(arrayValue(value["initial_goals"])) && len(arrayValue(value["initial_goals"])) > 0 {
						return false
					}
				}
			}
		}
	}
	return true
}

// Providers sometimes group goals/intentions by horizon instead of emitting
// the canonical arrays. Convert only the structural shape; every original
// sentence is preserved and no new semantic item is invented.
func normalizeFoundationCollections(foundation map[string]any) {
	for _, key := range []string{"initial_goals", "initial_intentions"} {
		grouped, ok := foundation[key].(map[string]any)
		if ok {
			groups := make([]string, 0, len(grouped))
			for group := range grouped {
				groups = append(groups, group)
			}
			sort.Strings(groups)
			items := make([]any, 0)
			for _, group := range groups {
				for _, raw := range arrayValue(grouped[group]) {
					if object := mapValue(raw); len(object) > 0 {
						items = append(items, object)
						continue
					}
					text := stringValue(raw)
					if text != "" {
						items = appendFoundationCollectionItem(items, key, text, group)
					}
				}
			}
			foundation[key] = items
			continue
		}
		// A few OpenAI-compatible providers emit a flat string array instead
		// of the typed collection objects. Wrap each sentence without changing
		// its text; the scalar confidence/importance values are required by the
		// persistence contract and match the grouped-shape normalization above.
		flat := arrayValue(foundation[key])
		if len(flat) == 0 {
			continue
		}
		items := make([]any, 0, len(flat))
		for _, raw := range flat {
			if object := mapValue(raw); len(object) > 0 {
				items = append(items, object)
			} else if text := stringValue(raw); text != "" {
				items = appendFoundationCollectionItem(items, key, text, "")
			}
		}
		foundation[key] = items
	}
}

func appendFoundationCollectionItem(items []any, key, text, horizon string) []any {
	if key == "initial_goals" {
		item := map[string]any{"description": text, "importance": 0.5, "urgency": 0.5}
		if horizon != "" {
			item["horizon"] = horizon
		}
		return append(items, item)
	}
	item := map[string]any{"action": text, "confidence": 0.5, "goal_index": 0}
	if horizon != "" {
		item["horizon"] = horizon
	}
	return append(items, item)
}

func defaultIdentity(id, name string) map[string]any {
	return map[string]any{"id": id, "name": name, "age": nil, "gender": nil, "occupation": nil, "residence": nil, "timezone": "Asia/Shanghai", "birthday": nil, "background": nil, "biography": nil, "core_values": []any{}, "worldview": nil, "notes": nil}
}

func defaultPersonality() map[string]any {
	return map[string]any{"openness": 0.5, "conscientiousness": 0.5, "extraversion": 0.5, "agreeableness": 0.5, "neuroticism": 0.5, "curiosity": 0.5, "independence": 0.5, "patience": 0.5, "empathy": 0.5, "assertiveness": 0.5, "humor": 0.5, "sociability": 0.5, "risk_tolerance": 0.5, "update_policy": map[string]any{"evidence_window_events": 3, "max_delta": 0.05, "cooldown_seconds": 86400, "minimum_confidence": 0.7}}
}

func defaultPolicy() map[string]any {
	return map[string]any{"response_style": "温和简洁", "message_length": "short", "emoji_frequency": 0.1, "punctuation_style": "自然", "humor_style": "适度", "sarcasm_tendency": 0.1, "directness": 0.6, "initiative": 0.5, "topic_initiation": 0.5, "silence_tolerance": 0.5, "response_delay": 0, "emotional_expression": 0.6, "conflict_style": "direct", "refusal_style": "clear", "intimacy_expression": "自然"}
}

func defaultLifeProfile() map[string]any {
	return map[string]any{"appearance": map[string]any{}, "social_background": map[string]any{}, "preferences": map[string]any{}, "life_habits": []any{}, "recurring_commitments": []any{}, "relationship_seeds": []any{}, "character_constraints": []any{}}
}

func defaultProvenance() map[string]any {
	return map[string]any{"field_sources": map[string]any{}, "self_model": map[string]any{}}
}

func defaultInnerState() (map[string]any, map[string]any, map[string]any, map[string]any, []any, []any) {
	return map[string]any{"pleasure": 0.0, "arousal": 0.0, "dominance": 0.0}, map[string]any{"label": nil, "intensity": 0.0, "source": "regulation"}, map[string]any{"value": 0.0, "trend": 0.0}, map[string]any{"stress": 0.0, "stability": 1.0}, []any{}, []any{}
}

func (a *App) CreateFluctlight(ctx context.Context, actorID, requestedID, name string, mode string, foundation map[string]any, goals, intentions []any) (Fluctlight, error) {
	if mode != "blank_slate" && mode != "llm_defined" {
		return Fluctlight{}, errors.New("initialization_mode_invalid")
	}
	if mode == "llm_defined" && (foundation == nil || !validFoundation(foundation)) {
		return Fluctlight{}, errors.New("initialization_foundation_invalid")
	}
	if mode == "blank_slate" && foundation != nil {
		return Fluctlight{}, errors.New("blank_slate_foundation_forbidden")
	}
	id := requestedID
	if id == "" {
		id = randomID("fluctlight_")
	}
	identity := defaultIdentity(id, name)
	personality := defaultPersonality()
	policy := defaultPolicy()
	lifeProfile := defaultLifeProfile()
	provenance := defaultProvenance()
	if foundation != nil {
		if value, ok := foundation["identity"].(map[string]any); ok {
			identity = value
			identity["id"] = id
		}
		if value, ok := foundation["personality"].(map[string]any); ok {
			personality = value
		}
		if value, ok := foundation["behavioral_policy"].(map[string]any); ok {
			policy = value
		}
		if value, ok := foundation["life_profile"].(map[string]any); ok {
			lifeProfile = value
		}
		if value, ok := foundation["provenance"].(map[string]any); ok {
			provenance = value
		}
		if value, ok := foundation["initial_goals"].([]any); ok {
			goals = value
		}
		if value, ok := foundation["initial_intentions"].([]any); ok {
			intentions = value
		}
	}
	var result Fluctlight
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var exists, existingOwner string
		if err := tx.QueryRow(ctx, `SELECT id,created_by_actor_id FROM public.fluctlights WHERE id=$1`, id).Scan(&exists, &existingOwner); err == nil {
			if existingOwner != actorID {
				return fmt.Errorf("fluctlight already exists")
			}
			// A request-id replay is idempotent. The caller will read and return
			// the already committed aggregate after the transaction.
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.actors (id,actor_type) VALUES ($1,'fluctlight') ON CONFLICT (id) DO NOTHING`, id); err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlights (id,created_by_actor_id,initialization_mode,status,current_revision,identity,personality,behavioral_policy,life_profile,provenance,created_at,updated_at) VALUES ($1,$2,$3,'active',0,$4,$5,$6,$7,$8,$9,$9)`, id, actorID, mode, jsonBytes(identity), jsonBytes(personality), jsonBytes(policy), jsonBytes(lifeProfile), jsonBytes(provenance), now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_foundation_revisions (id,fluctlight_id,revision,base_revision,source,status,actor_id,initialization_mode,foundation_status,foundation_created_at,confidence,changes,identity,personality,behavioral_policy,life_profile,provenance,evidence_refs,reason,idempotency_key,created_at,accepted_at) VALUES ($1,$2,0,0,'initialization','accepted',$3,$4,'active',$5,$6,'{}',$7,$8,$9,$10,$11,'[]',NULL,$12,$5,$5)`, randomID("foundation_revision_"), id, actorID, mode, now, jsonBytes(1.0), jsonBytes(identity), jsonBytes(personality), jsonBytes(policy), jsonBytes(lifeProfile), jsonBytes(provenance), "fluctlight-create:"+id); err != nil {
			return err
		}
		pad, mood, momentum, regulation, drives, conflicts := defaultInnerState()
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_inner_states (fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES ($1,0,$2,$3,$4,$5,$6,$7,$8)`, id, jsonBytes(pad), jsonBytes(mood), jsonBytes(momentum), jsonBytes(regulation), jsonBytes(drives), jsonBytes(conflicts), now); err != nil {
			return err
		}
		if err := a.insertDirectConversation(ctx, tx, actorID, id); err != nil {
			return err
		}
		localDate := now.Format("2006-01-02")
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'lifecycle','schedule.current_day',$3) ON CONFLICT DO NOTHING`, "schedule_intent:"+id, "schedule:"+id, jsonBytes(map[string]any{"fluctlight_id": id})); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'lifecycle','daily_review.current_day',$3) ON CONFLICT DO NOTHING`, "daily_review_intent:"+id+":"+localDate, "daily_review:"+id+":"+localDate, jsonBytes(map[string]any{"fluctlight_id": id, "local_date": localDate})); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'lifecycle','wake_up.current',$3) ON CONFLICT DO NOTHING`, "wake_up_intent:"+id, "wake_up:"+id, jsonBytes(map[string]any{"fluctlight_id": id, "cycle": 0})); err != nil {
			return err
		}
		if err := a.insertAgency(ctx, tx, id, actorID, goals, intentions); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Fluctlight{}, err
	}
	result, err = a.DB.GetFluctlight(ctx, id, actorID)
	return result, err
}

func (a *App) insertDirectConversation(ctx context.Context, tx pgx.Tx, ownerID, fluctlightID string) error {
	conversationID := randomID("conversation_")
	if _, err := tx.Exec(ctx, `INSERT INTO public.conversations (id,created_by_actor_id,title,revision) VALUES ($1,$2,NULL,0)`, conversationID, ownerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_heads (conversation_id,next_sequence) VALUES ($1,1)`, conversationID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_participants (conversation_id,actor_id,role,status) VALUES ($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_read_positions (conversation_id,actor_id) VALUES ($1,$2),($1,$3)`, conversationID, ownerID, fluctlightID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations (owner_actor_id,fluctlight_actor_id,conversation_id) VALUES ($1,$2,$3)`, ownerID, fluctlightID, conversationID)
	return err
}

// EnsureDirectConversation preserves the public get-or-create contract for an
// owner/persona pair. It is safe to call concurrently because the pair is a
// primary key and the conversation projection is created in the same tx.
func (a *App) EnsureDirectConversation(ctx context.Context, ownerID, fluctlightID string) (string, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, ownerID); err != nil {
		return "", err
	}
	var id string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT conversation_id FROM public.fluctlight_direct_conversations WHERE owner_actor_id=$1 AND fluctlight_actor_id=$2`, ownerID, fluctlightID).Scan(&id); err == nil {
		return id, nil
	}
	id = randomID("conversation_")
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var existing string
		if err := tx.QueryRow(ctx, `SELECT conversation_id FROM public.fluctlight_direct_conversations WHERE owner_actor_id=$1 AND fluctlight_actor_id=$2 FOR UPDATE`, ownerID, fluctlightID).Scan(&existing); err == nil {
			id = existing
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return a.insertDirectConversation(ctx, tx, ownerID, fluctlightID)
	})
	if err == nil {
		_ = a.DB.Pool().QueryRow(ctx, `SELECT conversation_id FROM public.fluctlight_direct_conversations WHERE owner_actor_id=$1 AND fluctlight_actor_id=$2`, ownerID, fluctlightID).Scan(&id)
	}
	return id, err
}

func (a *App) insertAgency(ctx context.Context, tx pgx.Tx, fluctlightID, actorID string, goals, intentions []any) error {
	goalIDs := make([]string, len(goals))
	for index, raw := range goals {
		item := mapValue(raw)
		goalIDs[index] = fmt.Sprintf("goal_initial_%s_%d", fluctlightID, index)
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_goals (id,fluctlight_id,source,description,importance,urgency,progress,status,evidence_refs,revision) VALUES ($1,$2,'self',$3,$4,$5,$6,'active',$7,0) ON CONFLICT DO NOTHING`, goalIDs[index], fluctlightID, stringValue(item["description"]), jsonBytes(item["importance"]), jsonBytes(item["urgency"]), jsonBytes(0.0), jsonBytes([]string{"foundation:" + fluctlightID})); err != nil {
			return err
		}
	}
	for index, raw := range intentions {
		item := mapValue(raw)
		goalIndex := intValue(item["goal_index"])
		if goalIndex < 0 || goalIndex >= len(goalIDs) {
			return errors.New("initial_intention_goal_invalid")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_intentions (id,fluctlight_id,goal_id,action,trigger,confidence,expiration,evidence_refs,permission_snapshot,budget_snapshot,status,revision) VALUES ($1,$2,$3,$4,$5,$6,now()+interval '24 hours',$7,'{}','{}','pending',0) ON CONFLICT DO NOTHING`, fmt.Sprintf("intention_initial_%s_%d", fluctlightID, index), fluctlightID, goalIDs[goalIndex], stringValue(item["action"]), jsonBytes(map[string]any{"type": "semantic", "schema_version": "semantic.trigger.v1", "evidence_refs": []string{"foundation:" + fluctlightID}}), jsonBytes(item["confidence"]), jsonBytes([]string{"foundation:" + fluctlightID})); err != nil {
			return err
		}
	}
	return nil
}
