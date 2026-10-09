package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type goalEvaluationWireBinding struct {
	snapshot          goalEvaluationSnapshot
	goalsByRef        map[string]goalEvaluationGoal
	goalRefsByID      map[string]string
	criteriaByRef     map[string]goalWireCriterionBinding
	criterionRefs     map[string]string
	stagesByRef       map[string]goalWireObjectBinding
	stageRefsByID     map[string]string
	commitmentsByRef  map[string]goalWireObjectBinding
	commitmentRefs    map[string]string
	dependenciesByRef map[string]goalWireObjectBinding
	dependencyRefs    map[string]string
	sourcesByRef      map[string]GoalSource
	sourceRefs        map[string]string
	actorsByRef       map[string]string
	actorRefs         map[string]string
	objectsByRef      map[string]string
	objectRefs        map[string]string
}

type goalWireCriterionBinding struct {
	GoalID      string
	ObjectKind  string
	ObjectID    string
	CriterionID string
}

type goalWireObjectBinding struct {
	GoalID string
	ID     string
}

func newGoalEvaluationWireBinding(snapshot goalEvaluationSnapshot) (*goalEvaluationWireBinding, error) {
	// EntityID is Core-only (json:"-"); persisted claims retain GoalID on
	// their entry. Normalize a private copy so replay has the same wire.
	snapshot.Goals = append([]goalEvaluationGoal(nil), snapshot.Goals...)
	binding := &goalEvaluationWireBinding{
		snapshot: snapshot, goalsByRef: map[string]goalEvaluationGoal{}, goalRefsByID: map[string]string{},
		criteriaByRef: map[string]goalWireCriterionBinding{}, criterionRefs: map[string]string{},
		stagesByRef: map[string]goalWireObjectBinding{}, stageRefsByID: map[string]string{},
		commitmentsByRef: map[string]goalWireObjectBinding{}, commitmentRefs: map[string]string{},
		dependenciesByRef: map[string]goalWireObjectBinding{}, dependencyRefs: map[string]string{},
		sourcesByRef: map[string]GoalSource{}, sourceRefs: map[string]string{},
		actorsByRef: map[string]string{}, actorRefs: map[string]string{},
		objectsByRef: map[string]string{}, objectRefs: map[string]string{},
	}
	binding.bindActor("actor_self", snapshot.FluctlightID)
	binding.bindActor("actor_user", snapshot.OwnerActorID)
	for goalIndex, entry := range snapshot.Goals {
		goalID := entry.Goal.EntityID
		if goalID == "" {
			goalID = entry.GoalID
		}
		if goalID == "" || entry.GoalID != "" && entry.GoalID != goalID {
			return nil, errors.New("goal_evaluation_wire_goal_invalid")
		}
		entry.Goal.EntityID = goalID
		entry.GoalID = goalID
		if entry.Goal.TargetActorID == "" {
			entry.Goal.TargetActorID = entry.TargetActorID
		}
		snapshot.Goals[goalIndex] = entry
		binding.snapshot.Goals[goalIndex] = entry
		if snapshot.FluctlightID != "" && entry.Goal.FluctlightID != snapshot.FluctlightID {
			return nil, errors.New("goal_evaluation_wire_goal_instance_invalid")
		}
		if !goalEvaluationProfileAuthorized(snapshot.ProfileID, entry.Goal.ProfileID) {
			return nil, errors.New("goal_evaluation_wire_goal_profile_invalid")
		}
		goalRef := fmt.Sprintf("goal:%d", goalIndex+1)
		if _, duplicate := binding.goalRefsByID[goalID]; duplicate {
			return nil, errors.New("goal_evaluation_wire_goal_duplicate")
		}
		binding.goalsByRef[goalRef] = entry
		binding.goalRefsByID[goalID] = goalRef
		if entry.Goal.TargetActorID != "" {
			binding.bindActor(fmt.Sprintf("actor_target:%d", goalIndex+1), entry.Goal.TargetActorID)
		}
		if len(entry.Goal.CriterionIDs) != len(entry.Goal.SuccessCriteria) {
			return nil, errors.New("goal_evaluation_wire_criteria_invalid")
		}
		for criterionIndex, criterionID := range entry.Goal.CriterionIDs {
			if criterionID == "" {
				return nil, errors.New("goal_evaluation_wire_criterion_invalid")
			}
			ref := fmt.Sprintf("criterion:%d.%d", goalIndex+1, criterionIndex+1)
			binding.bindCriterion(ref, goalID, "goal", goalID, criterionID)
		}
		for stageIndex, stage := range entry.Stages {
			ref := fmt.Sprintf("stage:%d.%d", goalIndex+1, stageIndex+1)
			if stage.ID == "" || stage.GoalID != "" && stage.GoalID != goalID || snapshot.FluctlightID != "" && stage.FluctlightID != snapshot.FluctlightID || !goalEvaluationProfileAuthorized(snapshot.ProfileID, stage.ProfileID) {
				return nil, errors.New("goal_evaluation_wire_stage_invalid")
			}
			if _, duplicate := binding.stageRefsByID[stage.ID]; duplicate {
				return nil, errors.New("goal_evaluation_wire_stage_duplicate")
			}
			binding.stagesByRef[ref] = goalWireObjectBinding{GoalID: goalID, ID: stage.ID}
			binding.stageRefsByID[stage.ID] = ref
			for criterionIndex, criterion := range stage.Criteria {
				if criterion.ID == "" {
					return nil, errors.New("goal_evaluation_wire_criterion_invalid")
				}
				criterionRef := fmt.Sprintf("criterion:%d.s%d.%d", goalIndex+1, stageIndex+1, criterionIndex+1)
				binding.bindCriterion(criterionRef, goalID, "stage", stage.ID, criterion.ID)
			}
		}
		for commitmentIndex, commitment := range entry.Commitments {
			ref := fmt.Sprintf("commitment:%d.%d", goalIndex+1, commitmentIndex+1)
			if commitment.ID == "" || commitment.GoalID != "" && commitment.GoalID != goalID || snapshot.FluctlightID != "" && commitment.FluctlightID != snapshot.FluctlightID || !goalEvaluationProfileAuthorized(snapshot.ProfileID, commitment.ProfileID) {
				return nil, errors.New("goal_evaluation_wire_commitment_invalid")
			}
			if _, duplicate := binding.commitmentRefs[commitment.ID]; duplicate {
				return nil, errors.New("goal_evaluation_wire_commitment_duplicate")
			}
			binding.commitmentsByRef[ref] = goalWireObjectBinding{GoalID: goalID, ID: commitment.ID}
			binding.commitmentRefs[commitment.ID] = ref
			for criterionIndex, criterion := range commitment.Criteria {
				if criterion.ID == "" {
					return nil, errors.New("goal_evaluation_wire_criterion_invalid")
				}
				criterionRef := fmt.Sprintf("criterion:%d.c%d.%d", goalIndex+1, commitmentIndex+1, criterionIndex+1)
				binding.bindCriterion(criterionRef, goalID, "commitment", commitment.ID, criterion.ID)
			}
		}
	}
	for goalIndex, entry := range snapshot.Goals {
		goalID := entry.Goal.EntityID
		for _, stage := range entry.Stages {
			for _, dependencyID := range stage.DependencyIDs {
				if _, ok := binding.stageRefsByID[dependencyID]; ok {
					continue
				}
				key := goalID + "\x1f" + dependencyID
				if _, ok := binding.dependencyRefs[key]; ok {
					continue
				}
				ref := fmt.Sprintf("dependency:%d.%d", goalIndex+1, len(binding.dependencyRefs)+1)
				binding.dependenciesByRef[ref] = goalWireObjectBinding{GoalID: goalID, ID: dependencyID}
				binding.dependencyRefs[key] = ref
			}
		}
	}
	for sourceIndex, source := range admittedGoalEvaluationSources(snapshot) {
		if source.Kind == "goal_revision" {
			continue
		}
		if source.Ref == "" {
			return nil, errors.New("goal_evaluation_wire_source_invalid")
		}
		if snapshot.FluctlightID != "" && source.FluctlightID != snapshot.FluctlightID {
			return nil, errors.New("goal_evaluation_wire_source_instance_invalid")
		}
		if !goalEvaluationProfileAuthorized(snapshot.ProfileID, source.ProfileID) {
			// Shared Goal evidence belongs to the continuous Actor, including
			// real messages published by another profile. Private Goals still
			// enforce their own source profile at hydration/commit.
			shared := false
			for _, entry := range snapshot.Goals {
				if entry.Goal.ProfileID == "" {
					shared = true
					break
				}
			}
			if !shared {
				return nil, errors.New("goal_evaluation_wire_source_profile_invalid")
			}
		}
		// Use a namespace distinct from the durable journal's source:N refs.
		ref := fmt.Sprintf("e%d", sourceIndex+1)
		if _, duplicate := binding.sourceRefs[source.Ref]; duplicate {
			return nil, errors.New("goal_evaluation_wire_source_duplicate")
		}
		binding.sourcesByRef[ref] = source
		binding.sourceRefs[source.Ref] = ref
		if source.SubjectActorID != "" {
			binding.ensureTargetActor(source.SubjectActorID)
		}
		binding.discoverSourceObjects(compactGoalEvaluationSourceData(source))
	}
	return binding, nil
}

func goalEvaluationProfileAuthorized(activeProfile, objectProfile string) bool {
	if objectProfile == "" {
		return true
	}
	return activeProfile != "" && objectProfile == activeProfile
}

func (binding *goalEvaluationWireBinding) bindCriterion(ref, goalID, objectKind, objectID, criterionID string) {
	binding.criteriaByRef[ref] = goalWireCriterionBinding{GoalID: goalID, ObjectKind: objectKind, ObjectID: objectID, CriterionID: criterionID}
	binding.criterionRefs[objectKind+"\x1f"+objectID+"\x1f"+criterionID] = ref
}

func (binding *goalEvaluationWireBinding) bindActor(ref, id string) {
	if id == "" {
		return
	}
	if existing := binding.actorRefs[id]; existing != "" {
		return
	}
	binding.actorsByRef[ref] = id
	binding.actorRefs[id] = ref
}

func (binding *goalEvaluationWireBinding) ensureTargetActor(id string) string {
	if id == "" {
		return ""
	}
	if ref := binding.actorRefs[id]; ref != "" {
		return ref
	}
	ref := fmt.Sprintf("actor_target:%d", len(binding.actorsByRef)-1)
	binding.bindActor(ref, id)
	return ref
}

func (binding *goalEvaluationWireBinding) discoverSourceObjects(value any) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := typed[key]
			normalized := strings.ToLower(key)
			if text, ok := child.(string); ok && text != "" {
				if strings.Contains(normalized, "actor") && strings.HasSuffix(normalized, "id") {
					binding.ensureTargetActor(text)
				} else if normalized == "id" || strings.HasSuffix(normalized, "_id") {
					binding.bindObject(text)
				} else if normalized == "ref" || strings.HasSuffix(normalized, "_ref") {
					if binding.sourceRefs[text] == "" && binding.goalRefsByID[text] == "" && binding.stageRefsByID[text] == "" && binding.commitmentRefs[text] == "" && binding.actorRefs[text] == "" {
						binding.bindObject(text)
					}
				}
			}
			if strings.HasSuffix(normalized, "_ids") || strings.HasSuffix(normalized, "_refs") {
				for _, item := range arrayValue(child) {
					if text := stringValue(item); text != "" {
						binding.bindObject(text)
					}
				}
			}
			binding.discoverSourceObjects(child)
		}
	case []any:
		for _, child := range typed {
			binding.discoverSourceObjects(child)
		}
	}
}

func (binding *goalEvaluationWireBinding) bindObject(id string) string {
	if id == "" {
		return ""
	}
	if ref := binding.objectRefs[id]; ref != "" {
		return ref
	}
	ref := fmt.Sprintf("object:%d", len(binding.objectRefs)+1)
	binding.objectRefs[id] = ref
	binding.objectsByRef[ref] = id
	return ref
}

func (binding *goalEvaluationWireBinding) stableDefinitions() map[string]any {
	actors := []any{map[string]any{"ref": "actor_self", "role": "self"}, map[string]any{"ref": "actor_user", "role": "user"}}
	for ref := range binding.actorsByRef {
		if strings.HasPrefix(ref, "actor_target:") {
			actors = append(actors, map[string]any{"ref": ref, "role": "target"})
		}
	}
	sort.Slice(actors[2:], func(i, j int) bool {
		return stringValue(actors[i+2].(map[string]any)["ref"]) < stringValue(actors[j+2].(map[string]any)["ref"])
	})
	goals := make([]any, 0, len(binding.snapshot.Goals))
	for _, entry := range binding.snapshot.Goals {
		goal := entry.Goal
		goalRef := binding.goalRefsByID[goal.EntityID]
		criteria := make([]any, 0, len(goal.CriterionIDs))
		for index, criterionID := range goal.CriterionIDs {
			criteria = append(criteria, map[string]any{"ref": binding.criterionRefs["goal\x1f"+goal.EntityID+"\x1f"+criterionID], "text": goal.SuccessCriteria[index]})
		}
		stages := make([]any, 0, len(entry.Stages))
		for _, stage := range entry.Stages {
			stageCriteria := make([]any, 0, len(stage.Criteria))
			for _, criterion := range stage.Criteria {
				stageCriteria = append(stageCriteria, map[string]any{"ref": binding.criterionRefs["stage\x1f"+stage.ID+"\x1f"+criterion.ID], "text": criterion.Text})
			}
			dependencies := make([]any, 0, len(stage.DependencyIDs))
			for _, dependencyID := range stage.DependencyIDs {
				ref := binding.stageRefsByID[dependencyID]
				if ref == "" {
					ref = binding.dependencyRefs[goal.EntityID+"\x1f"+dependencyID]
				}
				dependencies = append(dependencies, ref)
			}
			stages = append(stages, map[string]any{"ref": binding.stageRefsByID[stage.ID], "dependency_refs": dependencies, "purpose": stage.Purpose, "strategy": stage.Strategy, "entry_basis": stage.EntryBasis, "exit_basis": stage.ExitBasis, "criteria": stageCriteria})
		}
		commitments := make([]any, 0, len(entry.Commitments))
		for _, commitment := range entry.Commitments {
			commitmentCriteria := make([]any, 0, len(commitment.Criteria))
			for _, criterion := range commitment.Criteria {
				commitmentCriteria = append(commitmentCriteria, map[string]any{"ref": binding.criterionRefs["commitment\x1f"+commitment.ID+"\x1f"+criterion.ID], "text": criterion.Text})
			}
			item := map[string]any{"ref": binding.commitmentRefs[commitment.ID], "expected_result": commitment.ExpectedResult, "criteria": commitmentCriteria, "opportunity_condition": commitment.OpportunityCondition}
			if stageRef := binding.stageRefsByID[commitment.StageID]; stageRef != "" {
				item["stage_ref"] = stageRef
			}
			commitments = append(commitments, item)
		}
		definition := map[string]any{"ref": goalRef, "desired_outcome": goal.DesiredOutcome, "motivation": goal.Motivation, "scope": goal.Scope, "criteria": criteria, "criteria_policy": binding.projectCriteriaPolicy(goal), "deadline_policy": goal.DeadlinePolicy, "review_policy": effectiveGoalReviewPolicy(goal.ReviewPolicy), "stages": stages, "commitments": commitments}
		if targetRef := binding.actorRefs[goal.TargetActorID]; targetRef != "" {
			definition["target_actor_ref"] = targetRef
		}
		goals = append(goals, definition)
	}
	// Canonicalize nested typed policy objects as well as map keys. The
	// shared context hook copies JSON, so this is identical before/after it.
	return decodeObject(jsonBytes(map[string]any{"actors": actors, "goals": goals}))
}

func (binding *goalEvaluationWireBinding) projectCriteriaPolicy(goal GoalAuthority) map[string]any {
	policy := nonNilGoalCriteriaPolicy(goal.CriteriaPolicy)
	result := map[string]any{}
	for key, value := range policy {
		if key != "required" && key != "optional" && key != "optional_ids" {
			result[key] = value
			continue
		}
		refs := []any{}
		for _, raw := range arrayValue(value) {
			criterionID := stringValue(raw)
			if ref := binding.criterionRefs["goal\x1f"+goal.EntityID+"\x1f"+criterionID]; ref != "" {
				refs = append(refs, ref)
			}
		}
		if key == "optional_ids" {
			result["optional_refs"] = refs
		} else {
			result[key] = refs
		}
	}
	return result
}

func (binding *goalEvaluationWireBinding) currentInput() map[string]any {
	goals := make([]any, 0, len(binding.snapshot.Goals))
	for _, entry := range binding.snapshot.Goals {
		goal := entry.Goal
		state := map[string]any{"goal_ref": binding.goalRefsByID[goal.EntityID], "status": goal.Status, "deadline": goal.Deadline, "execution_hint": goalEvaluationExecutionHintInput(goal.ExecutionHint)}
		if ref := binding.stageRefsByID[goal.CurrentStageID]; ref != "" {
			state["current_stage_ref"] = ref
		}
		state["current_judgments"] = binding.projectJudgments(entry.CurrentJudgments, "goal", goal.EntityID)
		stageStates := make([]any, 0, len(entry.Stages))
		for _, stage := range entry.Stages {
			stageStates = append(stageStates, map[string]any{"ref": binding.stageRefsByID[stage.ID], "status": stage.Status, "reason": stage.Reason})
		}
		state["stage_states"] = stageStates
		commitmentStates := make([]any, 0, len(entry.Commitments))
		for _, commitment := range entry.Commitments {
			commitmentStates = append(commitmentStates, map[string]any{"ref": binding.commitmentRefs[commitment.ID], "status": commitment.Status, "window_start": commitment.WindowStart, "window_end": commitment.WindowEnd, "blocker": commitment.Blocker})
		}
		state["commitment_states"] = commitmentStates
		goals = append(goals, state)
	}
	sources := []any{}
	for _, source := range admittedGoalEvaluationSources(binding.snapshot) {
		if source.Kind == "goal_revision" {
			continue
		}
		goalRefs := make([]any, 0, len(source.GoalIDs))
		for _, goalID := range source.GoalIDs {
			if ref := binding.goalRefsByID[goalID]; ref != "" {
				goalRefs = append(goalRefs, ref)
			}
		}
		sources = append(sources, map[string]any{"ref": binding.sourceRefs[source.Ref], "kind": source.Kind, "subject_actor_ref": binding.actorRefs[source.SubjectActorID], "occurred_at": source.OccurredAt, "valid": source.Valid, "can_support_success": source.CanSupportSuccess, "goal_refs": goalRefs, "data": binding.projectSourceValue(compactGoalEvaluationSourceData(source), "")})
	}
	reviews := make([]any, 0, len(binding.snapshot.Reviews))
	for _, review := range binding.snapshot.Reviews {
		if goalRef := binding.goalRefsByID[review.GoalID]; goalRef != "" {
			reviews = append(reviews, map[string]any{"goal_ref": goalRef, "local_date": review.LocalDate, "timezone": review.Timezone, "window_start": review.WindowStart, "window_end": review.WindowEnd, "deadline_overdue": review.DeadlineOverdue})
		}
	}
	return map[string]any{"goal_states": goals, "sources": sources, "reviews": reviews}
}

func goalEvaluationExecutionHintInput(value map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"state", "blocker", "wait_condition", "next_step", "next_review_at", "ready_for_settlement"} {
		if child, ok := value[key]; ok {
			result[key] = child
		}
	}
	return result
}

func (binding *goalEvaluationWireBinding) projectJudgments(judgments []GoalCriterionJudgment, objectKind, objectID string) []any {
	result := make([]any, 0, len(judgments))
	for _, judgment := range judgments {
		refs := make([]any, 0, len(judgment.EvidenceRefs))
		for _, sourceRef := range judgment.EvidenceRefs {
			if ref := binding.sourceRefs[sourceRef]; ref != "" {
				refs = append(refs, ref)
			}
		}
		result = append(result, map[string]any{"criterion_ref": binding.criterionRefs[objectKind+"\x1f"+objectID+"\x1f"+judgment.CriterionID], "verdict": judgment.Verdict, "kind": judgment.Kind, "subject": judgment.Subject, "discourse": judgment.Discourse, "evidence_refs": refs, "reason": judgment.Reason})
	}
	return result
}

func (binding *goalEvaluationWireBinding) projectSourceValue(value any, key string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := map[string]any{}
		for childKey, child := range typed {
			normalized := strings.ToLower(childKey)
			if normalized == "participants" {
				actors := []any{}
				for _, participant := range arrayValue(child) {
					if ref := binding.actorRefs[stringValue(participant)]; ref != "" {
						actors = append(actors, ref)
					}
				}
				result[childKey] = actors
				continue
			}
			switch normalized {
			case "revision", "version", "schema_version", "criteria_version", "profile_id", "fluctlight_id", "conversation_id", "event_id", "request_id", "operation_id", "source_fingerprint", "source_digest", "projection_digest", "recorded_at", "updated_at", "created_at":
				continue
			}
			if normalized == "source_ref" || normalized == "evidence_ref" {
				if ref := binding.sourceRefs[stringValue(child)]; ref != "" {
					result[normalized] = ref
				}
				continue
			}
			if normalized == "evidence_refs" {
				refs := []any{}
				for _, item := range arrayValue(child) {
					if ref := binding.sourceRefs[stringValue(item)]; ref != "" {
						refs = append(refs, ref)
					}
				}
				result[childKey] = refs
				continue
			}
			if normalized == "ref" || strings.HasSuffix(normalized, "_ref") {
				text := stringValue(child)
				ref := binding.sourceRefs[text]
				if ref == "" {
					ref = binding.goalRefsByID[text]
				}
				if ref == "" {
					ref = binding.stageRefsByID[text]
				}
				if ref == "" {
					ref = binding.commitmentRefs[text]
				}
				if ref == "" {
					ref = binding.actorRefs[text]
				}
				if ref == "" {
					ref = binding.objectRefs[text]
				}
				if ref != "" {
					result[childKey] = ref
				}
				continue
			}
			if normalized == "goal_id" {
				if ref := binding.goalRefsByID[stringValue(child)]; ref != "" {
					result["goal_ref"] = ref
				}
				continue
			}
			if normalized == "stage_id" {
				if ref := binding.stageRefsByID[stringValue(child)]; ref != "" {
					result["stage_ref"] = ref
				}
				continue
			}
			if normalized == "commitment_id" {
				if ref := binding.commitmentRefs[stringValue(child)]; ref != "" {
					result["commitment_ref"] = ref
				}
				continue
			}
			if strings.HasSuffix(normalized, "_refs") {
				refs := []any{}
				for _, item := range arrayValue(child) {
					text := stringValue(item)
					ref := binding.sourceRefs[text]
					if ref == "" {
						ref = binding.objectRefs[text]
					}
					if ref != "" {
						refs = append(refs, ref)
					}
				}
				result[childKey] = refs
				continue
			}
			if strings.Contains(normalized, "actor") && strings.HasSuffix(normalized, "id") {
				if ref := binding.actorRefs[stringValue(child)]; ref != "" {
					result[strings.TrimSuffix(childKey, "id")+"ref"] = ref
				}
				continue
			}
			if normalized == "id" || strings.HasSuffix(normalized, "_id") {
				if ref := binding.objectRefs[stringValue(child)]; ref != "" {
					name := "ref"
					if normalized != "id" {
						name = strings.TrimSuffix(childKey, "id") + "ref"
					}
					result[name] = ref
				}
				continue
			}
			if strings.HasSuffix(normalized, "_ids") {
				refs := []any{}
				for _, item := range arrayValue(child) {
					if ref := binding.objectRefs[stringValue(item)]; ref != "" {
						refs = append(refs, ref)
					}
				}
				result[strings.TrimSuffix(childKey, "ids")+"refs"] = refs
				continue
			}
			result[childKey] = binding.projectSourceValue(child, childKey)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			result = append(result, binding.projectSourceValue(child, key))
		}
		return result
	default:
		return value
	}
}

func goalEvaluationWireInput(snapshot goalEvaluationSnapshot) (*goalEvaluationWireBinding, map[string]any, map[string]any, error) {
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		return nil, nil, nil, err
	}
	return binding, binding.stableDefinitions(), binding.currentInput(), nil
}

func (binding *goalEvaluationWireBinding) hydrateOutput(value map[string]any) (GoalEvaluationTaskOutput, error) {
	var wire goalEvaluationWireOutput
	decoder := json.NewDecoder(bytes.NewReader(jsonBytes(value)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_output_invalid")
	}
	output := GoalEvaluationTaskOutput{Evaluations: make([]GoalEvaluationCandidate, 0, len(wire.Evaluations)), Plans: make([]GoalPlanCandidate, 0, len(wire.Plans))}
	seenEvaluations := map[string]bool{}
	for _, candidate := range wire.Evaluations {
		entry, ok := binding.goalsByRef[candidate.GoalRef]
		if !ok || seenEvaluations[candidate.GoalRef] {
			return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_goal_ref_invalid")
		}
		seenEvaluations[candidate.GoalRef] = true
		goalID := entry.Goal.EntityID
		judgments, err := binding.hydrateJudgments(goalID, "goal", goalID, candidate.Judgments)
		if err != nil {
			return GoalEvaluationTaskOutput{}, err
		}
		hydrated := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: effectiveGoalCriteriaVersion(entry.Goal), Judgments: judgments, Impact: candidate.Impact, Blocker: candidate.Blocker, WaitCondition: candidate.WaitCondition, NextStep: candidate.NextStep, NextReviewAt: candidate.NextReviewAt, ResidualMotivation: candidate.ResidualMotivation, Followup: candidate.Followup}
		if candidate.StageEvaluation != nil {
			hydrated.StageEvaluation, err = binding.hydrateObjectEvaluation(goalID, "stage", *candidate.StageEvaluation)
			if err != nil {
				return GoalEvaluationTaskOutput{}, err
			}
		}
		seenCommitments := map[string]bool{}
		for _, evaluation := range candidate.CommitmentEvaluations {
			if seenCommitments[evaluation.ObjectRef] {
				return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_commitment_evaluation_duplicate")
			}
			seenCommitments[evaluation.ObjectRef] = true
			item, itemErr := binding.hydrateObjectEvaluation(goalID, "commitment", evaluation)
			if itemErr != nil {
				return GoalEvaluationTaskOutput{}, itemErr
			}
			hydrated.CommitmentEvaluations = append(hydrated.CommitmentEvaluations, *item)
		}
		if candidate.RelationshipConfirmation != nil {
			actorID, ok := binding.actorsByRef[candidate.RelationshipConfirmation.TargetActorRef]
			if !ok || actorID == "" || actorID != entry.Goal.TargetActorID {
				return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_actor_ref_invalid")
			}
			refs, refsErr := binding.hydrateSourceRefs(candidate.RelationshipConfirmation.EvidenceRefs)
			if refsErr != nil {
				return GoalEvaluationTaskOutput{}, refsErr
			}
			hydrated.RelationshipConfirmation = &GoalRelationshipConfirmation{TargetActorID: actorID, EvidenceRefs: refs, Label: candidate.RelationshipConfirmation.Label}
		}
		if candidate.Review != nil {
			refs, refsErr := binding.hydrateSourceRefs(candidate.Review.EvidenceRefs)
			if refsErr != nil {
				return GoalEvaluationTaskOutput{}, refsErr
			}
			stageID := ""
			if candidate.Review.StageRef != "" {
				stage, ok := binding.stagesByRef[candidate.Review.StageRef]
				if !ok || stage.GoalID != goalID {
					return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_review_stage_ref_invalid")
				}
				stageID = stage.ID
			}
			hydrated.Review = &GoalReviewDecision{ReasonCategory: candidate.Review.ReasonCategory, Decision: candidate.Review.Decision, Explanation: candidate.Review.Explanation, EvidenceRefs: refs, StageID: stageID, FeasibleAlternative: candidate.Review.FeasibleAlternative}
		}
		output.Evaluations = append(output.Evaluations, hydrated)
	}
	seenPlans := map[string]bool{}
	for _, plan := range wire.Plans {
		entry, ok := binding.goalsByRef[plan.GoalRef]
		if !ok || seenPlans[plan.GoalRef] {
			return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_plan_ref_invalid")
		}
		seenPlans[plan.GoalRef] = true
		goalID := entry.Goal.EntityID
		hydrated := GoalPlanCandidate{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: effectiveGoalCriteriaVersion(entry.Goal), Reason: plan.Reason, NextStep: plan.NextStep, WaitCondition: plan.WaitCondition, NextReviewAt: plan.NextReviewAt}
		if plan.Stage != nil {
			operation := firstString(plan.Stage.Operation, "create")
			if (operation != "create" && operation != "adjust" && operation != "skip") || (operation == "create" && plan.Stage.ObjectRef != "") || (operation != "create" && plan.Stage.ObjectRef == "") {
				return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_stage_operation_invalid")
			}
			stage := &GoalStagePlan{Operation: plan.Stage.Operation, Purpose: plan.Stage.Purpose, Strategy: plan.Stage.Strategy, EntryBasis: plan.Stage.EntryBasis, ExitBasis: plan.Stage.ExitBasis, Criteria: plan.Stage.Criteria, Reason: plan.Stage.Reason}
			if plan.Stage.ObjectRef != "" {
				object, ok := binding.stagesByRef[plan.Stage.ObjectRef]
				if !ok || object.GoalID != goalID {
					return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_stage_ref_invalid")
				}
				stage.ID, stage.ExpectedRevision = object.ID, goalStageRevision(entry.Stages, object.ID)
			}
			seenDependencies := map[string]bool{}
			for _, ref := range plan.Stage.DependencyRefs {
				if seenDependencies[ref] {
					return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_dependency_duplicate")
				}
				seenDependencies[ref] = true
				object, ok := binding.stagesByRef[ref]
				if !ok {
					object, ok = binding.dependenciesByRef[ref]
				}
				if !ok || object.GoalID != goalID {
					return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_dependency_ref_invalid")
				}
				stage.DependencyIDs = append(stage.DependencyIDs, object.ID)
			}
			hydrated.Stage = stage
		}
		if plan.Commitment != nil {
			operation := firstString(plan.Commitment.Operation, "create")
			if (operation != "create" && operation != "adjust" && operation != "abandon") || (operation == "create" && plan.Commitment.ObjectRef != "") || (operation != "create" && plan.Commitment.ObjectRef == "") {
				return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_commitment_operation_invalid")
			}
			commitment := &GoalCommitmentPlan{Operation: plan.Commitment.Operation, Reason: plan.Commitment.Reason, ExpectedResult: plan.Commitment.ExpectedResult, Criteria: plan.Commitment.Criteria, WindowStart: plan.Commitment.WindowStart, WindowEnd: plan.Commitment.WindowEnd, OpportunityCondition: plan.Commitment.OpportunityCondition, Blocker: plan.Commitment.Blocker}
			if plan.Commitment.ObjectRef != "" {
				object, ok := binding.commitmentsByRef[plan.Commitment.ObjectRef]
				if !ok || object.GoalID != goalID {
					return GoalEvaluationTaskOutput{}, errors.New("goal_evaluation_wire_commitment_ref_invalid")
				}
				commitment.ID, commitment.ExpectedRevision = object.ID, goalCommitmentRevision(entry.Commitments, object.ID)
			}
			hydrated.Commitment = commitment
		}
		output.Plans = append(output.Plans, hydrated)
	}
	return output, nil
}

func (binding *goalEvaluationWireBinding) hydrateJudgments(goalID, objectKind, objectID string, values []goalEvaluationWireJudgment) ([]GoalCriterionJudgment, error) {
	result := make([]GoalCriterionJudgment, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		criterion, ok := binding.criteriaByRef[value.CriterionRef]
		if !ok || criterion.GoalID != goalID || criterion.ObjectKind != objectKind || criterion.ObjectID != objectID || seen[value.CriterionRef] {
			return nil, errors.New("goal_evaluation_wire_criterion_ref_invalid")
		}
		seen[value.CriterionRef] = true
		refs, err := binding.hydrateSourceRefs(value.EvidenceRefs)
		if err != nil {
			return nil, err
		}
		result = append(result, GoalCriterionJudgment{CriterionID: criterion.CriterionID, Verdict: value.Verdict, Kind: value.Kind, Subject: value.Subject, Discourse: value.Discourse, EvidenceRefs: refs, Reason: value.Reason})
	}
	return result, nil
}

func (binding *goalEvaluationWireBinding) hydrateObjectEvaluation(goalID, kind string, value goalEvaluationWireObjectEvaluation) (*GoalObjectEvaluation, error) {
	var object goalWireObjectBinding
	var ok bool
	if kind == "stage" {
		object, ok = binding.stagesByRef[value.ObjectRef]
	} else {
		object, ok = binding.commitmentsByRef[value.ObjectRef]
	}
	if !ok || object.GoalID != goalID {
		return nil, errors.New("goal_evaluation_wire_object_ref_invalid")
	}
	judgments, err := binding.hydrateJudgments(goalID, kind, object.ID, value.Judgments)
	if err != nil {
		return nil, err
	}
	result := &GoalObjectEvaluation{ID: object.ID, Judgments: judgments, Completed: value.Completed, Reason: value.Reason}
	entry := binding.goalsByRef[binding.goalRefsByID[goalID]]
	if kind == "stage" {
		result.ExpectedRevision, result.CriteriaVersion = goalStageRevision(entry.Stages, object.ID), goalStageCriteriaVersion(entry.Stages, object.ID)
	} else {
		result.ExpectedRevision, result.CriteriaVersion = goalCommitmentRevision(entry.Commitments, object.ID), goalCommitmentCriteriaVersion(entry.Commitments, object.ID)
	}
	return result, nil
}

func (binding *goalEvaluationWireBinding) hydrateSourceRefs(refs []string) ([]string, error) {
	result := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		source, ok := binding.sourcesByRef[ref]
		if !ok || seen[ref] {
			return nil, errors.New("goal_evaluation_wire_source_ref_invalid")
		}
		seen[ref] = true
		result = append(result, source.Ref)
	}
	return result, nil
}

func goalStageRevision(values []GoalStage, id string) int {
	for _, value := range values {
		if value.ID == id {
			return value.Revision
		}
	}
	return 0
}
func goalStageCriteriaVersion(values []GoalStage, id string) int {
	for _, value := range values {
		if value.ID == id {
			return value.CriteriaVersion
		}
	}
	return 0
}
func goalCommitmentRevision(values []GoalCommitment, id string) int {
	for _, value := range values {
		if value.ID == id {
			return value.Revision
		}
	}
	return 0
}
func goalCommitmentCriteriaVersion(values []GoalCommitment, id string) int {
	for _, value := range values {
		if value.ID == id {
			return value.CriteriaVersion
		}
	}
	return 0
}
