package core

import (
	"context"
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
	var content, status string
	var revision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT content,revision,status FROM public.memories WHERE id=$1`, memoryID).Scan(&content, &revision, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]any{"memory_id": memoryID, "status": "not_found", "requested_revision": requestedRevision}, nil
		}
		return nil, err
	}
	if status != "active" || revision != requestedRevision {
		return map[string]any{"memory_id": memoryID, "status": "stale", "revision": revision, "requested_revision": requestedRevision}, nil
	}
	assignment, err := a.resolveMemoryEmbeddingAssignment(ctx, intentID, providerEndpointID, modelID)
	if err != nil {
		return nil, err
	}
	embeddingID := "embedding_" + stableDigest(memoryID+":"+fmt.Sprint(requestedRevision)+":"+assignment.ModelID)
	ready, err := a.prepareMemoryEmbeddingAttempt(ctx, memoryID, requestedRevision, embeddingID, assignment)
	if err != nil {
		if err.Error() == "memory_embedding_stale" {
			return map[string]any{"memory_id": memoryID, "status": "stale", "revision": revision, "requested_revision": requestedRevision}, nil
		}
		return nil, err
	}
	if ready {
		return map[string]any{"memory_id": memoryID, "status": "ready", "revision": requestedRevision, "model_id": assignment.ModelID, "replayed": true}, nil
	}
	model, vector, providerErr := a.Provider.embedWithAssignment(ctx, content, assignment)
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
	providerEndpointID = strings.TrimSpace(providerEndpointID)
	modelID = strings.TrimSpace(modelID)
	if (providerEndpointID == "") != (modelID == "") {
		return providerAssignment{}, errors.New("memory_embedding_assignment_incomplete")
	}
	if strings.TrimSpace(intentID) != "" {
		var payload []byte
		if err := a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='memory.embedding'`, intentID).Scan(&payload); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return providerAssignment{}, err
		} else if err == nil {
			value := decodeObject(payload)
			storedEndpointID := stringValue(value["provider_endpoint_id"])
			storedModelID := stringValue(value["model_id"])
			if (storedEndpointID == "") != (storedModelID == "") {
				return providerAssignment{}, errors.New("memory_embedding_assignment_incomplete")
			}
			if storedEndpointID != "" {
				if (providerEndpointID != "" && providerEndpointID != storedEndpointID) || (modelID != "" && modelID != storedModelID) {
					return providerAssignment{}, errors.New("memory_embedding_assignment_conflict")
				}
				providerEndpointID, modelID = storedEndpointID, storedModelID
			} else if providerEndpointID != "" || modelID != "" {
				return providerAssignment{}, errors.New("memory_embedding_assignment_conflict")
			}
		} else {
			return providerAssignment{}, errors.New("memory_embedding_intent_not_found")
		}
	}
	if providerEndpointID != "" {
		return a.Provider.embeddingAssignmentByID(ctx, providerEndpointID, modelID)
	}
	assignment, err := a.Provider.assignment(ctx, "embedding")
	if err != nil {
		return providerAssignment{}, err
	}
	if strings.TrimSpace(intentID) != "" {
		command, err := a.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=payload || jsonb_build_object('provider_endpoint_id',$2::text,'model_id',$3::text) WHERE intent_id=$1 AND intent_type='memory.embedding' AND COALESCE(payload->>'provider_endpoint_id','')='' AND COALESCE(payload->>'model_id','')=''`, intentID, assignment.EndpointID, assignment.ModelID)
		if err != nil {
			return providerAssignment{}, err
		}
		if command.RowsAffected() == 0 {
			return a.resolveMemoryEmbeddingAssignment(ctx, intentID, "", "")
		}
	}
	return assignment, nil
}

func (a *App) prepareMemoryEmbeddingAttempt(ctx context.Context, memoryID string, revision int, embeddingID string, assignment providerAssignment) (bool, error) {
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
		var liveOwner, liveStatus string
		var liveRevision int
		if err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,revision,status FROM public.memories WHERE id=$1 FOR UPDATE`, memoryID).Scan(&liveOwner, &liveRevision, &liveStatus); err != nil {
			return err
		}
		if liveStatus != "active" || liveRevision != revision {
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
