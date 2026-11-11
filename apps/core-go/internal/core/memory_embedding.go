package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (a *App) ProcessMemoryEmbeddingIntentAt(ctx context.Context, intentID, memoryID string, requestedRevision int, providerEndpointID, modelID string) (map[string]any, error) {
	memoryID = strings.TrimSpace(memoryID)
	if memoryID == "" {
		return nil, errors.New("memory_id_required")
	}
	if requestedRevision < 0 {
		return nil, errors.New("memory_embedding_revision_invalid")
	}
	var legacyEndpoint, legacyModel string
	if strings.TrimSpace(intentID) != "" {
		var payload []byte
		var contract string
		var snapshotExists bool
		if err := a.DB.Pool().QueryRow(ctx, `SELECT payload,embedding_snapshot_contract,EXISTS(SELECT 1 FROM public.memory_embedding_model_snapshots WHERE intent_id=$1) FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='memory.embedding'`, intentID).Scan(&payload, &contract, &snapshotExists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("memory_embedding_intent_not_found")
			}
			return nil, err
		}
		var identity struct {
			MemoryID   string `json:"memory_id"`
			Revision   *int   `json:"revision"`
			EndpointID string `json:"provider_endpoint_id"`
			ModelID    string `json:"model_id"`
		}
		if err := json.Unmarshal(payload, &identity); err != nil || strings.TrimSpace(identity.MemoryID) != memoryID || identity.Revision == nil || *identity.Revision != requestedRevision {
			return nil, errors.New("memory_embedding_intent_conflict")
		}
		if contract == "" {
			if snapshotExists {
				return nil, errMemoryEmbeddingModelSnapshotInvalid
			}
			legacyEndpoint, legacyModel = strings.TrimSpace(identity.EndpointID), strings.TrimSpace(identity.ModelID)
		}
	}
	var content, status, provenanceStatus string
	var revision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT content,revision,status,provenance_status FROM public.memories WHERE id=$1`, memoryID).Scan(&content, &revision, &status, &provenanceStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]any{"memory_id": memoryID, "status": "not_found", "requested_revision": requestedRevision}, nil
		}
		return nil, err
	}
	supported, err := memorySourcesStillValid(ctx, a.DB.Pool(), memoryID, revision, provenanceStatus)
	if err != nil {
		return nil, err
	}
	if status != "active" || revision != requestedRevision || !supported {
		return map[string]any{"memory_id": memoryID, "status": "stale", "revision": revision, "requested_revision": requestedRevision}, nil
	}
	if legacyEndpoint != "" && legacyModel != "" {
		requestedEndpoint, requestedModel := strings.TrimSpace(providerEndpointID), strings.TrimSpace(modelID)
		if (requestedEndpoint == "") != (requestedModel == "") {
			return nil, errors.New("memory_embedding_assignment_incomplete")
		}
		if (requestedEndpoint != "" && requestedEndpoint != legacyEndpoint) || (requestedModel != "" && requestedModel != legacyModel) {
			return nil, errors.New("memory_embedding_assignment_conflict")
		}
		legacyAssignment := providerAssignment{Role: "embedding", EndpointID: legacyEndpoint, ModelID: legacyModel}
		embeddingID := "embedding_" + stableDigest(memoryID+":"+fmt.Sprint(requestedRevision)+":"+legacyModel)
		ready, err := a.prepareMemoryEmbeddingAttempt(ctx, memoryID, requestedRevision, embeddingID, legacyAssignment, false)
		if err != nil {
			if err.Error() == "memory_embedding_stale" {
				return map[string]any{"memory_id": memoryID, "status": "stale", "revision": revision, "requested_revision": requestedRevision}, nil
			}
			return nil, err
		}
		if ready {
			return map[string]any{"memory_id": memoryID, "status": "ready", "revision": requestedRevision, "model_id": legacyModel, "replayed": true}, nil
		}
	}
	assignment, restoreCredential, err := a.resolveMemoryEmbeddingAssignmentState(ctx, intentID, providerEndpointID, modelID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("memory_embedding_unconfigured")
		}
		return nil, err
	}
	embeddingID := "embedding_" + stableDigest(memoryID+":"+fmt.Sprint(requestedRevision)+":"+assignment.ModelID)
	if restoreCredential {
		ready, err := a.prepareMemoryEmbeddingAttempt(ctx, memoryID, requestedRevision, embeddingID, assignment, false)
		if err != nil {
			if err.Error() == "memory_embedding_stale" {
				return map[string]any{"memory_id": memoryID, "status": "stale", "revision": revision, "requested_revision": requestedRevision}, nil
			}
			return nil, err
		}
		if ready {
			return map[string]any{"memory_id": memoryID, "status": "ready", "revision": requestedRevision, "model_id": assignment.ModelID, "replayed": true}, nil
		}
		if err := a.restoreMemoryEmbeddingCredential(ctx, &assignment); err != nil {
			return nil, fmt.Errorf("memory_embedding_model_snapshot_restore: %w", err)
		}
	}
	ready, err := a.prepareMemoryEmbeddingAttempt(ctx, memoryID, requestedRevision, embeddingID, assignment, true)
	if err != nil {
		if err.Error() == "memory_embedding_stale" {
			return map[string]any{"memory_id": memoryID, "status": "stale", "revision": revision, "requested_revision": requestedRevision}, nil
		}
		return nil, err
	}
	if ready {
		return map[string]any{"memory_id": memoryID, "status": "ready", "revision": requestedRevision, "model_id": assignment.ModelID, "replayed": true}, nil
	}
	model, vector, providerErr := a.RunFrozenEmbeddingTask(ctx, content, assignment)
	if providerErr != nil {
		if settleErr := a.settleMemoryEmbeddingFailure(ctx, memoryID, requestedRevision, embeddingID, assignment, "provider_request_failed"); settleErr != nil {
			return nil, fmt.Errorf("embedding Provider failed (%v) and failure settlement failed: %w", providerErr, settleErr)
		}
		return nil, providerErr
	}
	if model != assignment.ModelID || len(vector) == 0 {
		resultErr := errors.New("memory_embedding_provider_result_invalid")
		if settleErr := a.settleMemoryEmbeddingFailure(ctx, memoryID, requestedRevision, embeddingID, assignment, "provider_result_invalid"); settleErr != nil {
			return nil, fmt.Errorf("%v: %w", resultErr, settleErr)
		}
		return nil, resultErr
	}
	encoded := make([]string, len(vector))
	for index, value := range vector {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			resultErr := errors.New("memory_embedding_vector_invalid")
			if settleErr := a.settleMemoryEmbeddingFailure(ctx, memoryID, requestedRevision, embeddingID, assignment, "provider_vector_invalid"); settleErr != nil {
				return nil, fmt.Errorf("%v: %w", resultErr, settleErr)
			}
			return nil, resultErr
		}
		encoded[index] = fmt.Sprintf("%g", value)
	}
	settled, err := a.settleMemoryEmbeddingReady(ctx, memoryID, requestedRevision, embeddingID, assignment, vector, "["+strings.Join(encoded, ",")+"]")
	if err != nil {
		return nil, err
	}
	if !settled {
		return map[string]any{"memory_id": memoryID, "status": "stale", "revision": revision, "requested_revision": requestedRevision}, nil
	}
	return map[string]any{"memory_id": memoryID, "status": "ready", "revision": requestedRevision, "dimensions": len(vector), "model_id": model, "replayed": false}, nil
}

func (a *App) resolveMemoryEmbeddingAssignment(ctx context.Context, intentID, providerEndpointID, modelID string) (providerAssignment, error) {
	assignment, restoreCredential, err := a.resolveMemoryEmbeddingAssignmentState(ctx, intentID, providerEndpointID, modelID)
	if err == nil && restoreCredential {
		err = a.restoreMemoryEmbeddingCredential(ctx, &assignment)
	}
	return assignment, err
}

func (a *App) resolveMemoryEmbeddingAssignmentState(ctx context.Context, intentID, providerEndpointID, modelID string) (providerAssignment, bool, error) {
	providerEndpointID = strings.TrimSpace(providerEndpointID)
	modelID = strings.TrimSpace(modelID)
	if (providerEndpointID == "") != (modelID == "") {
		return providerAssignment{}, false, errors.New("memory_embedding_assignment_incomplete")
	}
	if strings.TrimSpace(intentID) != "" {
		assignment, frozen, err := a.resolveFrozenMemoryEmbedding(ctx, intentID, providerEndpointID, modelID)
		if err != nil || frozen {
			return assignment, frozen, err
		}
		var owner string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT m.owner_fluctlight_id FROM public.platform_workflow_intents w JOIN public.memories m ON m.id=w.payload->>'memory_id' WHERE w.intent_id=$1`, intentID).Scan(&owner); err != nil {
			return providerAssignment{}, false, err
		}
		a.recordDiagnosticEvent(ctx, "memory_embedding.model_binding.legacy_unfrozen", "warn", owner, intentID, intentID, map[string]any{"reason_code": "legacy_model_binding_unavailable"})
		var payload []byte
		if err := a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='memory.embedding'`, intentID).Scan(&payload); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return providerAssignment{}, false, err
		} else if err == nil {
			value := decodeObject(payload)
			storedEndpointID := stringValue(value["provider_endpoint_id"])
			storedModelID := stringValue(value["model_id"])
			if (storedEndpointID == "") != (storedModelID == "") {
				return providerAssignment{}, false, errors.New("memory_embedding_assignment_incomplete")
			}
			if storedEndpointID != "" {
				if (providerEndpointID != "" && providerEndpointID != storedEndpointID) || (modelID != "" && modelID != storedModelID) {
					return providerAssignment{}, false, errors.New("memory_embedding_assignment_conflict")
				}
				providerEndpointID, modelID = storedEndpointID, storedModelID
			} else if providerEndpointID != "" || modelID != "" {
				return providerAssignment{}, false, errors.New("memory_embedding_assignment_conflict")
			}
		} else {
			return providerAssignment{}, false, errors.New("memory_embedding_intent_not_found")
		}
	}
	if providerEndpointID != "" {
		assignment, err := a.Provider.embeddingAssignmentByID(ctx, providerEndpointID, modelID)
		return assignment, false, err
	}
	assignment, err := a.Provider.assignment(ctx, "embedding")
	if err != nil {
		return providerAssignment{}, false, err
	}
	if strings.TrimSpace(intentID) != "" {
		command, err := a.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=payload || jsonb_build_object('provider_endpoint_id',$2::text,'model_id',$3::text) WHERE intent_id=$1 AND intent_type='memory.embedding' AND COALESCE(payload->>'provider_endpoint_id','')='' AND COALESCE(payload->>'model_id','')=''`, intentID, assignment.EndpointID, assignment.ModelID)
		if err != nil {
			return providerAssignment{}, false, err
		}
		if command.RowsAffected() == 0 {
			return a.resolveMemoryEmbeddingAssignmentState(ctx, intentID, "", "")
		}
	}
	return assignment, false, nil
}

func (a *App) prepareMemoryEmbeddingAttempt(ctx context.Context, memoryID string, revision int, embeddingID string, assignment providerAssignment, begin bool) (bool, error) {
	ready := false
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var liveRevision int
		var liveStatus string
		if err := tx.QueryRow(ctx, `SELECT revision,status FROM public.memories WHERE id=$1 FOR UPDATE`, memoryID).Scan(&liveRevision, &liveStatus); err != nil {
			return err
		}
		if liveStatus != "active" || liveRevision != revision {
			return errors.New("memory_embedding_stale")
		}
		var existingID, existingStatus, existingEndpointID string
		err := tx.QueryRow(ctx, `SELECT id,status,COALESCE(provider_endpoint_id,'') FROM public.memory_embeddings WHERE memory_id=$1 AND memory_revision=$2 AND model_id=$3 FOR UPDATE`, memoryID, revision, assignment.ModelID).Scan(&existingID, &existingStatus, &existingEndpointID)
		if err == nil {
			if existingEndpointID != assignment.EndpointID {
				return errors.New("memory_embedding_assignment_conflict")
			}
			switch existingStatus {
			case "ready":
				ready = true
				return nil
			case "stale":
				return errors.New("memory_embedding_stale")
			case "pending", "failed":
				if !begin {
					return nil
				}
				updated, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET status='pending',dimensions=0,embedding='[]',embedding_vector=NULL,error_code=NULL,embedded_at=NULL WHERE id=$1 AND provider_endpoint_id=$2 AND status IN ('pending','failed')`, existingID, assignment.EndpointID)
				if err != nil {
					return err
				}
				if updated.RowsAffected() != 1 {
					return errors.New("memory_embedding_tuple_conflict")
				}
				return nil
			default:
				return errors.New("memory_embedding_status_invalid")
			}
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !begin {
			return nil
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.memory_embeddings(id,memory_id,memory_revision,provider_endpoint_id,model_id,dimensions,embedding,embedding_vector,status) VALUES($1,$2,$3,$4,$5,0,'[]',NULL,'pending')`, embeddingID, memoryID, revision, assignment.EndpointID, assignment.ModelID)
		return err
	})
	return ready, err
}

func (a *App) settleMemoryEmbeddingFailure(ctx context.Context, memoryID string, revision int, embeddingID string, assignment providerAssignment, code string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var liveOwner string
		var liveRevision int
		var liveStatus string
		if err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,revision,status FROM public.memories WHERE id=$1 FOR UPDATE`, memoryID).Scan(&liveOwner, &liveRevision, &liveStatus); err != nil {
			return err
		}
		if liveStatus != "active" || liveRevision != revision {
			_, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET status='stale',error_code=COALESCE(error_code,'memory_revision_changed') WHERE id=$1 AND status IN ('pending','failed')`, embeddingID)
			return err
		}
		updated, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET status='failed',dimensions=0,embedding='[]',embedding_vector=NULL,error_code=$2,embedded_at=NULL WHERE id=$1 AND memory_id=$3 AND memory_revision=$4 AND provider_endpoint_id=$5 AND model_id=$6 AND status IN ('pending','failed')`, embeddingID, code, memoryID, revision, assignment.EndpointID, assignment.ModelID)
		if err != nil {
			return err
		}
		if updated.RowsAffected() != 1 {
			return errors.New("memory_embedding_failure_settlement_conflict")
		}
		return appendOutboxTx(ctx, tx, "memory.embedding.failed", "memory", memoryID, liveOwner, memoryID, "memory:"+memoryID, "memory-embedding-failed:"+memoryID+":"+fmt.Sprint(revision)+":"+assignment.ModelID, map[string]any{"memory_id": memoryID, "revision": revision, "model_id": assignment.ModelID, "status": "failed", "error_code": code})
	})
}

func (a *App) settleMemoryEmbeddingReady(ctx context.Context, memoryID string, revision int, embeddingID string, assignment providerAssignment, vector []float64, vectorLiteral string) (bool, error) {
	settled := false
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var liveOwner, liveStatus, provenanceStatus string
		var liveRevision int
		if err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,revision,status,provenance_status FROM public.memories WHERE id=$1 FOR UPDATE`, memoryID).Scan(&liveOwner, &liveRevision, &liveStatus, &provenanceStatus); err != nil {
			return err
		}
		supported, err := memorySourcesStillValid(ctx, tx, memoryID, liveRevision, provenanceStatus)
		if err != nil {
			return err
		}
		if liveStatus != "active" || liveRevision != revision || !supported {
			_, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET status='stale',error_code=COALESCE(error_code,'memory_revision_changed') WHERE id=$1 AND status IN ('pending','failed')`, embeddingID)
			return err
		}
		updated, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET dimensions=$2,embedding=$3,embedding_vector=$4::vector,status='ready',error_code=NULL,embedded_at=now() WHERE id=$1 AND memory_id=$5 AND memory_revision=$6 AND provider_endpoint_id=$7 AND model_id=$8 AND status IN ('pending','failed')`, embeddingID, len(vector), jsonBytes(vector), vectorLiteral, memoryID, revision, assignment.EndpointID, assignment.ModelID)
		if err != nil {
			return err
		}
		if updated.RowsAffected() != 1 {
			var status string
			if err := tx.QueryRow(ctx, `SELECT status FROM public.memory_embeddings WHERE id=$1 AND memory_id=$2 AND memory_revision=$3 AND provider_endpoint_id=$4 AND model_id=$5`, embeddingID, memoryID, revision, assignment.EndpointID, assignment.ModelID).Scan(&status); err == nil && status == "ready" {
				settled = true
				return nil
			}
			return errors.New("memory_embedding_ready_settlement_conflict")
		}
		settled = true
		return appendOutboxTx(ctx, tx, "memory.embedding.ready", "memory", memoryID, liveOwner, memoryID, "memory:"+memoryID, "memory-embedding-ready:"+memoryID+":"+fmt.Sprint(revision)+":"+assignment.ModelID, map[string]any{"memory_id": memoryID, "revision": revision, "model_id": assignment.ModelID, "dimensions": len(vector), "status": "ready"})
	})
	return settled, err
}
