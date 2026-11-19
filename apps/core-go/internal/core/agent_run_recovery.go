package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Opaque tool output and frozen context numbers must survive recovery exactly.
// Completion retains its established decoding; only Trace is decoded losslessly.
func decodeRecordedAgentResult(raw []byte) (*ADKStructuredTaskResult, error) {
	result := &ADKStructuredTaskResult{}
	if len(raw) != 0 {
		type recordedAgentResult ADKStructuredTaskResult
		envelope := struct {
			*recordedAgentResult
			Trace json.RawMessage
		}{recordedAgentResult: (*recordedAgentResult)(result)}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return result, err
		}
		if len(envelope.Trace) > 0 && string(envelope.Trace) != "null" {
			decoder := json.NewDecoder(bytes.NewReader(envelope.Trace))
			decoder.UseNumber()
			if err := decoder.Decode(&result.Trace); err != nil {
				return result, err
			}
		}
	}
	if result.Trace == nil {
		result.Trace = &ADKCapabilityTrace{}
	}
	return result, nil
}

func (a *App) recoverCommittedAgentTrace(ctx context.Context, record *formalRunRecord, trace *ADKCapabilityTrace) (*ADKCapabilityTrace, error) {
	merged := trace.Clone()
	if merged == nil {
		merged = &ADKCapabilityTrace{}
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT capability_name,operation_id,invocation,result,COUNT(*) OVER(PARTITION BY capability_name,COALESCE(NULLIF(invocation->>'call_id',''),result->>'call_id')) FROM public.tool_executions WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3 ORDER BY committed_at,capability_name,operation_id`, record.FluctlightID, record.AgentID, record.RunID)
	if err != nil {
		return merged, err
	}
	defer rows.Close()
	usedResults := map[int]bool{}
	for rows.Next() {
		var name, operation string
		var invocationJSON, resultJSON []byte
		var nativeMatches int
		if err := rows.Scan(&name, &operation, &invocationJSON, &resultJSON, &nativeMatches); err != nil {
			return merged, err
		}
		var invocation CapabilityInvocation
		var receipt CapabilityResult
		decoder := json.NewDecoder(bytes.NewReader(invocationJSON))
		decoder.UseNumber()
		if err := decoder.Decode(&invocation); err != nil {
			return merged, fmt.Errorf("decode committed Agent invocation: %w", err)
		}
		decoder = json.NewDecoder(bytes.NewReader(resultJSON))
		decoder.UseNumber()
		if err := decoder.Decode(&receipt); err != nil {
			return merged, fmt.Errorf("decode committed Agent tool result: %w", err)
		}
		if invocation.CapabilityName != "" && invocation.CapabilityName != name || receipt.CapabilityName != "" && receipt.CapabilityName != name || invocation.Metadata.OperationID != "" && invocation.Metadata.OperationID != operation {
			return merged, errors.New("committed Agent receipt identity invalid")
		}
		if invocation.CallID == "" {
			invocation.CallID = receipt.CallID
		}
		if receipt.CallID == "" {
			receipt.CallID = invocation.CallID
		}
		if invocation.CallID == "" || invocation.CallID != receipt.CallID {
			return merged, errors.New("committed Agent receipt call identity invalid")
		}
		invocation.CapabilityName, receipt.CapabilityName = name, name
		invocation.Metadata.OperationID = operation
		mergeCommittedAgentReceipt(merged, invocation, receipt, usedResults, nativeMatches == 1)
	}
	return merged, rows.Err()
}

// Existing native calls, queries and failures are retained. A completed record
// for the same business operation may carry another native call ID on replay.
func mergeCommittedAgentReceipt(trace *ADKCapabilityTrace, invocation CapabilityInvocation, receipt CapabilityResult, usedResults map[int]bool, legacyUnique bool) {
	sameCallInvocation := false
	callBelongsToOtherOperation := false
	for invocationIndex, prior := range trace.Invocations {
		if prior.CapabilityName != invocation.CapabilityName {
			continue
		}
		sameOperation := prior.Metadata.OperationID != "" && prior.Metadata.OperationID == invocation.Metadata.OperationID
		legacyCall := legacyUnique && prior.Metadata.OperationID == "" && prior.CallID == invocation.CallID && sameRecordedInvocationArguments(prior, invocation)
		if prior.CallID == invocation.CallID && prior.Metadata.OperationID != "" && !sameOperation {
			callBelongsToOtherOperation = true
		}
		if !sameOperation && !legacyCall {
			continue
		}
		if legacyCall {
			trace.Invocations[invocationIndex].Metadata.OperationID = invocation.Metadata.OperationID
		}
		for index, result := range trace.Results {
			if !usedResults[index] && result.CallID == prior.CallID && result.CapabilityName == prior.CapabilityName && sameRecoveredAgentReceipt(result, receipt) {
				usedResults[index] = true
				return
			}
		}
		if prior.CallID == invocation.CallID {
			sameCallInvocation = true
		}
	}
	if sameCallInvocation {
		// Do not attach the receipt to an unrelated array position or invent a
		// model delivery to another incomplete native call.
		trace.AppendResult(receipt)
		usedResults[len(trace.Results)-1] = true
		return
	}
	for index, result := range trace.Results {
		if !callBelongsToOtherOperation && !usedResults[index] && result.CallID == receipt.CallID && result.CapabilityName == receipt.CapabilityName && sameRecoveredAgentReceipt(result, receipt) {
			trace.AppendInvocation(invocation)
			usedResults[index] = true
			return
		}
	}
	trace.Append(invocation, receipt)
	usedResults[len(trace.Results)-1] = true
}

func sameRecoveredAgentReceipt(left, right CapabilityResult) bool {
	// CallID can differ for a genuine business replay. All actual receipt data
	// must still match; a failed conflicting invocation cannot hide a commit.
	if left.CapabilityName != right.CapabilityName || left.Status != right.Status || left.ErrorCode != right.ErrorCode || left.Retryable != right.Retryable {
		return false
	}
	for _, result := range []*CapabilityResult{&left, &right} {
		if result.Status == "failed" || result.Status == "rejected" || result.Status == "handled" {
			result.Output = redactDiagnosticFailureOutput(result.Output)
		}
	}
	// ExecuteTool enriches provenance/required-context after the ledger write.
	// Its replay reader also changes the reserved top-level replay marker.
	if l, ok := left.Output.(map[string]any); ok {
		if r, ok := right.Output.(map[string]any); ok {
			_, leftReplay := l["replayed"].(bool)
			_, rightReplay := r["replayed"].(bool)
			if leftReplay && rightReplay {
				lc, rc := map[string]any{}, map[string]any{}
				for key, value := range l {
					lc[key] = value
				}
				for key, value := range r {
					rc[key] = value
				}
				lc["replayed"], rc["replayed"] = false, false
				left.Output, right.Output = lc, rc
			}
		}
	}
	return bytes.Equal(jsonBytes(left.Output), jsonBytes(right.Output))
}

func sameRecordedInvocationArguments(left, right CapabilityInvocation) bool {
	compact := func(raw []byte) []byte {
		if len(raw) == 0 {
			return nil
		}
		var output bytes.Buffer
		if json.Compact(&output, raw) != nil {
			return raw
		}
		return output.Bytes()
	}
	return bytes.Equal(compact(left.Arguments), compact(right.Arguments)) && bytes.Equal(compact(left.PreparedPayload), compact(right.PreparedPayload))
}

func (a *App) restoreRecordedAgentRun(ctx context.Context, record *formalRunRecord, digest string) (*formalRunRecord, *ADKStructuredTaskResult, error) {
	var latest *ADKStructuredTaskResult
	for attempt := 0; attempt < 3; attempt++ {
		var storedDigest, status, failure string
		var encoded []byte
		var started time.Time
		if err := a.DB.Pool().QueryRow(ctx, `SELECT input_digest,status,error_detail,result,started_at FROM public.agent_runs WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, record.FluctlightID, record.AgentID, record.RunID).Scan(&storedDigest, &status, &failure, &encoded, &started); err != nil {
			return nil, latest, err
		}
		if storedDigest != digest {
			return nil, &ADKStructuredTaskResult{Trace: &ADKCapabilityTrace{}}, newCapabilityError("agent_run_input_conflict", false, ErrConflict)
		}
		result, err := decodeRecordedAgentResult(encoded)
		if err != nil {
			safe := sanitizeAgentRunResult(*result)
			return nil, &safe, fmt.Errorf("decode Agent result: %w", err)
		}
		if status == "completed" {
			safe := sanitizeAgentRunResult(*result)
			return nil, &safe, nil
		}
		if status != "running" && status != "failed" {
			return nil, result, errors.New("agent_run_status_invalid")
		}
		before := jsonBytes(result.Trace)
		result.Trace, err = a.recoverCommittedAgentTrace(ctx, record, result.Trace)
		safe := sanitizeAgentRunResult(*result)
		latest = &safe
		if err != nil {
			return nil, latest, err
		}
		if status == "running" && time.Since(started) <= 5*time.Minute {
			return nil, latest, errors.New("agent_run_incomplete: inspect committed results before starting a new business run")
		}
		traceJSON := jsonBytes(safe.Trace)
		if status == "failed" && bytes.Equal(before, traceJSON) {
			return nil, latest, fmt.Errorf("agent_run_failed: %s", boundedOperationalCause(failure))
		}
		// Patch only the owned Trace field. Unknown envelope fields and exact
		// numbers outside Trace must not be rewritten by a recovery read.
		query := `UPDATE public.agent_runs SET result=(result-'trace'-'Trace')||jsonb_build_object('Trace',$4::jsonb) WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3 AND status='failed' AND result=$5::jsonb`
		if status == "running" {
			query = `UPDATE public.agent_runs SET status='failed',error_detail='stale_run_timeout',failure_stage='timeout',failure_code='request_timeout',finished_at=now(),result=(result-'trace'-'Trace')||jsonb_build_object('Trace',$4::jsonb) WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3 AND status='running' AND result=$5::jsonb`
			failure = "stale_run_timeout"
		}
		tag, err := a.DB.Pool().Exec(ctx, query, record.FluctlightID, record.AgentID, record.RunID, traceJSON, encoded)
		if err != nil {
			return nil, latest, fmt.Errorf("record Agent recovery: %w", err)
		}
		if tag.RowsAffected() == 1 {
			return nil, latest, fmt.Errorf("agent_run_failed: %s", boundedOperationalCause(failure))
		}
		// A concurrent finish/recovery won. Read its actual state instead of
		// reporting timeout or overwriting a completed result.
	}
	return nil, latest, ErrConflict
}
