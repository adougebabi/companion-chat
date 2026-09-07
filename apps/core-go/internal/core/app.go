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
	"github.com/redis/go-redis/v9"
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
	Redis        redis.UniversalClient
}

// SetRedisClient wires the optional Redis acceleration layers (provider queue
// coordination and trigger notifications). Domain state remains PostgreSQL/
// Temporal-owned when Redis is unavailable.
func (a *App) SetRedisClient(client redis.UniversalClient, processID string) {
	if a == nil {
		return
	}
	a.Redis = client
	if a.Provider != nil {
		a.Provider.SetRedisClient(client, processID)
	}
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
	app.Provider.generated = newProviderQueue(providerQueueDefaultConcurrency)
	app.Provider.embedding = newProviderQueue(providerQueueDefaultEmbedding)
	app.Capabilities = NewCapabilityRegistry(
		&imageCapabilityExecutor{app: app},
		&visualIdentityCapabilityExecutor{app: app},
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
		{"role": "system", "content": "You are initializing actor_self, the current Fluctlight. The fixed Actor ref actor_user always means the current authenticated Human user; use actor_user as target_actor_id when a relationship or goal refers to that user, never a database ID. Return exactly one JSON object with exactly these top-level fields: schema_version, core_persona, developing_self, initial_relationships, initial_goals, initial_intentions, extensions. Always return all defined fields and arrays, using [] or {} only where the schema declares an empty value. All known fields must stay in their canonical groups; put any not-yet-classified field only under extensions, never invent another top-level field. Known identity fields are name, age, gender, occupation, residence, timezone, birthday, background, biography, core_values, worldview, and notes. Known personality fields are openness, conscientiousness, extraversion, agreeableness, neuroticism, curiosity, independence, patience, empathy, assertiveness, humor, sociability, risk_tolerance, and update_policy. Known behavioral_policy fields are response_style, message_length, emoji_frequency, punctuation_style, humor_style, sarcasm_tendency, directness, initiative, topic_initiation, silence_tolerance, response_delay, emotional_expression, conflict_style, refusal_style, and intimacy_expression. Known life_profile fields are appearance, social_background, preferences, life_habits, recurring_commitments, relationship_seeds, and character_constraints. core_persona must contain identity, personality, behavioral_policy, life_profile, and personality_system. personality_system must contain mode, profiles, active_profile_id, switching, influence, conflict_resolution, integration, behavior_state_machine, and extensions. Every personality_system.profiles item must independently define id, name, identity, personality, behavioral_policy, emotional_state, voice, body_language, behavior_state_machine, behavior_loops, scenario_behavior, secrets, intimacy_progression, output_preferences, fears, desires, and extensions. voice should explicitly describe known sound fields such as tone, pitch, speed, volume, timbre, and speech_patterns; body_language should describe posture, gestures, movement_style, gaze, and proximity; switching.rules should identify each condition and target profile; influence.edges should identify source, target, strength, direction, and condition; integration should describe fusion_progress and stage; conflict_resolution should describe strategy, priority, dominant_profile_id, and tie_breaker. These are semantic inputs for the later cognition decision; do not make the server infer switching from them. When a relationship, goal, or intention belongs to one personality, include its profile_id. Put stable identity, values, temperament, expression principles, and boundaries in core_persona. Put only uncertain preferences, habits, sensitivities, emotion patterns, self-perceptions, capabilities, or interests in developing_self.claims. Every developing_self claim must include category, claim, value, confidence (0..1), evidence_refs, and provenance; use provenance.source=owner_defined for facts explicitly stated by the owner. Never put current mood, fatigue, scene, presence, or a one-off reaction in core_persona. Current State is initialized by the server and must not be returned. initial_relationships may describe actor_self's relationship to actor_user; role is an open semantic object with a label and optional role.addressing.preferred/self_reference. Do not infer a relationship only because actor_user is the current user; return an unknown role when the description does not establish one. initial_goals must be an array of objects with description, importance (0..1), urgency (0..1), and optional scope (general or relationship) plus target_actor_id for relationship goals; include profile_id when the goal belongs to a personality. initial_intentions must be an array of objects with action, goal_index (zero-based index into initial_goals), confidence (0..1), and profile_id when the intention belongs to a personality. Do not return markdown or legacy foundation/personality candidate fields."},
		{"role": "system", "content": "Canonical visual appearance contract: if the description specifies a chest cup, put only the normalized label A/B/C/D in exactly core_persona.life_profile.appearance.chest_cup (example: {\"life_profile\":{\"appearance\":{\"chest_cup\":\"A\"}}}). Do not put cup labels in identity.body_type, identity.build, identity.chest, life_profile.physical_traits, or free-form visible_text. For male or non-applicable bodies, omit chest_cup; the renderer will mark it not_applicable. Keep other appearance fields under life_profile.appearance."},
		{"role": "user", "content": description},
	}
	result, err := a.Provider.Structured(WithProviderScenario(ctx, "initialization"), "initialization", messages)
	if err != nil {
		return nil, err
	}
	if !hasInitializationEnvelope(result) {
		return nil, errors.New("initialization_persona_invalid")
	}
	result = normalizeInitializationResponse(result)
	normalizeVisualIdentityFoundation(mapValue(result["core_persona"]))
	if !validInitialization(result) {
		return nil, errors.New("initialization_persona_invalid")
	}
	return result, nil
}

func hasInitializationEnvelope(value map[string]any) bool {
	if value == nil {
		return false
	}
	if _, ok := numberFloat(value["schema_version"]); !ok || !isObjectValue(value["core_persona"]) || !isObjectValue(value["developing_self"]) || !isObjectValue(value["extensions"]) {
		return false
	}
	for _, key := range []string{"initial_relationships", "initial_goals", "initial_intentions"} {
		if _, ok := value[key].([]any); !ok {
			return false
		}
	}
	return true
}

func validInitialization(value map[string]any) bool {
	if _, ok := numberFloat(value["schema_version"]); !ok {
		return false
	}
	for key := range value {
		if _, ok := map[string]struct{}{"schema_version": {}, "core_persona": {}, "developing_self": {}, "initial_goals": {}, "initial_intentions": {}, "initial_relationships": {}, "extensions": {}}[key]; !ok {
			return false
		}
	}
	if extensions, ok := value["extensions"]; ok && !isObjectValue(extensions) {
		return false
	}
	corePersona, ok := value["core_persona"].(map[string]any)
	if !ok || len(corePersona) == 0 {
		return false
	}
	for _, key := range []string{"identity", "personality", "behavioral_policy", "life_profile"} {
		if child, ok := corePersona[key].(map[string]any); !ok || len(child) == 0 {
			return false
		}
	}
	if !hasInitializationKeys(mapValue(corePersona["identity"]), []string{"name", "age", "gender", "occupation", "residence", "timezone", "birthday", "background", "biography", "core_values", "worldview", "notes"}) ||
		!hasInitializationKeys(mapValue(corePersona["personality"]), []string{"openness", "conscientiousness", "extraversion", "agreeableness", "neuroticism", "curiosity", "independence", "patience", "empathy", "assertiveness", "humor", "sociability", "risk_tolerance", "update_policy"}) ||
		!hasInitializationKeys(mapValue(corePersona["behavioral_policy"]), []string{"response_style", "message_length", "emoji_frequency", "punctuation_style", "humor_style", "sarcasm_tendency", "directness", "initiative", "topic_initiation", "silence_tolerance", "response_delay", "emotional_expression", "conflict_style", "refusal_style", "intimacy_expression"}) ||
		!hasInitializationKeys(mapValue(corePersona["life_profile"]), []string{"appearance", "social_background", "preferences", "life_habits", "recurring_commitments", "relationship_seeds", "character_constraints"}) {
		return false
	}
	if _, ok := numberFloat(corePersona["schema_version"]); !ok {
		return false
	}
	for key := range corePersona {
		if _, ok := map[string]struct{}{"schema_version": {}, "identity": {}, "personality": {}, "behavioral_policy": {}, "life_profile": {}, "personality_system": {}}[key]; !ok {
			return false
		}
	}
	if timezone := stringValue(mapValue(corePersona["identity"])["timezone"]); timezone != "" {
		if _, err := time.LoadLocation(canonicalTimezone(timezone)); err != nil {
			return false
		}
	}
	system, ok := corePersona["personality_system"].(map[string]any)
	if !ok || len(system) == 0 {
		return false
	}
	if mode := stringValue(system["mode"]); mode != "single" && mode != "multiple" {
		return false
	}
	if !hasInitializationKeys(system, []string{"mode", "profiles", "active_profile_id", "switching", "influence", "conflict_resolution", "integration", "behavior_state_machine", "extensions"}) {
		return false
	}
	if strings.TrimSpace(stringValue(system["active_profile_id"])) == "" {
		return false
	}
	profiles, ok := system["profiles"].([]any)
	if !ok {
		return false
	}
	seenProfiles := map[string]struct{}{}
	for _, raw := range profiles {
		profile := mapValue(raw)
		profileID := strings.TrimSpace(stringValue(profile["id"]))
		if profileID == "" {
			return false
		}
		if _, exists := seenProfiles[profileID]; exists {
			return false
		}
		seenProfiles[profileID] = struct{}{}
		for _, field := range personalityProfileFieldNames() {
			if _, ok := profile[field]; !ok {
				return false
			}
		}
	}
	if active := stringValue(system["active_profile_id"]); active != "default" {
		if _, exists := seenProfiles[active]; !exists {
			return false
		}
	}
	developingSelf, ok := value["developing_self"].(map[string]any)
	if !ok {
		return false
	}
	claims, ok := developingSelf["claims"].([]any)
	if !ok {
		return false
	}
	for _, raw := range claims {
		claim := mapValue(raw)
		for key := range claim {
			if _, ok := map[string]struct{}{"category": {}, "claim": {}, "value": {}, "confidence": {}, "evidence_refs": {}, "provenance": {}, "status": {}}[key]; !ok {
				return false
			}
		}
		if len(claim) == 0 || stringValue(claim["category"]) == "" || strings.TrimSpace(stringValue(claim["claim"])) == "" || claim["value"] == nil {
			return false
		}
		if confidence, ok := numberFloat(claim["confidence"]); !ok || confidence < 0 || confidence > 1 {
			return false
		}
		if !validateDevelopingSelfCategory(stringValue(claim["category"])) {
			return false
		}
		if provenance := mapValue(claim["provenance"]); stringValue(provenance["source"]) == "" {
			return false
		}
		refs, refsPresent := claim["evidence_refs"]
		if !refsPresent {
			return false
		}
		refValues, ok := refs.([]any)
		if !ok {
			return false
		}
		if len(refValues) > 0 {
			for _, ref := range refValues {
				if strings.TrimSpace(stringValue(ref)) == "" {
					return false
				}
			}
		}
	}
	for _, key := range []string{"initial_goals", "initial_intentions", "initial_relationships"} {
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
				} else if key == "initial_intentions" {
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
	for _, raw := range arrayValue(value["initial_relationships"]) {
		item := mapValue(raw)
		if strings.TrimSpace(stringValue(item["target_actor_id"])) == "" {
			return false
		}
		if _, err := normalizeRelationshipRole(item["role"]); err != nil {
			return false
		}
		if _, err := validateRelationshipMetrics(item["metrics"]); err != nil {
			return false
		}
		trend := firstString(item["trend"], "stable")
		if trend != "improving" && trend != "stable" && trend != "declining" {
			return false
		}
	}
	return true
}

func hasInitializationKeys(value map[string]any, keys []string) bool {
	for _, key := range keys {
		if _, ok := value[key]; !ok {
			return false
		}
	}
	return true
}

func isObjectValue(value any) bool {
	object, ok := value.(map[string]any)
	return ok && object != nil
}

func normalizeInitializationResponse(value map[string]any) map[string]any {
	result := cloneMap(value)
	if result == nil {
		result = map[string]any{}
	}
	extensions := mapValue(result["extensions"])
	if extensions == nil {
		extensions = map[string]any{}
	}
	if _, ok := result["schema_version"]; !ok {
		result["schema_version"] = 2
	}
	if _, ok := result["initial_relationships"]; !ok {
		if raw, exists := result["relationships"]; exists {
			result["initial_relationships"] = raw
			delete(result, "relationships")
		} else if raw, exists := result["relationship_seeds"]; exists {
			result["initial_relationships"] = raw
			delete(result, "relationship_seeds")
		} else {
			lifeProfile := mapValue(mapValue(result["core_persona"])["life_profile"])
			result["initial_relationships"] = lifeProfile["relationship_seeds"]
			delete(lifeProfile, "relationship_seeds")
		}
	}
	if result["initial_relationships"] == nil {
		result["initial_relationships"] = []any{}
	}
	if result["initial_goals"] == nil {
		result["initial_goals"] = []any{}
	}
	if result["initial_intentions"] == nil {
		result["initial_intentions"] = []any{}
	}
	if raw, ok := result["other"]; ok {
		extensions["other"] = raw
		delete(result, "other")
	}
	knownTopLevel := map[string]struct{}{"schema_version": {}, "core_persona": {}, "developing_self": {}, "initial_relationships": {}, "initial_goals": {}, "initial_intentions": {}, "extensions": {}}
	for key, raw := range result {
		if _, ok := knownTopLevel[key]; !ok {
			extensions["top_level."+key] = raw
			delete(result, key)
		}
	}
	persona := mapValue(result["core_persona"])
	knownPersona := map[string]map[string]struct{}{
		"identity":           {"name": {}, "age": {}, "gender": {}, "occupation": {}, "residence": {}, "timezone": {}, "birthday": {}, "background": {}, "biography": {}, "core_values": {}, "worldview": {}, "notes": {}},
		"personality":        {"openness": {}, "conscientiousness": {}, "extraversion": {}, "agreeableness": {}, "neuroticism": {}, "curiosity": {}, "independence": {}, "patience": {}, "empathy": {}, "assertiveness": {}, "humor": {}, "sociability": {}, "risk_tolerance": {}, "update_policy": {}},
		"behavioral_policy":  {"response_style": {}, "message_length": {}, "emoji_frequency": {}, "punctuation_style": {}, "humor_style": {}, "sarcasm_tendency": {}, "directness": {}, "initiative": {}, "topic_initiation": {}, "silence_tolerance": {}, "response_delay": {}, "emotional_expression": {}, "conflict_style": {}, "refusal_style": {}, "intimacy_expression": {}},
		"life_profile":       {"appearance": {}, "social_background": {}, "preferences": {}, "life_habits": {}, "recurring_commitments": {}, "relationship_seeds": {}, "character_constraints": {}},
		"personality_system": {"mode": {}, "profiles": {}, "active_profile_id": {}, "switching": {}, "influence": {}, "conflict_resolution": {}, "integration": {}, "behavior_state_machine": {}, "extensions": {}},
	}
	for group, allowed := range knownPersona {
		fields := mapValue(persona[group])
		for key, raw := range fields {
			if _, ok := allowed[key]; !ok {
				extensions["core_persona."+group+"."+key] = raw
				delete(fields, key)
			}
		}
	}
	normalizePersonalityProfiles(mapValue(persona["personality_system"]))
	result["extensions"] = extensions
	return result
}

func normalizePersonalityProfiles(system map[string]any) {
	profiles := arrayValue(system["profiles"])
	if len(profiles) == 0 {
		return
	}
	known := map[string]struct{}{
		"id": {}, "name": {}, "identity": {}, "personality": {}, "behavioral_policy": {},
		"emotional_state": {}, "voice": {}, "body_language": {}, "behavior_state_machine": {},
		"behavior_loops": {}, "scenario_behavior": {}, "secrets": {}, "intimacy_progression": {},
		"output_preferences": {}, "fears": {}, "desires": {}, "extensions": {},
	}
	for _, raw := range profiles {
		profile := mapValue(raw)
		if len(profile) == 0 {
			continue
		}
		extensions := mapValue(profile["extensions"])
		if len(extensions) == 0 {
			extensions = map[string]any{}
		}
		for key, value := range profile {
			if _, ok := known[key]; !ok {
				extensions[key] = value
				delete(profile, key)
			}
		}
		profile["extensions"] = extensions
	}
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

func defaultCorePersona(id, name string) map[string]any {
	return map[string]any{
		"schema_version":     1,
		"identity":           defaultIdentity(id, name),
		"personality":        defaultPersonality(),
		"behavioral_policy":  defaultPolicy(),
		"life_profile":       defaultLifeProfile(),
		"personality_system": defaultPersonalitySystem(),
	}
}

func defaultPersonalitySystem() map[string]any {
	return map[string]any{
		"mode": "single", "profiles": []any{}, "active_profile_id": "default",
		"switching": map[string]any{"rules": []any{}}, "influence": map[string]any{"edges": []any{}},
		"conflict_resolution": map[string]any{}, "integration": map[string]any{},
		"behavior_state_machine": map[string]any{}, "extensions": map[string]any{},
	}
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
	return map[string]any{"field_sources": map[string]any{}}
}

func defaultInnerState() (map[string]any, map[string]any, map[string]any, map[string]any, []any, []any) {
	return map[string]any{"pleasure": 0.0, "arousal": 0.0, "dominance": 0.0}, map[string]any{"label": nil, "intensity": 0.0, "source": "regulation"}, map[string]any{"value": 0.0, "trend": 0.0}, map[string]any{"stress": 0.0, "stability": 1.0}, []any{}, []any{}
}

func (a *App) CreateFluctlight(ctx context.Context, actorID, requestedID, name string, mode string, foundation map[string]any, goals, intentions []any) (Fluctlight, error) {
	if mode != "blank_slate" && mode != "llm_defined" {
		return Fluctlight{}, errors.New("initialization_mode_invalid")
	}
	if mode == "llm_defined" && !hasInitializationEnvelope(foundation) {
		return Fluctlight{}, errors.New("initialization_persona_invalid")
	}
	if foundation != nil {
		foundation = normalizeInitializationResponse(foundation)
	}
	if mode == "llm_defined" && (foundation == nil || !validInitialization(foundation)) {
		return Fluctlight{}, errors.New("initialization_persona_invalid")
	}
	if mode == "blank_slate" && foundation != nil {
		return Fluctlight{}, errors.New("blank_slate_foundation_forbidden")
	}
	id := requestedID
	if id == "" {
		id = randomID("fluctlight_")
	}
	corePersona := defaultCorePersona(id, name)
	identity := mapValue(corePersona["identity"])
	personality := mapValue(corePersona["personality"])
	policy := mapValue(corePersona["behavioral_policy"])
	lifeProfile := mapValue(corePersona["life_profile"])
	provenance := defaultProvenance()
	if foundation != nil {
		if extensions := mapValue(foundation["extensions"]); len(extensions) > 0 {
			provenance["initialization_extensions"] = extensions
		}
		if value, ok := foundation["core_persona"].(map[string]any); ok {
			corePersona = value
		}
		if value, ok := corePersona["identity"].(map[string]any); ok {
			identity = value
			identity["id"] = id
		}
		if value, ok := corePersona["personality"].(map[string]any); ok {
			personality = value
		}
		if value, ok := corePersona["behavioral_policy"].(map[string]any); ok {
			policy = value
		}
		if value, ok := corePersona["life_profile"].(map[string]any); ok {
			lifeProfile = value
		}
		if _, ok := corePersona["schema_version"]; !ok {
			corePersona["schema_version"] = 1
		}
		if value, ok := foundation["initial_goals"].([]any); ok {
			goals = value
		}
		if value, ok := foundation["initial_intentions"].([]any); ok {
			intentions = value
		}
	}
	normalizeVisualIdentityFoundation(corePersona)
	developingSelfClaims := []any{}
	if foundation != nil {
		developingSelfClaims = arrayValue(mapValue(foundation["developing_self"])["claims"])
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
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlights (id,created_by_actor_id,initialization_mode,status,current_revision,core_persona,identity,personality,behavioral_policy,life_profile,provenance,created_at,updated_at) VALUES ($1,$2,$3,'active',0,$4,$5,$6,$7,$8,$9,$10,$10)`, id, actorID, mode, jsonBytes(corePersona), jsonBytes(identity), jsonBytes(personality), jsonBytes(policy), jsonBytes(lifeProfile), jsonBytes(provenance), now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_foundation_revisions (id,fluctlight_id,revision,base_revision,source,status,actor_id,initialization_mode,foundation_status,foundation_created_at,confidence,changes,core_persona,identity,personality,behavioral_policy,life_profile,provenance,evidence_refs,reason,idempotency_key,created_at,accepted_at) VALUES ($1,$2,0,0,'initialization','accepted',$3,$4,'active',$5,$6,'{}',$7,$8,$9,$10,$11,$12,'[]',NULL,$13,$5,$5)`, randomID("foundation_revision_"), id, actorID, mode, now, jsonBytes(1.0), jsonBytes(corePersona), jsonBytes(identity), jsonBytes(personality), jsonBytes(policy), jsonBytes(lifeProfile), jsonBytes(provenance), "fluctlight-create:"+id); err != nil {
			return err
		}
		pad, mood, momentum, regulation, drives, conflicts := defaultInnerState()
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_inner_states (fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES ($1,0,$2,$3,$4,$5,$6,$7,$8)`, id, jsonBytes(pad), jsonBytes(mood), jsonBytes(momentum), jsonBytes(regulation), jsonBytes(drives), jsonBytes(conflicts), now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision,updated_at) VALUES($1,$2,0,$3) ON CONFLICT DO NOTHING`, id, initialPersonalityProfileID(corePersona), now); err != nil {
			return err
		}
		if err := a.insertDevelopingSelfSeeds(ctx, tx, id, developingSelfClaims); err != nil {
			return err
		}
		if err := a.insertDirectConversation(ctx, tx, actorID, id); err != nil {
			return err
		}
		if _, err := a.ensureVisualIdentityInitializationTx(ctx, tx, id, "initialization", "foundation:"+id, corePersona); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'lifecycle','schedule.current_day',$3) ON CONFLICT DO NOTHING`, "schedule_intent:"+id, "schedule:"+id, jsonBytes(map[string]any{"fluctlight_id": id})); err != nil {
			return err
		}
		profileIDs := personalityProfileIDs(corePersona)
		defaultProfileID := initialPersonalityProfileID(corePersona)
		if err := a.insertAgency(ctx, tx, id, actorID, goals, intentions, defaultProfileID, profileIDs); err != nil {
			return err
		}
		if err := a.insertRelationshipSeeds(ctx, tx, id, actorID, foundation, defaultProfileID, profileIDs); err != nil {
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

func initialPersonalityProfileID(corePersona map[string]any) string {
	system := mapValue(corePersona["personality_system"])
	if active := strings.TrimSpace(stringValue(system["active_profile_id"])); active != "" {
		return active
	}
	for _, raw := range arrayValue(system["profiles"]) {
		if id := strings.TrimSpace(stringValue(mapValue(raw)["id"])); id != "" {
			return id
		}
	}
	return "default"
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
	if err == nil {
	}
	return id, err
}

func (a *App) insertAgency(ctx context.Context, tx pgx.Tx, fluctlightID, actorID string, goals, intentions []any, defaultProfileID string, profileIDs map[string]struct{}) error {
	goalIDs := make([]string, len(goals))
	for index, raw := range goals {
		item := mapValue(raw)
		goalIDs[index] = fmt.Sprintf("goal_initial_%s_%d", fluctlightID, index)
		profileID, _ := normalizeProfileID(stringValue(item["profile_id"]), defaultProfileID)
		if _, ok := profileIDs[profileID]; !ok {
			return errors.New("initial_goal_profile_invalid")
		}
		scope := firstString(item["scope"], "general")
		if scope != "general" && scope != "relationship" {
			return errors.New("initial_goal_scope_invalid")
		}
		targetActorIDValue := resolveInitializationActorRef(stringValue(item["target_actor_id"]), actorID, fluctlightID)
		targetActorID := nullableString(targetActorIDValue)
		if scope == "relationship" && targetActorID == nil {
			return errors.New("initial_relationship_goal_target_required")
		}
		if targetActorIDValue != "" {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.actors WHERE id=$1 AND status='active')`, targetActorIDValue).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_goals (id,fluctlight_id,profile_id,source,scope,target_actor_id,description,importance,urgency,progress,status,evidence_refs,revision) VALUES ($1,$2,$3,'self',$4,$5,$6,$7,$8,$9,'active',$10,0) ON CONFLICT DO NOTHING`, goalIDs[index], fluctlightID, profileID, scope, targetActorID, stringValue(item["description"]), jsonBytes(item["importance"]), jsonBytes(item["urgency"]), jsonBytes(0.0), jsonBytes([]string{"foundation:" + fluctlightID})); err != nil {
			return err
		}
	}
	for index, raw := range intentions {
		item := mapValue(raw)
		profileID, _ := normalizeProfileID(stringValue(item["profile_id"]), defaultProfileID)
		if _, ok := profileIDs[profileID]; !ok {
			return errors.New("initial_intention_profile_invalid")
		}
		goalIndex := intValue(item["goal_index"])
		if goalIndex < 0 || goalIndex >= len(goalIDs) {
			return errors.New("initial_intention_goal_invalid")
		}
		var goalProfileID string
		if err := tx.QueryRow(ctx, `SELECT COALESCE(profile_id,'') FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2`, goalIDs[goalIndex], fluctlightID).Scan(&goalProfileID); err != nil {
			return err
		}
		if goalProfileID != profileID {
			return errors.New("initial_intention_profile_goal_mismatch")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_intentions (id,fluctlight_id,profile_id,goal_id,action,trigger,confidence,expiration,evidence_refs,permission_snapshot,budget_snapshot,status,revision) VALUES ($1,$2,$3,$4,$5,$6,$7,now()+interval '24 hours',$8,'{}','{}','pending',0) ON CONFLICT DO NOTHING`, fmt.Sprintf("intention_initial_%s_%d", fluctlightID, index), fluctlightID, profileID, goalIDs[goalIndex], stringValue(item["action"]), jsonBytes(map[string]any{"type": "semantic", "schema_version": "semantic.trigger.v1", "evidence_refs": []string{"foundation:" + fluctlightID}}), jsonBytes(item["confidence"]), jsonBytes([]string{"foundation:" + fluctlightID})); err != nil {
			return err
		}
	}
	return nil
}

func resolveInitializationActorRef(value, humanActorID, fluctlightID string) string {
	switch strings.TrimSpace(value) {
	case "actor_user":
		return humanActorID
	case "actor_self":
		return fluctlightID
	default:
		return strings.TrimSpace(value)
	}
}

// resolveConversationActorAlias maps provider-facing group aliases back to
// real Actor IDs using the authoritative participant order. The model sees
// actor_user/actor_self/actor_b...; persistence and capability lookup always
// operate on the real IDs.
func (a *App) resolveConversationActorAlias(ctx context.Context, conversationID, humanActorID, fluctlightID, value string) string {
	value = strings.TrimSpace(value)
	if value == "actor_user" {
		return humanActorID
	}
	if value == "actor_self" {
		return fluctlightID
	}
	if !strings.HasPrefix(value, "actor_") || len(value) != len("actor_")+1 {
		return value
	}
	index := int(value[len(value)-1] - 'b')
	if index < 0 {
		return value
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT actor_id FROM public.conversation_participants WHERE conversation_id=$1 AND status='active' ORDER BY joined_at,actor_id`, conversationID)
	if err != nil {
		return value
	}
	defer rows.Close()
	others := make([]string, 0)
	for rows.Next() {
		var actorID string
		if rows.Scan(&actorID) != nil {
			return value
		}
		if actorID != humanActorID && actorID != fluctlightID {
			others = append(others, actorID)
		}
	}
	if index < len(others) {
		return others[index]
	}
	return value
}

func (a *App) insertRelationshipSeeds(ctx context.Context, tx pgx.Tx, fluctlightID, humanActorID string, foundation map[string]any, defaultProfileID string, profileIDs map[string]struct{}) error {
	seeds := arrayValue(foundation["initial_relationships"])
	if len(seeds) == 0 {
		lifeProfile := mapValue(mapValue(foundation["core_persona"])["life_profile"])
		seeds = arrayValue(lifeProfile["relationship_seeds"])
	}
	for index, raw := range seeds {
		item := mapValue(raw)
		profileID, _ := normalizeProfileID(stringValue(item["profile_id"]), defaultProfileID)
		if _, ok := profileIDs[profileID]; !ok {
			return errors.New("initial_relationship_profile_invalid")
		}
		target := resolveInitializationActorRef(stringValue(item["target_actor_id"]), humanActorID, fluctlightID)
		if target == "" {
			return errors.New("initial_relationship_target_required")
		}
		var actorType, status string
		if err := tx.QueryRow(ctx, `SELECT actor_type,status FROM public.actors WHERE id=$1`, target).Scan(&actorType, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status != "active" || (actorType != "human" && actorType != "fluctlight") {
			return errors.New("initial_relationship_target_invalid")
		}
		role, err := normalizeRelationshipRole(item["role"])
		if err != nil {
			return err
		}
		metrics, err := validateRelationshipMetrics(item["metrics"])
		if err != nil {
			return err
		}
		trend := firstString(item["trend"], "stable")
		if trend != "improving" && trend != "stable" && trend != "declining" {
			return errors.New("initial_relationship_trend_invalid")
		}
		refs := arrayValue(item["evidence_refs"])
		provenance := map[string]any{"source": "initialization", "evidence_refs": refs}
		relationshipID := "relationship_" + stableDigest(fluctlightID+":"+profileID+":"+target)
		if _, err := tx.Exec(ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,profile_id,target_actor_id,role,metrics,trend,summary,emotional_association,provenance,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,0) ON CONFLICT (id) DO NOTHING`, relationshipID, fluctlightID, profileID, target, jsonBytes(role), jsonBytes(metrics), trend, nullableString(stringValue(item["summary"])), jsonBytes(mapValue(item["emotional_association"])), jsonBytes(provenance)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.relationship_revisions(id,relationship_id,revision,base_revision,role,metrics,trend,summary,emotional_association,evidence_refs,actor_id,idempotency_key) VALUES($1,$2,0,0,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`, "relationship_revision_"+stableDigest(fluctlightID+":"+profileID+":"+target+":initial"), relationshipID, jsonBytes(role), jsonBytes(metrics), trend, nullableString(stringValue(item["summary"])), jsonBytes(mapValue(item["emotional_association"])), jsonBytes(refs), fluctlightID, "relationship-initialization:"+fluctlightID+":"+profileID+":"+target+":"+fmt.Sprint(index)); err != nil {
			return err
		}
	}
	return nil
}
