package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

const (
	contextReferenceIndexVersion     = "fluctlight.context-reference-index.v1"
	maxContextReferences             = 128
	maxContextReferenceRunes         = 96
	maxContextReferenceEntityRunes   = 256
	maxContextReferenceSnapshotBytes = 32 * 1024
	maxContextReferenceIndexBytes    = maxToolArgumentsBytes * 2
	maxDecisionInfluences            = 24
	maxDecisionInfluenceNoteRunes    = 500
	maxDriveSemanticSignals          = 16
	maxDriveSignalEvidenceRefs       = 12
)

type ContextReferenceKind string

const (
	ContextReferenceMemory           ContextReferenceKind = "memory"
	ContextReferenceRelationship     ContextReferenceKind = "relationship"
	ContextReferenceGoal             ContextReferenceKind = "goal"
	ContextReferenceIntention        ContextReferenceKind = "intention"
	ContextReferenceScene            ContextReferenceKind = "scene"
	ContextReferenceLifeContext      ContextReferenceKind = "life_context"
	ContextReferenceSchedule         ContextReferenceKind = "schedule"
	ContextReferenceScheduleItem     ContextReferenceKind = "schedule_item"
	ContextReferencePresence         ContextReferenceKind = "presence"
	ContextReferenceState            ContextReferenceKind = "state"
	ContextReferenceAffectProfile    ContextReferenceKind = "affect_profile"
	ContextReferenceDevelopingSelf   ContextReferenceKind = "developing_self"
	ContextReferenceDrive            ContextReferenceKind = "drive"
	ContextReferencePreference       ContextReferenceKind = "preference"
	ContextReferenceTrigger          ContextReferenceKind = "trigger"
	ContextReferenceOutcome          ContextReferenceKind = "outcome"
	ContextReferenceEvolutionOverlay ContextReferenceKind = "evolution_overlay"
)

var (
	validContextReferenceKinds = map[ContextReferenceKind]struct{}{
		ContextReferenceMemory: {}, ContextReferenceRelationship: {}, ContextReferenceGoal: {},
		ContextReferenceIntention: {}, ContextReferenceScene: {}, ContextReferenceLifeContext: {}, ContextReferenceSchedule: {},
		ContextReferenceScheduleItem: {}, ContextReferencePresence: {}, ContextReferenceState: {},
		ContextReferenceAffectProfile: {}, ContextReferenceDevelopingSelf: {}, ContextReferenceDrive: {},
		ContextReferencePreference: {}, ContextReferenceTrigger: {}, ContextReferenceOutcome: {},
		ContextReferenceEvolutionOverlay: {},
	}
	contextReferencePattern     = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}:ctx_[a-f0-9]{32}$`)
	validDecisionInfluenceRoles = map[string]struct{}{
		"grounds": {}, "motivates": {}, "constrains": {}, "conflicts": {}, "satisfies": {},
	}
)

// ContextReference is an internal, frozen mapping from one Provider-safe token
// to the authoritative entity revision that was visible for a decision. Raw
// entity IDs and scope never cross the Provider boundary.
type ContextReference struct {
	Ref      string               `json:"ref"`
	Kind     ContextReferenceKind `json:"kind"`
	EntityID string               `json:"entity_id"`
	Revision int                  `json:"revision"`
	Scope    string               `json:"scope"`
	Snapshot json.RawMessage      `json:"snapshot"`
}

// ContextReferenceIndex belongs to the Core-owned ContextProjection. It is
// never accepted from Provider output. Its identity fields bind every entry to
// the exact actor/conversation/profile surface that saw the opaque ref.
type ContextReferenceIndex struct {
	SchemaVersion   string                      `json:"schema_version"`
	FluctlightID    string                      `json:"fluctlight_id"`
	OwnerActorID    string                      `json:"owner_actor_id"`
	SpeakerActorID  string                      `json:"speaker_actor_id,omitempty"`
	ConversationID  string                      `json:"conversation_id,omitempty"`
	ActiveProfileID string                      `json:"active_profile_id,omitempty"`
	ByRef           map[string]ContextReference `json:"by_ref"`
}

type DecisionInfluence struct {
	Ref        string  `json:"ref"`
	Role       string  `json:"role"`
	Confidence float64 `json:"confidence"`
	Note       string  `json:"note"`
}

func contextReferenceScope(fluctlightID, actorID, conversationID, activeProfileID string) string {
	return strings.Join([]string{
		strings.TrimSpace(fluctlightID),
		strings.TrimSpace(actorID),
		strings.TrimSpace(conversationID),
		strings.TrimSpace(activeProfileID),
	}, "\x1f")
}

func contextReferenceToken(kind ContextReferenceKind, entityID string, revision int, scope string, snapshot json.RawMessage) string {
	return string(kind) + ":ctx_" + stableDigest(strings.Join([]string{
		string(kind), strings.TrimSpace(entityID), fmt.Sprint(revision), scope, stableDigest(string(snapshot)),
	}, "\x1f"))
}

func (index ContextReferenceIndex) expectedScope() string {
	actorID := index.SpeakerActorID
	if actorID == "" {
		actorID = index.OwnerActorID
	}
	return contextReferenceScope(index.FluctlightID, actorID, index.ConversationID, index.ActiveProfileID)
}

func (index ContextReferenceIndex) Validate() error {
	if index.SchemaVersion != contextReferenceIndexVersion {
		return errors.New("context_reference_index_schema_invalid")
	}
	if strings.TrimSpace(index.FluctlightID) == "" || strings.TrimSpace(index.OwnerActorID) == "" {
		return errors.New("context_reference_index_identity_invalid")
	}
	if index.ByRef == nil {
		return errors.New("context_reference_index_entries_invalid")
	}
	if len(index.ByRef) > maxContextReferences {
		return errors.New("context_reference_index_too_large")
	}
	expectedScope := index.expectedScope()
	for ref, entry := range index.ByRef {
		if ref != entry.Ref || len([]rune(ref)) > maxContextReferenceRunes || !contextReferencePattern.MatchString(ref) {
			return errors.New("context_reference_invalid")
		}
		if _, ok := validContextReferenceKinds[entry.Kind]; !ok {
			return errors.New("context_reference_kind_invalid")
		}
		if entityID := strings.TrimSpace(entry.EntityID); entityID == "" || len([]rune(entityID)) > maxContextReferenceEntityRunes {
			return errors.New("context_reference_entity_invalid")
		}
		if entry.Revision < 0 || entry.Scope != expectedScope {
			return errors.New("context_reference_scope_invalid")
		}
		if len(entry.Snapshot) == 0 || len(entry.Snapshot) > maxContextReferenceSnapshotBytes || !json.Valid(entry.Snapshot) {
			return errors.New("context_reference_snapshot_invalid")
		}
		if expected := contextReferenceToken(entry.Kind, entry.EntityID, entry.Revision, entry.Scope, entry.Snapshot); expected != ref {
			return errors.New("context_reference_token_invalid")
		}
	}
	encoded, err := json.Marshal(index)
	if err != nil || len(encoded) > maxContextReferenceIndexBytes {
		return errors.New("context_reference_index_too_large")
	}
	return nil
}

func newContextReferenceIndex(projection ContextProjection) ContextReferenceIndex {
	return ContextReferenceIndex{
		SchemaVersion:   contextReferenceIndexVersion,
		FluctlightID:    strings.TrimSpace(projection.FluctlightID),
		OwnerActorID:    strings.TrimSpace(projection.OwnerActorID),
		SpeakerActorID:  strings.TrimSpace(stringValue(projection.CurrentSpeaker["actor_id"])),
		ConversationID:  strings.TrimSpace(projection.ConversationID),
		ActiveProfileID: strings.TrimSpace(stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"])),
		ByRef:           make(map[string]ContextReference),
	}
}

func (index *ContextReferenceIndex) add(kind ContextReferenceKind, entityID string, revision int, snapshot any) (string, error) {
	if index == nil {
		return "", errors.New("context_reference_index_missing")
	}
	if len(index.ByRef) >= maxContextReferences {
		return "", errors.New("context_reference_index_too_large")
	}
	entityID = strings.TrimSpace(entityID)
	if _, ok := validContextReferenceKinds[kind]; !ok || entityID == "" || len([]rune(entityID)) > maxContextReferenceEntityRunes || revision < 0 {
		return "", errors.New("context_reference_target_invalid")
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || len(encoded) == 0 || len(encoded) > maxContextReferenceSnapshotBytes {
		return "", errors.New("context_reference_snapshot_invalid")
	}
	scope := index.expectedScope()
	ref := contextReferenceToken(kind, entityID, revision, scope, encoded)
	entry := ContextReference{Ref: ref, Kind: kind, EntityID: entityID, Revision: revision, Scope: scope, Snapshot: encoded}
	if existing, ok := index.ByRef[ref]; ok {
		if existing.EntityID != entry.EntityID || existing.Kind != entry.Kind || existing.Revision != entry.Revision || existing.Scope != entry.Scope || string(existing.Snapshot) != string(entry.Snapshot) {
			return "", errors.New("context_reference_collision")
		}
		return ref, nil
	}
	index.ByRef[ref] = entry
	return ref, nil
}

func addReferenceToRow(index *ContextReferenceIndex, kind ContextReferenceKind, row map[string]any, entityID string, revision int) (string, error) {
	snapshot := cloneMap(row)
	delete(snapshot, "ref")
	ref, err := index.add(kind, entityID, revision, snapshot)
	if err != nil {
		return "", err
	}
	row["ref"] = ref
	return ref, nil
}

func buildContextReferenceIndex(projection *ContextProjection) error {
	if projection == nil || strings.TrimSpace(projection.FluctlightID) == "" || strings.TrimSpace(projection.OwnerActorID) == "" {
		return errors.New("context_reference_projection_invalid")
	}
	index := newContextReferenceIndex(*projection)
	activeProfileID := index.ActiveProfileID

	for _, memory := range projection.Memories {
		if _, err := addReferenceToRow(&index, ContextReferenceMemory, memory, stringValue(memory["id"]), intValue(memory["revision"])); err != nil {
			return err
		}
	}
	for _, relationship := range selectActiveProfileRelationships(projection.Relationships, activeProfileID) {
		if _, err := addReferenceToRow(&index, ContextReferenceRelationship, relationship, stringValue(relationship["id"]), intValue(relationship["revision"])); err != nil {
			return err
		}
	}
	goalRefs := make(map[string]string)
	for _, goal := range filterActiveProfileRows(projection.Goals, activeProfileID) {
		ref, err := addReferenceToRow(&index, ContextReferenceGoal, goal, stringValue(goal["id"]), intValue(goal["revision"]))
		if err != nil {
			return err
		}
		goalRefs[stringValue(goal["id"])] = ref
	}
	for _, intention := range filterActiveProfileRows(projection.Intentions, activeProfileID) {
		if goalRef := goalRefs[stringValue(intention["goal_id"])]; goalRef != "" {
			intention["goal_ref"] = goalRef
		}
		if _, err := addReferenceToRow(&index, ContextReferenceIntention, intention, stringValue(intention["id"]), intValue(intention["revision"])); err != nil {
			return err
		}
	}
	for _, outcome := range projection.RecentOutcomes {
		if _, err := addReferenceToRow(&index, ContextReferenceOutcome, outcome, stringValue(outcome["id"]), intValue(outcome["revision"])); err != nil {
			return err
		}
	}
	for _, claim := range projection.DevelopingSelf {
		if _, err := addReferenceToRow(&index, ContextReferenceDevelopingSelf, claim, stringValue(claim["id"]), intValue(claim["revision"])); err != nil {
			return err
		}
	}
	for _, overlay := range projection.EvolutionOverlays {
		if _, err := addReferenceToRow(&index, ContextReferenceEvolutionOverlay, overlay, stringValue(overlay["id"]), intValue(overlay["revision"])); err != nil {
			return err
		}
	}
	driveRefs := make(map[string]string)
	for _, row := range projection.DriveSlots {
		ref, err := addReferenceToRow(&index, ContextReferenceDrive, row, stringValue(row["id"]), intValue(row["revision"]))
		if err != nil {
			return err
		}
		driveRefs[stringValue(row["key"])] = ref
	}
	for _, group := range []struct {
		kind ContextReferenceKind
		rows []map[string]any
	}{
		{ContextReferencePreference, projection.PreferenceSlots},
		{ContextReferenceTrigger, projection.TriggerPreferences},
	} {
		for _, row := range group.rows {
			if _, err := addReferenceToRow(&index, group.kind, row, stringValue(row["id"]), intValue(row["revision"])); err != nil {
				return err
			}
		}
	}
	for _, raw := range arrayValue(projection.InnerState["drives"]) {
		drive := mapValue(raw)
		key := strings.TrimSpace(stringValue(drive["key"]))
		if key == "" {
			continue
		}
		ref := driveRefs[key]
		if ref == "" {
			var err error
			ref, err = index.add(ContextReferenceDrive, projection.FluctlightID+":built-in-drive:"+key, projection.CurrentStateRevision, drive)
			if err != nil {
				return err
			}
		}
		drive["ref"] = ref
	}

	if len(projection.InnerState) > 0 {
		ref, err := addReferenceToRow(&index, ContextReferenceState, projection.InnerState, projection.FluctlightID, projection.CurrentStateRevision)
		if err != nil {
			return err
		}
		if data := mapValue(projection.CurrentState["data"]); len(data) > 0 {
			if inner := mapValue(data["inner_state"]); len(inner) > 0 {
				inner["ref"] = ref
			}
		}
	}
	if len(projection.AffectProfile) > 0 {
		ref, err := addReferenceToRow(&index, ContextReferenceAffectProfile, projection.AffectProfile, projection.FluctlightID, intValue(projection.AffectProfile["revision"]))
		if err != nil {
			return err
		}
		if data := mapValue(projection.CurrentState["data"]); len(data) > 0 {
			if profile := mapValue(data["affect_profile"]); len(profile) > 0 {
				profile["ref"] = ref
			}
		}
	}

	scheduleRef := ""
	scheduleItemRefs := make(map[string]string)
	if len(projection.Schedule) > 0 {
		scheduleSnapshot := cloneMap(projection.Schedule)
		delete(scheduleSnapshot, "items")
		ref, err := index.add(ContextReferenceSchedule, stringValue(projection.Schedule["id"]), intValue(projection.Schedule["revision"]), scheduleSnapshot)
		if err != nil {
			return err
		}
		scheduleRef = ref
		projection.Schedule["ref"] = ref
		for _, raw := range arrayValue(projection.Schedule["items"]) {
			item := mapValue(raw)
			if len(item) == 0 {
				continue
			}
			itemRef, err := addReferenceToRow(&index, ContextReferenceScheduleItem, item, stringValue(item["id"]), intValue(projection.Schedule["revision"]))
			if err != nil {
				return err
			}
			scheduleItemRefs[stringValue(item["id"])] = itemRef
		}
	}

	if len(projection.LifeContext) > 0 {
		switch stringValue(projection.LifeContext["source"]) {
		case "event":
			eventID := stringValue(projection.LifeContext["event_id"])
			if eventID != "" {
				ref, err := index.add(ContextReferenceScene, eventID, intValue(projection.LifeContext["event_revision"]), projection.LifeContext)
				if err != nil {
					return err
				}
				projection.LifeContext["event_ref"] = ref
			}
		}
		if scheduleRef != "" && stringValue(projection.LifeContext["schedule_id"]) != "" {
			projection.LifeContext["schedule_ref"] = scheduleRef
		}
		if itemRef := scheduleItemRefs[stringValue(projection.LifeContext["schedule_item_id"])]; itemRef != "" {
			projection.LifeContext["schedule_item_ref"] = itemRef
		}
		if presence := mapValue(projection.LifeContext["presence"]); len(presence) > 0 {
			if presenceID := stringValue(presence["id"]); presenceID != "" {
				ref, err := addReferenceToRow(&index, ContextReferencePresence, presence, presenceID, intValue(presence["revision"]))
				if err != nil {
					return err
				}
				projection.Presence = presence
				projection.LifeContext["presence"] = presence
				projection.LifeContext["presence_ref"] = ref
			}
		}
		lifeRef, err := index.add(ContextReferenceLifeContext, projection.FluctlightID, intValue(projection.LifeContext["revision"]), projection.LifeContext)
		if err != nil {
			return err
		}
		projection.LifeContext["ref"] = lifeRef
		if data := mapValue(projection.CurrentState["data"]); len(data) > 0 {
			if life := mapValue(data["life_context"]); len(life) > 0 {
				life["ref"] = lifeRef
			}
		}
	}

	if err := index.Validate(); err != nil {
		return err
	}
	projection.ReferenceIndex = index
	projection.SchemaVersion = "fluctlight.context.v3"
	return nil
}

func validateDecisionInfluences(value any, index ContextReferenceIndex) ([]DecisionInfluence, error) {
	if err := index.Validate(); err != nil {
		return nil, err
	}
	if value == nil {
		return []DecisionInfluence{}, nil
	}
	var raw []any
	switch typed := value.(type) {
	case []any:
		raw = typed
	case []map[string]any:
		raw = make([]any, len(typed))
		for position, item := range typed {
			raw[position] = item
		}
	default:
		return nil, errors.New("decision_influences_invalid")
	}
	if len(raw) > maxDecisionInfluences {
		return nil, errors.New("decision_influences_too_large")
	}
	result := make([]DecisionInfluence, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for position, item := range raw {
		candidate := mapValue(item)
		if len(candidate) != 4 {
			return nil, fmt.Errorf("decision_influence_%d_shape_invalid", position)
		}
		for key := range candidate {
			switch key {
			case "ref", "role", "confidence", "note":
			default:
				return nil, fmt.Errorf("decision_influence_%d_shape_invalid", position)
			}
		}
		ref := strings.TrimSpace(stringValue(candidate["ref"]))
		if ref == "" || len([]rune(ref)) > maxContextReferenceRunes || !contextReferencePattern.MatchString(ref) {
			return nil, fmt.Errorf("decision_influence_%d_ref_invalid", position)
		}
		if _, ok := index.ByRef[ref]; !ok {
			return nil, fmt.Errorf("decision_influence_%d_ref_unknown", position)
		}
		role := strings.TrimSpace(stringValue(candidate["role"]))
		if _, ok := validDecisionInfluenceRoles[role]; !ok {
			return nil, fmt.Errorf("decision_influence_%d_role_invalid", position)
		}
		confidence, ok := numberFloat(candidate["confidence"])
		if !ok || math.IsNaN(confidence) || math.IsInf(confidence, 0) || confidence < 0 || confidence > 1 {
			return nil, fmt.Errorf("decision_influence_%d_confidence_invalid", position)
		}
		note := strings.TrimSpace(stringValue(candidate["note"]))
		if note == "" || len([]rune(note)) > maxDecisionInfluenceNoteRunes {
			return nil, fmt.Errorf("decision_influence_%d_note_invalid", position)
		}
		identity := ref + "\x1f" + role
		if _, duplicate := seen[identity]; duplicate {
			return nil, fmt.Errorf("decision_influence_%d_duplicate", position)
		}
		seen[identity] = struct{}{}
		result = append(result, DecisionInfluence{Ref: ref, Role: role, Confidence: confidence, Note: note})
	}
	return result, nil
}

func contextReferenceIndexFromValue(value any) (ContextReferenceIndex, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return ContextReferenceIndex{}, errors.New("context_reference_index_invalid")
	}
	var index ContextReferenceIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return ContextReferenceIndex{}, errors.New("context_reference_index_invalid")
	}
	if err := index.Validate(); err != nil {
		return ContextReferenceIndex{}, err
	}
	return index, nil
}

// freezeDecisionInfluences validates only the Provider-owned influence list,
// then installs the Core-owned reference index. Any attempt by the Provider to
// supply that internal mapping is rejected instead of silently merged.
func freezeDecisionInfluences(decision map[string]any, projection ContextProjection, required bool) ([]DecisionInfluence, error) {
	if decision == nil {
		return nil, errors.New("decision_missing")
	}
	if _, supplied := decision["context_reference_index"]; supplied {
		return nil, errors.New("provider_context_reference_index_forbidden")
	}
	if _, supplied := decision["context_reference_version"]; supplied {
		return nil, errors.New("provider_context_reference_version_forbidden")
	}
	if _, supplied := decision["goal_refs"]; supplied {
		return nil, errors.New("provider_goal_refs_forbidden")
	}
	if _, supplied := decision["intention_refs"]; supplied {
		return nil, errors.New("provider_intention_refs_forbidden")
	}
	if _, supplied := decision["personality_transition"]; supplied {
		return nil, errors.New("provider_personality_transition_forbidden")
	}
	if _, supplied := decision["cognitive_state_transition"]; supplied {
		return nil, errors.New("provider_cognitive_state_transition_forbidden")
	}
	if projection.SchemaVersion != "fluctlight.context.v3" {
		return nil, errors.New("decision_context_projection_version_invalid")
	}
	if projection.ReferenceIndex.FluctlightID != projection.FluctlightID || projection.ReferenceIndex.OwnerActorID != projection.OwnerActorID || projection.ReferenceIndex.SpeakerActorID != strings.TrimSpace(stringValue(projection.CurrentSpeaker["actor_id"])) || projection.ReferenceIndex.ConversationID != projection.ConversationID || projection.ReferenceIndex.ActiveProfileID != strings.TrimSpace(stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"])) {
		return nil, errors.New("decision_context_reference_scope_invalid")
	}
	values, err := validateDecisionInfluences(decision["influences"], projection.ReferenceIndex)
	if err != nil {
		return nil, err
	}
	if required {
		if err := requireDecisionInfluences(values, "decision_influences_required"); err != nil {
			return nil, err
		}
	}
	if err := normalizeDecisionAppraisalEvidence(decision, projection.ReferenceIndex, projection.SourceFactID); err != nil {
		return nil, err
	}
	if _, err := validateDecisionDriveSignals(decision, projection.ReferenceIndex, values); err != nil {
		return nil, err
	}
	decision["influences"] = decisionInfluenceMaps(values)
	decision["context_reference_version"] = contextReferenceIndexVersion
	decision["context_reference_index"] = projection.ReferenceIndex
	goalRefs, intentionRefs := decisionServiceRefs(values, projection.ReferenceIndex)
	decision["goal_refs"] = goalRefs
	decision["intention_refs"] = intentionRefs
	return values, nil
}

func validateFrozenDecisionInfluences(decision map[string]any) error {
	if transition := strings.TrimSpace(stringValue(decision["cognitive_state_transition"])); transition != "" && transition != "not_proposed" {
		return errors.New("frozen_cognitive_state_transition_invalid")
	}
	version := strings.TrimSpace(stringValue(decision["context_reference_version"]))
	if version == "" {
		return errors.New("frozen_context_reference_version_invalid")
	}
	if version != contextReferenceIndexVersion {
		return errors.New("frozen_context_reference_version_invalid")
	}
	index, err := contextReferenceIndexFromValue(decision["context_reference_index"])
	if err != nil {
		return err
	}
	influences, err := validateDecisionInfluences(decision["influences"], index)
	if err != nil {
		return err
	}
	sourceFactID := ""
	if projection, ok := contextProjectionFromValue(decision["context_projection"]); ok {
		sourceFactID = projection.SourceFactID
	}
	if err := normalizeDecisionAppraisalEvidence(decision, index, sourceFactID); err != nil {
		return err
	}
	if _, err := validateDecisionDriveSignals(decision, index, influences); err != nil {
		return err
	}
	goalRefs, intentionRefs := decisionServiceRefs(influences, index)
	if !sameStringSet(decision["goal_refs"], goalRefs) || !sameStringSet(decision["intention_refs"], intentionRefs) {
		return errors.New("frozen_decision_service_refs_invalid")
	}
	return nil
}

func normalizeDecisionAppraisalEvidence(decision map[string]any, index ContextReferenceIndex, sourceFactID string) error {
	stages := cognitiveStagePayload(decision)
	if len(mapValue(stages["appraisal"])) == 0 {
		return nil
	}
	appraisal, err := normalizeAppraisal(stages["appraisal"])
	if err != nil {
		return err
	}
	refs := arrayValue(appraisal["evidence_refs"])
	seen := make(map[string]struct{}, len(refs))
	for position, raw := range refs {
		ref := strings.TrimSpace(stringValue(raw))
		if ref == "" {
			return fmt.Errorf("appraisal_evidence_ref_%d_invalid", position)
		}
		if ref != strings.TrimSpace(sourceFactID) {
			if _, exists := index.ByRef[ref]; !exists {
				return fmt.Errorf("appraisal_evidence_ref_%d_foreign", position)
			}
		}
		if _, duplicate := seen[ref]; duplicate {
			return fmt.Errorf("appraisal_evidence_ref_%d_duplicate", position)
		}
		seen[ref] = struct{}{}
	}
	decision["appraisal"] = appraisal
	return nil
}

func validateDecisionDriveSignals(decision map[string]any, index ContextReferenceIndex, influences []DecisionInfluence) ([]driveSemanticSignal, error) {
	stages := cognitiveStagePayload(decision)
	appraisal := mapValue(stages["appraisal"])
	if len(appraisal) == 0 {
		return []driveSemanticSignal{}, nil
	}
	rawValue, supplied := appraisal["drive_signals"]
	if !supplied || rawValue == nil {
		appraisal["drive_signals"] = []any{}
		decision["appraisal"] = appraisal
		return []driveSemanticSignal{}, nil
	}
	raw, ok := rawValue.([]any)
	if !ok {
		return nil, errors.New("drive_signals_invalid")
	}
	if len(raw) > maxDriveSemanticSignals {
		return nil, errors.New("drive_signals_too_large")
	}
	influenceRefs := make(map[string]struct{}, len(influences))
	for _, influence := range influences {
		influenceRefs[influence.Ref] = struct{}{}
	}
	seen := make(map[string]struct{}, len(raw))
	normalized := make([]any, 0, len(raw))
	result := make([]driveSemanticSignal, 0, len(raw))
	for position, item := range raw {
		candidate := mapValue(item)
		if len(candidate) != 5 {
			return nil, fmt.Errorf("drive_signal_%d_shape_invalid", position)
		}
		for key := range candidate {
			switch key {
			case "ref", "direction", "strength", "confidence", "evidence_refs":
			default:
				return nil, fmt.Errorf("drive_signal_%d_shape_invalid", position)
			}
		}
		ref := strings.TrimSpace(stringValue(candidate["ref"]))
		entry, exists := index.ByRef[ref]
		if !exists || entry.Kind != ContextReferenceDrive {
			return nil, fmt.Errorf("drive_signal_%d_ref_invalid", position)
		}
		if _, influenced := influenceRefs[ref]; !influenced {
			return nil, fmt.Errorf("drive_signal_%d_influence_required", position)
		}
		direction := strings.TrimSpace(stringValue(candidate["direction"]))
		if direction != "increase" && direction != "decrease" {
			return nil, fmt.Errorf("drive_signal_%d_direction_invalid", position)
		}
		strength, strengthOK := numberFloat(candidate["strength"])
		confidence, confidenceOK := numberFloat(candidate["confidence"])
		if !strengthOK || strength < 0 || strength > 1 || !confidenceOK || confidence < 0 || confidence > 1 {
			return nil, fmt.Errorf("drive_signal_%d_strength_invalid", position)
		}
		evidenceValues, evidenceOK := candidate["evidence_refs"].([]any)
		if !evidenceOK || len(evidenceValues) == 0 || len(evidenceValues) > maxDriveSignalEvidenceRefs {
			return nil, fmt.Errorf("drive_signal_%d_evidence_invalid", position)
		}
		evidenceRefs := make([]string, 0, len(evidenceValues))
		evidenceSeen := make(map[string]struct{}, len(evidenceValues))
		for _, rawEvidence := range evidenceValues {
			evidenceRef := strings.TrimSpace(stringValue(rawEvidence))
			if _, exists := index.ByRef[evidenceRef]; !exists {
				return nil, fmt.Errorf("drive_signal_%d_evidence_invalid", position)
			}
			if _, duplicate := evidenceSeen[evidenceRef]; duplicate {
				return nil, fmt.Errorf("drive_signal_%d_evidence_duplicate", position)
			}
			evidenceSeen[evidenceRef] = struct{}{}
			evidenceRefs = append(evidenceRefs, evidenceRef)
		}
		identity := ref + "\x1f" + direction
		if _, duplicate := seen[identity]; duplicate {
			return nil, fmt.Errorf("drive_signal_%d_duplicate", position)
		}
		seen[identity] = struct{}{}
		snapshot := decodeObject(entry.Snapshot)
		key := strings.TrimSpace(stringValue(snapshot["key"]))
		if key == "" {
			return nil, fmt.Errorf("drive_signal_%d_target_invalid", position)
		}
		normalized = append(normalized, map[string]any{"ref": ref, "direction": direction, "strength": strength, "confidence": confidence, "evidence_refs": stringSliceAny(evidenceRefs)})
		result = append(result, driveSemanticSignal{Ref: ref, Key: key, Direction: direction, Strength: strength, Confidence: confidence, EvidenceRefs: evidenceRefs})
	}
	appraisal["drive_signals"] = normalized
	decision["appraisal"] = appraisal
	return result, nil
}

func frozenDecisionDriveSignals(decision map[string]any) ([]driveSemanticSignal, error) {
	version := strings.TrimSpace(stringValue(decision["context_reference_version"]))
	if version == "" {
		return nil, errors.New("frozen_context_reference_version_invalid")
	}
	index, err := contextReferenceIndexFromValue(decision["context_reference_index"])
	if err != nil {
		return nil, err
	}
	influences, err := validateDecisionInfluences(decision["influences"], index)
	if err != nil {
		return nil, err
	}
	return validateDecisionDriveSignals(decision, index, influences)
}

func frozenDecisionCausality(decision map[string]any) (map[string]any, error) {
	version := strings.TrimSpace(stringValue(decision["context_reference_version"]))
	if version == "" {
		return nil, errors.New("frozen_context_reference_version_invalid")
	}
	if err := validateFrozenDecisionInfluences(decision); err != nil {
		return nil, err
	}
	index, err := contextReferenceIndexFromValue(decision["context_reference_index"])
	if err != nil {
		return nil, err
	}
	influences, err := validateDecisionInfluences(decision["influences"], index)
	if err != nil {
		return nil, err
	}
	references := make(map[string]ContextReference, len(influences))
	for _, influence := range influences {
		references[influence.Ref] = index.ByRef[influence.Ref]
	}
	return map[string]any{
		"context_reference_version": version,
		"influences":                decisionInfluenceMaps(influences),
		"context_references":        references,
		"goal_refs":                 decisionServiceRefValues(decision["goal_refs"]),
		"intention_refs":            decisionServiceRefValues(decision["intention_refs"]),
	}, nil
}

func decisionServiceRefs(influences []DecisionInfluence, index ContextReferenceIndex) ([]string, []string) {
	goals := make(map[string]struct{})
	intentions := make(map[string]struct{})
	for _, influence := range influences {
		entry, ok := index.ByRef[influence.Ref]
		if !ok {
			continue
		}
		switch entry.Kind {
		case ContextReferenceGoal:
			goals[influence.Ref] = struct{}{}
		case ContextReferenceIntention:
			intentions[influence.Ref] = struct{}{}
		}
	}
	goalRefs := make([]string, 0, len(goals))
	for ref := range goals {
		goalRefs = append(goalRefs, ref)
	}
	intentionRefs := make([]string, 0, len(intentions))
	for ref := range intentions {
		intentionRefs = append(intentionRefs, ref)
	}
	sort.Strings(goalRefs)
	sort.Strings(intentionRefs)
	return goalRefs, intentionRefs
}

func decisionServiceRefValues(value any) []string {
	result := make([]string, 0)
	for _, item := range arrayValue(value) {
		if ref := stringValue(item); ref != "" {
			result = append(result, ref)
		}
	}
	sort.Strings(result)
	return result
}

func sameStringSet(raw any, expected []string) bool {
	actual := decisionServiceRefValues(raw)
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func decisionInfluenceMaps(values []DecisionInfluence) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, map[string]any{
			"ref": value.Ref, "role": value.Role, "confidence": value.Confidence, "note": value.Note,
		})
	}
	return result
}

func requireDecisionInfluences(values []DecisionInfluence, reason string) error {
	if len(values) == 0 {
		return errors.New(reason)
	}
	return nil
}

func decisionInfluenceRefs(values []DecisionInfluence) []string {
	refs := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value.Ref]; ok {
			continue
		}
		seen[value.Ref] = struct{}{}
		refs = append(refs, value.Ref)
	}
	sort.Strings(refs)
	return refs
}
