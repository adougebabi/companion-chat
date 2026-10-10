package core

import (
	"errors"
)

// validateGoalEvaluationOutput is the model-output preflight. It uses only the
// frozen offered authority and sources; persistence repeats the same semantic
// checks after re-reading live authority before any write.
func validateGoalEvaluationOutput(snapshot goalEvaluationSnapshot, output GoalEvaluationTaskOutput) error {
	sources := make(map[string]GoalSource, len(snapshot.Sources))
	for _, source := range admittedGoalEvaluationSources(snapshot) {
		sources[source.Ref] = source
	}
	entries := make(map[string]goalEvaluationGoal, len(snapshot.Goals))
	for _, entry := range snapshot.Goals {
		goalID := entry.Goal.EntityID
		if goalID == "" {
			goalID = entry.GoalID
			entry.Goal.EntityID = goalID
		}
		entries[goalID] = entry
	}
	for _, candidate := range output.Evaluations {
		entry, ok := entries[candidate.GoalID]
		if !ok {
			return errors.New("goal_evaluation_scope_invalid")
		}
		if _, _, _, err := validateGoalCompletionSemantics(entry.Goal, candidate, sources); err != nil {
			return err
		}
		if candidate.StageEvaluation != nil {
			if _, err := validateGoalObjectEvaluation(entry.Goal, *candidate.StageEvaluation, "stage", entry.Stages, entry.Commitments, sources); err != nil {
				return err
			}
		}
		for _, commitment := range candidate.CommitmentEvaluations {
			if _, err := validateGoalObjectEvaluation(entry.Goal, commitment, "commitment", entry.Stages, entry.Commitments, sources); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateGoalObjectEvaluation(parent GoalAuthority, candidate GoalObjectEvaluation, kind string, stages []GoalStage, commitments []GoalCommitment, sources map[string]GoalSource) ([]GoalCriterionJudgment, error) {
	criteria := []GoalCriterion{}
	revision, version := 0, 0
	switch kind {
	case "stage":
		for _, stage := range stages {
			if stage.ID == candidate.ID && stage.GoalID == parent.EntityID && stage.FluctlightID == parent.FluctlightID && goalObjectProfileAuthorized(parent.ProfileID, stage.ProfileID) {
				criteria, revision, version = stage.Criteria, stage.Revision, stage.CriteriaVersion
			}
		}
	case "commitment":
		for _, commitment := range commitments {
			if commitment.ID == candidate.ID && commitment.GoalID == parent.EntityID && commitment.FluctlightID == parent.FluctlightID && goalObjectProfileAuthorized(parent.ProfileID, commitment.ProfileID) {
				criteria, revision, version = commitment.Criteria, commitment.Revision, commitment.CriteriaVersion
			}
		}
	default:
		return nil, errors.New("goal_object_evaluation_kind_invalid")
	}
	if revision == 0 || revision != candidate.ExpectedRevision || version != candidate.CriteriaVersion {
		return nil, errors.New("goal_object_evaluation_version_conflict")
	}
	scoped := parent
	scoped.CriterionIDs = nil
	scoped.SuccessCriteria = nil
	scoped.CriteriaPolicy = nonNilGoalCriteriaPolicy(nil)
	for _, criterion := range criteria {
		scoped.CriterionIDs = append(scoped.CriterionIDs, criterion.ID)
		scoped.SuccessCriteria = append(scoped.SuccessCriteria, criterion.Text)
	}
	judgments, complete, _, err := validatedGoalJudgments(scoped, candidate.Judgments, sources)
	if err != nil {
		return nil, err
	}
	if candidate.Completed && !complete {
		return nil, errors.New("goal_object_completion_criteria_incomplete")
	}
	if complete && !candidate.Completed {
		return nil, errors.New("goal_object_evaluation_completion_mismatch")
	}
	return judgments, nil
}

func goalObjectProfileAuthorized(parentProfile, objectProfile string) bool {
	return parentProfile == "" || objectProfile == "" || objectProfile == parentProfile
}
