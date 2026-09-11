package core

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	sceneMutationPlanVersion    = "life.scene-mutation.v1"
	presenceMutationPlanVersion = "life.presence-mutation.v1"
	sceneDefaultDuration        = 2 * time.Hour
	presenceDefaultDuration     = 2 * time.Hour
	presenceMaxDuration         = 24 * time.Hour
)

type preparedSceneMutation struct {
	SchemaVersion               string    `json:"schema_version"`
	Operation                   string    `json:"operation"`
	Scene                       string    `json:"scene,omitempty"`
	Activity                    string    `json:"activity,omitempty"`
	Location                    string    `json:"location,omitempty"`
	Confidence                  float64   `json:"confidence"`
	EvidenceRefs                []string  `json:"evidence_refs"`
	OccurredAt                  time.Time `json:"occurred_at"`
	EndsAt                      time.Time `json:"ends_at"`
	ExpectedLifeContextRevision string    `json:"expected_life_context_revision"`
	ExpectedSource              string    `json:"expected_source"`
	ExpectedEventID             string    `json:"expected_event_id,omitempty"`
	ExpectedEventRevision       int       `json:"expected_event_revision,omitempty"`
	IdempotencyKey              string    `json:"idempotency_key"`
	RequestDigest               string    `json:"request_digest"`
}

type preparedPresenceMutation struct {
	SchemaVersion               string     `json:"schema_version"`
	Operation                   string     `json:"operation"`
	ActorID                     string     `json:"actor_id"`
	CurrentTask                 string     `json:"current_task,omitempty"`
	UserPresence                string     `json:"user_presence,omitempty"`
	Confidence                  float64    `json:"confidence"`
	EvidenceRefs                []string   `json:"evidence_refs"`
	OccurredAt                  time.Time  `json:"occurred_at"`
	ExpiresAt                   *time.Time `json:"expires_at,omitempty"`
	ExpectedLifeContextRevision string     `json:"expected_life_context_revision"`
	IdempotencyKey              string     `json:"idempotency_key"`
	RequestDigest               string     `json:"request_digest"`
}

func sceneMutationDigest(plan preparedSceneMutation) string {
	plan.RequestDigest = ""
	return stableDigest(jsonString(plan))
}

func presenceMutationDigest(plan preparedPresenceMutation) string {
	plan.RequestDigest = ""
	return stableDigest(jsonString(plan))
}

func (a *App) prepareSceneCapability(_ context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if err := requireCapabilityContext(resolved, SlotCurrentLife); err != nil {
		return invocation, err
	}
	if strings.TrimSpace(invocation.ActionID) == "" || len(invocation.ContextSnapshot) == 0 {
		return invocation, errors.New("scene_frozen_action_required")
	}
	args, err := capabilityExecutionArguments(invocation, sceneCapabilityDefinition())
	if err != nil {
		return invocation, err
	}
	operation, err := normalizeSceneOperation(args)
	if err != nil {
		return invocation, err
	}
	scene := strings.TrimSpace(stringValue(args["scene"]))
	activity := strings.TrimSpace(stringValue(args["activity"]))
	if operation != "end" && (scene == "" || activity == "") {
		return invocation, errors.New("scene_fields_required")
	}
	confidence, err := boundedNumberOrError(args["confidence"], -1)
	if err != nil || confidence < 0 {
		return invocation, errors.New("scene_confidence_invalid")
	}
	life := resolved.Life.Data
	expectedRevision := stringValue(life["context_revision"])
	expectedSource := stringValue(life["source"])
	if expectedRevision == "" {
		return invocation, errors.New("scene_context_revision_required")
	}
	if operation == "start" && expectedSource == "event" {
		return invocation, errors.New("scene_start_requires_no_active_event")
	}
	if operation == "end" && expectedSource != "event" {
		return invocation, errors.New("scene_end_requires_active_event")
	}
	evidence := sortedUniqueStrings(decisionServiceRefValues(args["evidence_refs"]))
	if len(evidence) != 1 || evidence[0] != invocation.SourceFactID {
		return invocation, errors.New("scene_runtime_evidence_invalid")
	}
	occurredAt := time.Now().UTC()
	plan := preparedSceneMutation{
		SchemaVersion: sceneMutationPlanVersion, Operation: operation, Scene: scene, Activity: activity,
		Location: stringValue(args["location"]), Confidence: confidence, EvidenceRefs: evidence,
		OccurredAt: occurredAt, EndsAt: occurredAt.Add(sceneDefaultDuration),
		ExpectedLifeContextRevision: expectedRevision, ExpectedSource: expectedSource,
		ExpectedEventID: stringValue(life["event_id"]), ExpectedEventRevision: intValue(life["event_revision"]),
		IdempotencyKey: "scene:" + stableDigest(invocation.Metadata.FluctlightID+"\x1f"+invocation.ActionID+"\x1f"+invocation.CallID),
	}
	plan.RequestDigest = sceneMutationDigest(plan)
	return withCapabilityPreparedData(invocation, "scene_plan", plan)
}

func (a *App) preparePresenceCapability(_ context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if err := requireCapabilityContext(resolved, SlotCurrentLife); err != nil {
		return invocation, err
	}
	if strings.TrimSpace(invocation.ActionID) == "" || len(invocation.ContextSnapshot) == 0 {
		return invocation, errors.New("presence_frozen_action_required")
	}
	args, err := capabilityExecutionArguments(invocation, presenceCapabilityDefinition())
	if err != nil {
		return invocation, err
	}
	operation := firstString(args["operation"], "set")
	if operation != "set" && operation != "clear" {
		return invocation, errors.New("presence_operation_invalid")
	}
	currentTask := stringValue(args["current_task"])
	userPresence := stringValue(args["user_presence"])
	if operation == "set" && currentTask == "" && userPresence == "" {
		return invocation, errors.New("presence_fields_required")
	}
	if operation == "clear" && (currentTask != "" || userPresence != "" || stringValue(args["expires_at"]) != "") {
		return invocation, errors.New("presence_clear_fields_forbidden")
	}
	confidence, err := boundedNumberOrError(args["confidence"], -1)
	if err != nil || confidence < 0 {
		return invocation, errors.New("presence_confidence_invalid")
	}
	index, err := contextReferenceIndexFromValue(invocation.ContextSnapshot["context_reference_index"])
	if err != nil {
		return invocation, err
	}
	actorID := strings.TrimSpace(index.SpeakerActorID)
	if actorID == "" {
		actorID = strings.TrimSpace(index.OwnerActorID)
	}
	if actorID == "" {
		return invocation, errors.New("presence_actor_required")
	}
	evidence := sortedUniqueStrings(decisionServiceRefValues(args["evidence_refs"]))
	if len(evidence) != 1 || evidence[0] != invocation.SourceFactID {
		return invocation, errors.New("presence_runtime_evidence_invalid")
	}
	occurredAt := time.Now().UTC()
	var expiresAt *time.Time
	if operation == "set" {
		expires := occurredAt.Add(presenceDefaultDuration)
		if raw := stringValue(args["expires_at"]); raw != "" {
			parsed, parseErr := time.Parse(time.RFC3339, raw)
			if parseErr != nil || !parsed.After(occurredAt) || parsed.After(occurredAt.Add(presenceMaxDuration)) {
				return invocation, errors.New("presence_expiration_invalid")
			}
			expires = parsed.UTC()
		}
		expiresAt = &expires
	}
	plan := preparedPresenceMutation{
		SchemaVersion: presenceMutationPlanVersion, Operation: operation, ActorID: actorID,
		CurrentTask: currentTask, UserPresence: userPresence, Confidence: confidence, EvidenceRefs: evidence,
		OccurredAt: occurredAt, ExpiresAt: expiresAt,
		ExpectedLifeContextRevision: stringValue(resolved.Life.Data["context_revision"]),
		IdempotencyKey:              "presence:" + stableDigest(invocation.Metadata.FluctlightID+"\x1f"+invocation.ActionID+"\x1f"+invocation.CallID),
	}
	if plan.ExpectedLifeContextRevision == "" {
		return invocation, errors.New("presence_context_revision_required")
	}
	plan.RequestDigest = presenceMutationDigest(plan)
	return withCapabilityPreparedData(invocation, "presence_plan", plan)
}

func scenePlanFromInvocation(invocation CapabilityInvocation) (preparedSceneMutation, error) {
	raw, found, err := capabilityPreparedData(invocation, "scene_plan")
	if err != nil || !found {
		return preparedSceneMutation{}, errors.New("scene_plan_invalid")
	}
	var plan preparedSceneMutation
	if jsonUnmarshal(jsonBytes(raw), &plan) != nil || plan.SchemaVersion != sceneMutationPlanVersion || plan.RequestDigest != sceneMutationDigest(plan) || plan.OccurredAt.IsZero() || !plan.EndsAt.After(plan.OccurredAt) || plan.ExpectedLifeContextRevision == "" || plan.IdempotencyKey == "" || len(plan.EvidenceRefs) == 0 {
		return preparedSceneMutation{}, errors.New("scene_plan_invalid")
	}
	return plan, nil
}

func presencePlanFromInvocation(invocation CapabilityInvocation) (preparedPresenceMutation, error) {
	raw, found, err := capabilityPreparedData(invocation, "presence_plan")
	if err != nil || !found {
		return preparedPresenceMutation{}, errors.New("presence_plan_invalid")
	}
	var plan preparedPresenceMutation
	if jsonUnmarshal(jsonBytes(raw), &plan) != nil || plan.SchemaVersion != presenceMutationPlanVersion || plan.RequestDigest != presenceMutationDigest(plan) || plan.OccurredAt.IsZero() || plan.ExpectedLifeContextRevision == "" || plan.IdempotencyKey == "" || len(plan.EvidenceRefs) == 0 {
		return preparedPresenceMutation{}, errors.New("presence_plan_invalid")
	}
	return plan, nil
}
