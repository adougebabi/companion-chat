package core

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	goalAuthoritySchemaVersion      = "fluctlight.goal.v2"
	intentionAuthoritySchemaVersion = "fluctlight.intention.v2"
	intentionDueFactType            = "agency.intention_due"
	goalProgressPolicyVersion       = "goal.progress.v1"
)

type GoalLifecycleStatus string

const (
	GoalCandidate GoalLifecycleStatus = "candidate"
	GoalActive    GoalLifecycleStatus = "active"
	GoalPaused    GoalLifecycleStatus = "paused"
	GoalCompleted GoalLifecycleStatus = "completed"
	GoalAbandoned GoalLifecycleStatus = "abandoned"
	GoalCancelled GoalLifecycleStatus = "cancelled"
)

type GoalLifecycleOperation string

const (
	GoalCreate   GoalLifecycleOperation = "create"
	GoalUpdate   GoalLifecycleOperation = "update"
	GoalPause    GoalLifecycleOperation = "pause"
	GoalResume   GoalLifecycleOperation = "resume"
	GoalComplete GoalLifecycleOperation = "complete"
	GoalAbandon  GoalLifecycleOperation = "abandon"
	GoalCancel   GoalLifecycleOperation = "cancel"
)

type GoalAuthority struct {
	EntityID        string              `json:"-"`
	SchemaVersion   string              `json:"schema_version"`
	Ref             string              `json:"ref"`
	FluctlightID    string              `json:"fluctlight_id"`
	ProfileID       string              `json:"profile_id"`
	DesiredOutcome  string              `json:"desired_outcome"`
	SuccessCriteria []string            `json:"success_criteria"`
	Motivation      string              `json:"motivation"`
	Scope           string              `json:"scope"`
	TargetActorID   string              `json:"-"`
	TargetActorRef  string              `json:"target_actor_ref,omitempty"`
	Importance      float64             `json:"importance"`
	Urgency         float64             `json:"urgency"`
	Progress        float64             `json:"progress"`
	Deadline        *time.Time          `json:"deadline,omitempty"`
	NeedsReflection bool                `json:"needs_reflection"`
	Status          GoalLifecycleStatus `json:"status"`
	Revision        int                 `json:"revision"`
	EvidenceRefs    []string            `json:"evidence_refs"`
}

func CreateGoalAuthority(goal GoalAuthority, evidenceRefs []string, occurredAt time.Time) (GoalAuthority, GoalGovernanceRecord, error) {
	if goal.Revision != 1 || occurredAt.IsZero() || len(evidenceRefs) == 0 {
		return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_create_invalid")
	}
	goal.EvidenceRefs = mergeStableRefs(goal.EvidenceRefs, evidenceRefs)
	if err := goal.Validate(); err != nil {
		return GoalAuthority{}, GoalGovernanceRecord{}, err
	}
	record := GoalGovernanceRecord{GoalRef: goal.Ref, Operation: GoalCreate, FromStatus: "", ToStatus: goal.Status, BaseRevision: 0, Revision: 1, EvidenceRefs: mergeStableRefs(evidenceRefs), Reason: "goal created", PolicyVersion: goalProgressPolicyVersion, OccurredAt: occurredAt.UTC()}
	return goal, record, nil
}

type GoalPatch struct {
	DesiredOutcome  *string
	SuccessCriteria []string
	Motivation      *string
	Scope           *string
	TargetActorRef  *string
	Importance      *float64
	Urgency         *float64
	Deadline        *time.Time
	ClearDeadline   bool
}

type GoalCommand struct {
	Operation        GoalLifecycleOperation
	ExpectedRevision int
	Patch            GoalPatch
	EvidenceRefs     []string
	Reason           string
	OccurredAt       time.Time
}

type GoalGovernanceRecord struct {
	GoalRef       string                 `json:"goal_ref"`
	Operation     GoalLifecycleOperation `json:"operation"`
	FromStatus    GoalLifecycleStatus    `json:"from_status"`
	ToStatus      GoalLifecycleStatus    `json:"to_status"`
	BaseRevision  int                    `json:"base_revision"`
	Revision      int                    `json:"revision"`
	EvidenceRefs  []string               `json:"evidence_refs"`
	Reason        string                 `json:"reason"`
	PolicyVersion string                 `json:"policy_version"`
	OccurredAt    time.Time              `json:"occurred_at"`
}

func (goal GoalAuthority) Validate() error {
	if goal.SchemaVersion != goalAuthoritySchemaVersion || !validAgencyReference(goal.Ref, ContextReferenceGoal) || strings.TrimSpace(goal.FluctlightID) == "" || strings.TrimSpace(goal.ProfileID) == "" {
		return errors.New("goal_identity_invalid")
	}
	if text := strings.TrimSpace(goal.DesiredOutcome); text == "" || len([]rune(text)) > 2000 {
		return errors.New("goal_desired_outcome_invalid")
	}
	if len(goal.SuccessCriteria) > 16 || (len(goal.SuccessCriteria) == 0 && !goal.NeedsReflection) {
		return errors.New("goal_success_criteria_invalid")
	}
	for _, criterion := range goal.SuccessCriteria {
		if text := strings.TrimSpace(criterion); text == "" || len([]rune(text)) > 500 {
			return errors.New("goal_success_criterion_invalid")
		}
	}
	if strings.TrimSpace(goal.Motivation) == "" || len([]rune(goal.Motivation)) > 1000 || strings.TrimSpace(goal.Scope) == "" || len([]rune(goal.Scope)) > 128 {
		return errors.New("goal_semantics_invalid")
	}
	if goal.TargetActorRef != "" && !contextReferencePattern.MatchString(goal.TargetActorRef) {
		return errors.New("goal_target_actor_ref_invalid")
	}
	if !unitFinite(goal.Importance) || !unitFinite(goal.Urgency) || !unitFinite(goal.Progress) {
		return errors.New("goal_numeric_invalid")
	}
	switch goal.Status {
	case GoalCandidate, GoalActive, GoalPaused, GoalCompleted, GoalAbandoned, GoalCancelled:
	default:
		return errors.New("goal_status_invalid")
	}
	if goal.Revision < 1 || len(goal.EvidenceRefs) == 0 || len(goal.EvidenceRefs) > 64 {
		return errors.New("goal_revision_evidence_invalid")
	}
	return validateBoundedRefs(goal.EvidenceRefs)
}

func ApplyGoalCommand(current *GoalAuthority, command GoalCommand) (GoalAuthority, GoalGovernanceRecord, error) {
	at := command.OccurredAt.UTC()
	if at.IsZero() || len(command.EvidenceRefs) == 0 || validateBoundedRefs(command.EvidenceRefs) != nil {
		return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_command_evidence_invalid")
	}
	if current == nil {
		return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_current_required")
	}
	if err := current.Validate(); err != nil {
		return GoalAuthority{}, GoalGovernanceRecord{}, err
	}
	if command.Operation == GoalCreate || command.ExpectedRevision != current.Revision {
		return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_revision_conflict")
	}
	next := *current
	next.SuccessCriteria = append([]string(nil), current.SuccessCriteria...)
	next.EvidenceRefs = mergeStableRefs(current.EvidenceRefs, command.EvidenceRefs)
	from := current.Status
	if goalStatusTerminal(from) {
		return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_terminal")
	}
	switch command.Operation {
	case GoalUpdate:
		applyGoalPatch(&next, command.Patch)
	case GoalPause:
		if from != GoalActive {
			return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_transition_invalid")
		}
		next.Status = GoalPaused
	case GoalResume:
		if from != GoalPaused && from != GoalCandidate {
			return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_transition_invalid")
		}
		next.Status = GoalActive
	case GoalComplete:
		if next.Progress < 1 {
			return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_success_criteria_unsatisfied")
		}
		next.Status = GoalCompleted
	case GoalAbandon:
		next.Status = GoalAbandoned
	case GoalCancel:
		next.Status = GoalCancelled
	default:
		return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_operation_invalid")
	}
	next.Revision++
	if err := next.Validate(); err != nil {
		return GoalAuthority{}, GoalGovernanceRecord{}, err
	}
	record := GoalGovernanceRecord{
		GoalRef: next.Ref, Operation: command.Operation, FromStatus: from, ToStatus: next.Status,
		BaseRevision: current.Revision, Revision: next.Revision, EvidenceRefs: append([]string(nil), command.EvidenceRefs...),
		Reason: strings.TrimSpace(command.Reason), PolicyVersion: goalProgressPolicyVersion, OccurredAt: at,
	}
	return next, record, nil
}

func applyGoalPatch(goal *GoalAuthority, patch GoalPatch) {
	if patch.DesiredOutcome != nil {
		goal.DesiredOutcome = strings.TrimSpace(*patch.DesiredOutcome)
	}
	if patch.SuccessCriteria != nil {
		goal.SuccessCriteria = append([]string(nil), patch.SuccessCriteria...)
		goal.NeedsReflection = len(patch.SuccessCriteria) == 0
	}
	if patch.Motivation != nil {
		goal.Motivation = strings.TrimSpace(*patch.Motivation)
	}
	if patch.Scope != nil {
		goal.Scope = strings.TrimSpace(*patch.Scope)
	}
	if patch.TargetActorRef != nil {
		goal.TargetActorRef = strings.TrimSpace(*patch.TargetActorRef)
	}
	if patch.Importance != nil {
		goal.Importance = *patch.Importance
	}
	if patch.Urgency != nil {
		goal.Urgency = *patch.Urgency
	}
	if patch.ClearDeadline {
		goal.Deadline = nil
	} else if patch.Deadline != nil {
		deadline := patch.Deadline.UTC()
		goal.Deadline = &deadline
	}
}

type GoalProgressProposal struct {
	GoalRef          string
	OutcomeRefs      []string
	CriterionIndexes []int
	Strength         float64
	Confidence       float64
	Complete         bool
	EvidenceRefs     []string
	OccurredAt       time.Time
}

func ApplyGoalProgress(goal GoalAuthority, proposal GoalProgressProposal, outcomes map[string]ActionOutcome) (GoalAuthority, GoalGovernanceRecord, error) {
	if err := goal.Validate(); err != nil {
		return GoalAuthority{}, GoalGovernanceRecord{}, err
	}
	if goalStatusTerminal(goal.Status) || proposal.GoalRef != goal.Ref || !unitFinite(proposal.Strength) || !unitFinite(proposal.Confidence) || proposal.Confidence == 0 || proposal.OccurredAt.IsZero() {
		return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_progress_proposal_invalid")
	}
	if len(proposal.OutcomeRefs) == 0 || len(proposal.CriterionIndexes) == 0 {
		return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_progress_evidence_required")
	}
	for _, ref := range proposal.OutcomeRefs {
		outcome, ok := outcomes[ref]
		if !ok || outcome.Status != ActionOutcomeCompleted || !containsString(outcome.GoalRefs, goal.Ref) {
			return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_progress_outcome_not_successful")
		}
	}
	criteria := make(map[int]struct{}, len(proposal.CriterionIndexes))
	for _, index := range proposal.CriterionIndexes {
		if index < 0 || index >= len(goal.SuccessCriteria) {
			return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_progress_criterion_invalid")
		}
		criteria[index] = struct{}{}
	}
	evidenceFloor := float64(len(criteria)) / float64(len(goal.SuccessCriteria))
	semanticStep := goal.Progress + 0.25*proposal.Strength*proposal.Confidence
	nextProgress := math.Min(1, math.Max(goal.Progress, math.Max(evidenceFloor, semanticStep)))
	if proposal.Complete {
		if len(criteria) != len(goal.SuccessCriteria) {
			return GoalAuthority{}, GoalGovernanceRecord{}, errors.New("goal_completion_criteria_incomplete")
		}
		nextProgress = 1
	}
	next := goal
	next.Progress = nextProgress
	next.Revision++
	next.EvidenceRefs = mergeStableRefs(goal.EvidenceRefs, mergeStableRefs(proposal.EvidenceRefs, proposal.OutcomeRefs))
	if proposal.Complete {
		next.Status = GoalCompleted
	}
	if err := next.Validate(); err != nil {
		return GoalAuthority{}, GoalGovernanceRecord{}, err
	}
	record := GoalGovernanceRecord{
		GoalRef: goal.Ref, Operation: GoalUpdate, FromStatus: goal.Status, ToStatus: next.Status,
		BaseRevision: goal.Revision, Revision: next.Revision, EvidenceRefs: mergeStableRefs(proposal.EvidenceRefs, proposal.OutcomeRefs),
		Reason: "evidence-backed outcome progress", PolicyVersion: goalProgressPolicyVersion, OccurredAt: proposal.OccurredAt.UTC(),
	}
	return next, record, nil
}

type IntentionLifecycleStatus string

const (
	IntentionCandidate  IntentionLifecycleStatus = "candidate"
	IntentionQualified  IntentionLifecycleStatus = "qualified"
	IntentionDue        IntentionLifecycleStatus = "due"
	IntentionInProgress IntentionLifecycleStatus = "in_progress"
	IntentionPaused     IntentionLifecycleStatus = "paused"
	IntentionCompleted  IntentionLifecycleStatus = "completed"
	IntentionExpired    IntentionLifecycleStatus = "expired"
	IntentionCancelled  IntentionLifecycleStatus = "cancelled"
)

type IntentionLifecycleOperation string

const (
	IntentionCreate   IntentionLifecycleOperation = "create"
	IntentionUpdate   IntentionLifecycleOperation = "update"
	IntentionQualify  IntentionLifecycleOperation = "qualify"
	IntentionMarkDue  IntentionLifecycleOperation = "mark_due"
	IntentionPause    IntentionLifecycleOperation = "pause"
	IntentionResume   IntentionLifecycleOperation = "resume"
	IntentionComplete IntentionLifecycleOperation = "complete"
	IntentionExpire   IntentionLifecycleOperation = "expire"
	IntentionCancel   IntentionLifecycleOperation = "cancel"
)

type IntentionTriggerType string

const (
	IntentionTriggerTime     IntentionTriggerType = "time"
	IntentionTriggerEvent    IntentionTriggerType = "event"
	IntentionTriggerSemantic IntentionTriggerType = "semantic"
)

type TypedIntentionTrigger struct {
	Type      IntentionTriggerType `json:"type"`
	DueAt     *time.Time           `json:"due_at,omitempty"`
	EventType string               `json:"event_type,omitempty"`
	EventRef  string               `json:"event_ref,omitempty"`
}

func (trigger TypedIntentionTrigger) Validate() error {
	switch trigger.Type {
	case IntentionTriggerTime:
		if trigger.DueAt == nil || trigger.DueAt.IsZero() || trigger.EventType != "" || trigger.EventRef != "" {
			return errors.New("intention_time_trigger_invalid")
		}
	case IntentionTriggerEvent:
		if strings.TrimSpace(trigger.EventType) == "" || len([]rune(trigger.EventType)) > 128 || trigger.DueAt != nil || (trigger.EventRef != "" && !contextReferencePattern.MatchString(trigger.EventRef)) {
			return errors.New("intention_event_trigger_invalid")
		}
	case IntentionTriggerSemantic:
		if trigger.DueAt != nil || trigger.EventType != "" || trigger.EventRef != "" {
			return errors.New("intention_semantic_trigger_invalid")
		}
	default:
		return errors.New("intention_trigger_type_invalid")
	}
	return nil
}

type IntentionAuthority struct {
	EntityID              string                   `json:"-"`
	GoalEntityID          string                   `json:"-"`
	SchemaVersion         string                   `json:"schema_version"`
	Ref                   string                   `json:"ref"`
	FluctlightID          string                   `json:"fluctlight_id"`
	ProfileID             string                   `json:"profile_id"`
	GoalRef               string                   `json:"goal_ref"`
	ActionIntent          string                   `json:"action_intent"`
	ExpectedOutcome       string                   `json:"expected_outcome"`
	CapabilityConstraints []string                 `json:"capability_constraints"`
	Trigger               TypedIntentionTrigger    `json:"typed_trigger"`
	PreferredTime         *time.Time               `json:"preferred_time,omitempty"`
	Expiration            time.Time                `json:"expiration"`
	Confidence            float64                  `json:"confidence"`
	Status                IntentionLifecycleStatus `json:"status"`
	Revision              int                      `json:"revision"`
	EvidenceRefs          []string                 `json:"evidence_refs"`
	LastAttemptID         string                   `json:"last_attempt_id,omitempty"`
	LastAttemptStatus     string                   `json:"last_attempt_status,omitempty"`
}

type IntentionPatch struct {
	GoalRef               *string
	GoalEntityID          *string
	ActionIntent          *string
	ExpectedOutcome       *string
	CapabilityConstraints []string
	Trigger               *TypedIntentionTrigger
	PreferredTime         *time.Time
	ClearPreferredTime    bool
	Expiration            *time.Time
	Confidence            *float64
}

type IntentionCommand struct {
	Operation        IntentionLifecycleOperation
	ExpectedRevision int
	Patch            IntentionPatch
	EvidenceRefs     []string
	Reason           string
	OccurredAt       time.Time
}

type IntentionGovernanceRecord struct {
	IntentionRef  string                      `json:"intention_ref"`
	Operation     IntentionLifecycleOperation `json:"operation"`
	FromStatus    IntentionLifecycleStatus    `json:"from_status"`
	ToStatus      IntentionLifecycleStatus    `json:"to_status"`
	BaseRevision  int                         `json:"base_revision"`
	Revision      int                         `json:"revision"`
	EvidenceRefs  []string                    `json:"evidence_refs"`
	Reason        string                      `json:"reason"`
	PolicyVersion string                      `json:"policy_version"`
	OccurredAt    time.Time                   `json:"occurred_at"`
}

func CreateIntentionAuthority(intention IntentionAuthority, evidenceRefs []string, occurredAt time.Time) (IntentionAuthority, IntentionGovernanceRecord, error) {
	if intention.Revision != 1 || occurredAt.IsZero() || len(evidenceRefs) == 0 {
		return IntentionAuthority{}, IntentionGovernanceRecord{}, errors.New("intention_create_invalid")
	}
	intention.EvidenceRefs = mergeStableRefs(intention.EvidenceRefs, evidenceRefs)
	if err := intention.Validate(); err != nil {
		return IntentionAuthority{}, IntentionGovernanceRecord{}, err
	}
	record := IntentionGovernanceRecord{IntentionRef: intention.Ref, Operation: IntentionCreate, ToStatus: intention.Status, BaseRevision: 0, Revision: 1, EvidenceRefs: mergeStableRefs(evidenceRefs), Reason: "intention created", PolicyVersion: goalProgressPolicyVersion, OccurredAt: occurredAt.UTC()}
	return intention, record, nil
}

func ApplyIntentionCommand(current IntentionAuthority, command IntentionCommand) (IntentionAuthority, IntentionGovernanceRecord, error) {
	if err := current.Validate(); err != nil {
		return IntentionAuthority{}, IntentionGovernanceRecord{}, err
	}
	if command.Operation == IntentionCreate || command.ExpectedRevision != current.Revision || command.OccurredAt.IsZero() || len(command.EvidenceRefs) == 0 || validateBoundedRefs(command.EvidenceRefs) != nil {
		return IntentionAuthority{}, IntentionGovernanceRecord{}, errors.New("intention_command_invalid")
	}
	if intentionStatusTerminal(current.Status) {
		return IntentionAuthority{}, IntentionGovernanceRecord{}, errors.New("intention_terminal")
	}
	next := current
	next.CapabilityConstraints = append([]string(nil), current.CapabilityConstraints...)
	next.EvidenceRefs = mergeStableRefs(current.EvidenceRefs, command.EvidenceRefs)
	from := current.Status
	switch command.Operation {
	case IntentionUpdate:
		applyIntentionPatch(&next, command.Patch)
	case IntentionQualify:
		if from != IntentionCandidate {
			return IntentionAuthority{}, IntentionGovernanceRecord{}, errors.New("intention_transition_invalid")
		}
		next.Status = IntentionQualified
	case IntentionMarkDue:
		if from != IntentionQualified {
			return IntentionAuthority{}, IntentionGovernanceRecord{}, errors.New("intention_transition_invalid")
		}
		next.Status = IntentionDue
	case IntentionPause:
		if from != IntentionQualified && from != IntentionDue && from != IntentionInProgress {
			return IntentionAuthority{}, IntentionGovernanceRecord{}, errors.New("intention_transition_invalid")
		}
		next.Status = IntentionPaused
	case IntentionResume:
		if from != IntentionPaused {
			return IntentionAuthority{}, IntentionGovernanceRecord{}, errors.New("intention_transition_invalid")
		}
		next.Status = IntentionQualified
	case IntentionComplete:
		next.Status = IntentionCompleted
	case IntentionExpire:
		next.Status = IntentionExpired
	case IntentionCancel:
		next.Status = IntentionCancelled
	default:
		return IntentionAuthority{}, IntentionGovernanceRecord{}, errors.New("intention_operation_invalid")
	}
	next.Revision++
	if err := next.Validate(); err != nil {
		return IntentionAuthority{}, IntentionGovernanceRecord{}, err
	}
	record := IntentionGovernanceRecord{IntentionRef: next.Ref, Operation: command.Operation, FromStatus: from, ToStatus: next.Status, BaseRevision: current.Revision, Revision: next.Revision, EvidenceRefs: mergeStableRefs(command.EvidenceRefs), Reason: strings.TrimSpace(command.Reason), PolicyVersion: goalProgressPolicyVersion, OccurredAt: command.OccurredAt.UTC()}
	return next, record, nil
}

func applyIntentionPatch(intention *IntentionAuthority, patch IntentionPatch) {
	if patch.GoalRef != nil {
		intention.GoalRef = strings.TrimSpace(*patch.GoalRef)
	}
	if patch.GoalEntityID != nil {
		intention.GoalEntityID = strings.TrimSpace(*patch.GoalEntityID)
	}
	if patch.ActionIntent != nil {
		intention.ActionIntent = strings.TrimSpace(*patch.ActionIntent)
	}
	if patch.ExpectedOutcome != nil {
		intention.ExpectedOutcome = strings.TrimSpace(*patch.ExpectedOutcome)
	}
	if patch.CapabilityConstraints != nil {
		intention.CapabilityConstraints = append([]string(nil), patch.CapabilityConstraints...)
	}
	if patch.Trigger != nil {
		intention.Trigger = *patch.Trigger
	}
	if patch.ClearPreferredTime {
		intention.PreferredTime = nil
	} else if patch.PreferredTime != nil {
		value := patch.PreferredTime.UTC()
		intention.PreferredTime = &value
	}
	if patch.Expiration != nil {
		intention.Expiration = patch.Expiration.UTC()
	}
	if patch.Confidence != nil {
		intention.Confidence = *patch.Confidence
	}
}

func intentionStatusTerminal(status IntentionLifecycleStatus) bool {
	return status == IntentionCompleted || status == IntentionExpired || status == IntentionCancelled
}

func (intention IntentionAuthority) Validate() error {
	if intention.SchemaVersion != intentionAuthoritySchemaVersion || !validAgencyReference(intention.Ref, ContextReferenceIntention) || !validAgencyReference(intention.GoalRef, ContextReferenceGoal) || strings.TrimSpace(intention.FluctlightID) == "" || strings.TrimSpace(intention.ProfileID) == "" {
		return errors.New("intention_identity_invalid")
	}
	if text := strings.TrimSpace(intention.ActionIntent); text == "" || len([]rune(text)) > 2000 {
		return errors.New("intention_action_intent_invalid")
	}
	if text := strings.TrimSpace(intention.ExpectedOutcome); text == "" || len([]rune(text)) > 2000 {
		return errors.New("intention_expected_outcome_invalid")
	}
	if len(intention.CapabilityConstraints) > 16 {
		return errors.New("intention_capability_constraints_invalid")
	}
	seen := map[string]struct{}{}
	for _, capability := range intention.CapabilityConstraints {
		name := strings.TrimSpace(capability)
		if name == "" || len([]rune(name)) > 128 {
			return errors.New("intention_capability_constraint_invalid")
		}
		if _, exists := seen[name]; exists {
			return errors.New("intention_capability_constraint_duplicate")
		}
		seen[name] = struct{}{}
	}
	if err := intention.Trigger.Validate(); err != nil {
		return err
	}
	if intention.Expiration.IsZero() || !unitFinite(intention.Confidence) || intention.Revision < 1 || len(intention.EvidenceRefs) == 0 || len(intention.EvidenceRefs) > 64 {
		return errors.New("intention_contract_invalid")
	}
	switch intention.Status {
	case IntentionCandidate, IntentionQualified, IntentionDue, IntentionInProgress, IntentionPaused, IntentionCompleted, IntentionExpired, IntentionCancelled:
	default:
		return errors.New("intention_status_invalid")
	}
	return validateBoundedRefs(intention.EvidenceRefs)
}

type IntentionTriggerObservation struct {
	At         time.Time
	EventType  string
	EventRef   string
	NewFactRef string
}

type IntentionDueFact struct {
	ID                string    `json:"id"`
	EventType         string    `json:"event_type"`
	IntentionRef      string    `json:"intention_ref"`
	GoalRef           string    `json:"goal_ref"`
	IntentionRevision int       `json:"intention_revision"`
	AttemptID         string    `json:"attempt_id"`
	TriggerType       string    `json:"trigger_type"`
	SourceRef         string    `json:"source_ref,omitempty"`
	OccurredAt        time.Time `json:"occurred_at"`
}

func EvaluateIntentionDue(intention IntentionAuthority, observation IntentionTriggerObservation) (IntentionDueFact, bool, error) {
	if err := intention.Validate(); err != nil {
		return IntentionDueFact{}, false, err
	}
	if intention.Status != IntentionQualified && intention.Status != IntentionDue {
		return IntentionDueFact{}, false, nil
	}
	at := observation.At.UTC()
	if at.IsZero() {
		return IntentionDueFact{}, false, errors.New("intention_trigger_observation_time_required")
	}
	if !at.Before(intention.Expiration) {
		return IntentionDueFact{}, false, errors.New("intention_expired")
	}
	due := false
	sourceRef := ""
	switch intention.Trigger.Type {
	case IntentionTriggerTime:
		due = intention.Trigger.DueAt != nil && !at.Before(intention.Trigger.DueAt.UTC())
	case IntentionTriggerEvent:
		due = observation.EventType == intention.Trigger.EventType && (intention.Trigger.EventRef == "" || observation.EventRef == intention.Trigger.EventRef)
		sourceRef = observation.EventRef
	case IntentionTriggerSemantic:
		due = strings.TrimSpace(observation.NewFactRef) != ""
		sourceRef = strings.TrimSpace(observation.NewFactRef)
	}
	if !due {
		return IntentionDueFact{}, false, nil
	}
	if intention.Status == IntentionDue && strings.HasPrefix(intention.LastAttemptID, "intention_attempt_") {
		return IntentionDueFact{
			ID: "intention_due_" + strings.TrimPrefix(intention.LastAttemptID, "intention_attempt_"), EventType: intentionDueFactType,
			IntentionRef: intention.Ref, GoalRef: intention.GoalRef, IntentionRevision: intention.Revision,
			AttemptID: intention.LastAttemptID, TriggerType: string(intention.Trigger.Type), SourceRef: sourceRef, OccurredAt: at,
		}, true, nil
	}
	identity := stableDigest(intention.Ref + "\x1f" + fmt.Sprint(intention.Revision) + "\x1f" + string(intention.Trigger.Type))
	return IntentionDueFact{
		ID: "intention_due_" + identity, EventType: intentionDueFactType, IntentionRef: intention.Ref, GoalRef: intention.GoalRef,
		IntentionRevision: intention.Revision, AttemptID: "intention_attempt_" + identity, TriggerType: string(intention.Trigger.Type),
		SourceRef: sourceRef, OccurredAt: at,
	}, true, nil
}

type AgencyExecutionGate struct {
	Now                  time.Time
	PermissionAllowed    bool
	BudgetAvailable      bool
	QuietHoursBlocked    bool
	CooldownUntil        *time.Time
	FoundationRevision   int
	CurrentStateRevision int
	LifeContextRevision  string
}

type FrozenIntentionAction struct {
	IntentionRef      string              `json:"intention_ref"`
	GoalRef           string              `json:"goal_ref"`
	AttemptID         string              `json:"attempt_id"`
	ActionID          string              `json:"action_id"`
	OutcomeID         string              `json:"outcome_id"`
	CapabilityName    string              `json:"capability_name"`
	ExpectedOutcome   string              `json:"expected_outcome"`
	SuccessCriteria   []string            `json:"success_criteria"`
	ExpectedRevisions map[string]any      `json:"expected_revisions"`
	Influences        []DecisionInfluence `json:"influences"`
}

func FreezeIntentionAction(goal GoalAuthority, intention IntentionAuthority, due IntentionDueFact, capabilityName string, influences []DecisionInfluence, gate AgencyExecutionGate) (FrozenIntentionAction, error) {
	if err := goal.Validate(); err != nil {
		return FrozenIntentionAction{}, err
	}
	if err := intention.Validate(); err != nil {
		return FrozenIntentionAction{}, err
	}
	if goal.Ref != intention.GoalRef || due.IntentionRef != intention.Ref || due.GoalRef != goal.Ref || due.IntentionRevision != intention.Revision || due.AttemptID == "" {
		return FrozenIntentionAction{}, errors.New("intention_due_identity_invalid")
	}
	if intention.Status != IntentionQualified && intention.Status != IntentionDue {
		return FrozenIntentionAction{}, errors.New("intention_not_executable")
	}
	now := gate.Now.UTC()
	if now.IsZero() || !now.Before(intention.Expiration) {
		return FrozenIntentionAction{}, errors.New("intention_expired")
	}
	if !gate.PermissionAllowed {
		return FrozenIntentionAction{}, errors.New("intention_permission_denied")
	}
	if !gate.BudgetAvailable {
		return FrozenIntentionAction{}, errors.New("intention_budget_exhausted")
	}
	if gate.QuietHoursBlocked {
		return FrozenIntentionAction{}, errors.New("intention_quiet_hours")
	}
	if gate.CooldownUntil != nil && now.Before(gate.CooldownUntil.UTC()) {
		return FrozenIntentionAction{}, errors.New("intention_cooldown")
	}
	if gate.FoundationRevision < 0 || gate.CurrentStateRevision < 0 || strings.TrimSpace(gate.LifeContextRevision) == "" {
		return FrozenIntentionAction{}, errors.New("intention_authority_revision_invalid")
	}
	capabilityName = strings.TrimSpace(capabilityName)
	if capabilityName == "" {
		return FrozenIntentionAction{}, errors.New("intention_capability_required")
	}
	if len(intention.CapabilityConstraints) > 0 && !containsString(intention.CapabilityConstraints, capabilityName) {
		return FrozenIntentionAction{}, errors.New("intention_capability_not_allowed")
	}
	if !influencesContainRefs(influences, goal.Ref, intention.Ref) {
		return FrozenIntentionAction{}, errors.New("intention_service_influences_required")
	}
	actionID := "intention_action_" + stableDigest(due.AttemptID+"\x1f"+capabilityName)
	return FrozenIntentionAction{
		IntentionRef: intention.Ref, GoalRef: goal.Ref, AttemptID: due.AttemptID, ActionID: actionID,
		OutcomeID: "outcome_" + stableDigest(actionID+"\x1f"+actionPrimaryCallID), CapabilityName: capabilityName,
		ExpectedOutcome: intention.ExpectedOutcome, SuccessCriteria: append([]string(nil), goal.SuccessCriteria...),
		ExpectedRevisions: map[string]any{"goal_revision": goal.Revision, "intention_revision": intention.Revision, "foundation_revision": gate.FoundationRevision, "current_state_revision": gate.CurrentStateRevision, "life_context_revision": gate.LifeContextRevision},
		Influences:        append([]DecisionInfluence(nil), influences...),
	}, nil
}

type IntentionAttemptStatus string

const (
	IntentionAttemptSucceeded  IntentionAttemptStatus = "succeeded"
	IntentionAttemptFailed     IntentionAttemptStatus = "failed"
	IntentionAttemptCancelled  IntentionAttemptStatus = "cancelled"
	IntentionAttemptSuppressed IntentionAttemptStatus = "suppressed"
)

type IntentionAttempt struct {
	AttemptID     string                 `json:"attempt_id"`
	IntentionRef  string                 `json:"intention_ref"`
	GoalRef       string                 `json:"goal_ref"`
	ActionID      string                 `json:"action_id"`
	OutcomeID     string                 `json:"outcome_id"`
	OutcomeDigest string                 `json:"outcome_digest"`
	Status        IntentionAttemptStatus `json:"status"`
	OccurredAt    time.Time              `json:"occurred_at"`
}

type IntentionAttemptSettlement struct {
	Intention IntentionAuthority `json:"intention"`
	Attempt   IntentionAttempt   `json:"attempt"`
	Replayed  bool               `json:"replayed"`
}

func SettleIntentionAttempt(intention IntentionAuthority, frozen FrozenIntentionAction, outcome ActionOutcome, existing *IntentionAttempt) (IntentionAttemptSettlement, error) {
	if err := intention.Validate(); err != nil {
		return IntentionAttemptSettlement{}, err
	}
	if frozen.IntentionRef != intention.Ref || frozen.GoalRef != intention.GoalRef || frozen.AttemptID == "" || outcome.ActionID != frozen.ActionID || !containsString(outcome.IntentionRefs, intention.Ref) || !containsString(outcome.GoalRefs, intention.GoalRef) {
		return IntentionAttemptSettlement{}, errors.New("intention_outcome_identity_invalid")
	}
	digest := stableDigest(jsonString(map[string]any{"id": outcome.ID, "action_id": outcome.ActionID, "call_id": outcome.CallID, "status": outcome.Status, "success_boundary": outcome.SuccessBoundary, "error_code": outcome.ErrorCode, "revision": outcome.Revision}))
	if existing != nil {
		if existing.AttemptID == frozen.AttemptID && existing.OutcomeDigest == digest {
			return IntentionAttemptSettlement{Intention: intention, Attempt: *existing, Replayed: true}, nil
		}
		return IntentionAttemptSettlement{}, errors.New("intention_attempt_replay_conflict")
	}
	next := intention
	next.Revision++
	next.LastAttemptID = frozen.AttemptID
	attemptStatus := IntentionAttemptFailed
	switch outcome.Status {
	case ActionOutcomeCompleted:
		attemptStatus = IntentionAttemptSucceeded
		next.Status = IntentionCompleted
	case ActionOutcomeFailed:
		attemptStatus = IntentionAttemptFailed
		next.Status = IntentionQualified
	case ActionOutcomeCancelled:
		attemptStatus = IntentionAttemptCancelled
		next.Status = IntentionCancelled
	case ActionOutcomeSuppressed:
		attemptStatus = IntentionAttemptSuppressed
		next.Status = IntentionQualified
	default:
		return IntentionAttemptSettlement{}, errors.New("intention_outcome_not_terminal")
	}
	next.LastAttemptStatus = string(attemptStatus)
	next.EvidenceRefs = mergeStableRefs(next.EvidenceRefs, outcome.EvidenceRefs)
	if err := next.Validate(); err != nil {
		return IntentionAttemptSettlement{}, err
	}
	attempt := IntentionAttempt{
		AttemptID: frozen.AttemptID, IntentionRef: intention.Ref, GoalRef: intention.GoalRef,
		ActionID: outcome.ActionID, OutcomeID: outcome.ID, OutcomeDigest: digest, Status: attemptStatus, OccurredAt: outcome.OccurredAt.UTC(),
	}
	return IntentionAttemptSettlement{Intention: next, Attempt: attempt}, nil
}

func validAgencyReference(ref string, kind ContextReferenceKind) bool {
	return strings.HasPrefix(ref, string(kind)+":") && contextReferencePattern.MatchString(ref)
}

func unitFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func validateBoundedRefs(refs []string) error {
	for _, ref := range refs {
		if strings.TrimSpace(ref) == "" || len([]rune(ref)) > 256 {
			return errors.New("evidence_ref_invalid")
		}
	}
	return nil
}

func mergeStableRefs(groups ...[]string) []string {
	seen := map[string]struct{}{}
	for _, group := range groups {
		for _, raw := range group {
			if ref := strings.TrimSpace(raw); ref != "" {
				seen[ref] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(seen))
	for ref := range seen {
		result = append(result, ref)
	}
	sort.Strings(result)
	return result
}

func goalStatusTerminal(status GoalLifecycleStatus) bool {
	return status == GoalCompleted || status == GoalAbandoned || status == GoalCancelled
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func influencesContainRefs(influences []DecisionInfluence, refs ...string) bool {
	seen := map[string]struct{}{}
	for _, influence := range influences {
		seen[influence.Ref] = struct{}{}
	}
	for _, ref := range refs {
		if _, ok := seen[ref]; !ok {
			return false
		}
	}
	return true
}
