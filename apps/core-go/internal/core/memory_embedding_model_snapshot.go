package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const memoryEmbeddingModelSnapshotVersion = "v1"

var errMemoryEmbeddingModelSnapshotInvalid = errors.New("memory_embedding_model_snapshot_invalid")

type memoryEmbeddingModelSnapshot struct {
	Version                       string             `json:"version"`
	CredentialPurposePresent      bool               `json:"credential_purpose_present"`
	CredentialRevisionWasPositive bool               `json:"credential_revision_was_positive"`
	Assignment                    providerAssignment `json:"assignment"`
}

func newMemoryEmbeddingModelSnapshot(assignment providerAssignment) memoryEmbeddingModelSnapshot {
	assignment.Secret = ""
	return memoryEmbeddingModelSnapshot{Version: memoryEmbeddingModelSnapshotVersion, CredentialPurposePresent: strings.TrimSpace(assignment.SecretPurpose) != "", CredentialRevisionWasPositive: assignment.CredentialRevision > 0, Assignment: assignment}
}

func readMemoryEmbeddingModelSnapshot(raw []byte) (memoryEmbeddingModelSnapshot, error) {
	var snapshot memoryEmbeddingModelSnapshot
	var fields struct {
		Assignment                    map[string]json.RawMessage `json:"assignment"`
		CredentialPurposePresent      *bool                      `json:"credential_purpose_present"`
		CredentialRevisionWasPositive *bool                      `json:"credential_revision_was_positive"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil || fields.Assignment == nil || fields.CredentialPurposePresent == nil || fields.CredentialRevisionWasPositive == nil {
		return memoryEmbeddingModelSnapshot{}, errMemoryEmbeddingModelSnapshotInvalid
	}
	var purpose *string
	if value, ok := fields.Assignment["SecretPurpose"]; !ok || json.Unmarshal(value, &purpose) != nil || purpose == nil {
		return memoryEmbeddingModelSnapshot{}, errMemoryEmbeddingModelSnapshotInvalid
	}
	var credentialRevision *int64
	if value, ok := fields.Assignment["CredentialRevision"]; !ok || json.Unmarshal(value, &credentialRevision) != nil || credentialRevision == nil || *credentialRevision < 0 {
		return memoryEmbeddingModelSnapshot{}, errMemoryEmbeddingModelSnapshotInvalid
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil || snapshot.Version != memoryEmbeddingModelSnapshotVersion || snapshot.Assignment.Role != "embedding" || strings.TrimSpace(snapshot.Assignment.EndpointID) == "" || strings.TrimSpace(snapshot.Assignment.ModelID) == "" || strings.TrimSpace(snapshot.Assignment.BaseURL) == "" || snapshot.Assignment.EndpointRevision <= 0 || snapshot.Assignment.Timeout <= 0 || snapshot.Assignment.Secret != "" || snapshot.CredentialPurposePresent != (strings.TrimSpace(snapshot.Assignment.SecretPurpose) != "") || snapshot.CredentialRevisionWasPositive != (snapshot.Assignment.CredentialRevision > 0) {
		return memoryEmbeddingModelSnapshot{}, errMemoryEmbeddingModelSnapshotInvalid
	}
	return snapshot, nil
}

// The Memory mutation and its embedding intent share a transaction, so this
// reads the binding using that same transaction and never reads plaintext keys.
type embeddingSnapshotQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func embeddingAssignmentForEnqueue(ctx context.Context, queryer embeddingSnapshotQueryer) (providerAssignment, error) {
	var endpointID, baseURL, purpose, modelID string
	var timeoutSeconds int
	var bindingRevision, endpointRevision, credentialRevision int64
	err := queryer.QueryRow(ctx, `SELECT e.id,e.base_url,COALESCE(e.secret_purpose,''),r.model_id,r.timeout_seconds,COALESCE((SELECT revision FROM public.configuration_revisions WHERE resource_key='binding:'||r.role),0),COALESCE((SELECT revision FROM public.configuration_revisions WHERE resource_key='endpoint:'||e.id),0),COALESCE((SELECT revision FROM public.configuration_revisions WHERE resource_key='secret:'||e.secret_purpose),0) FROM public.model_roles r JOIN public.provider_endpoints e ON e.id=r.provider_endpoint_id WHERE r.role='embedding' AND e.capability_status<>'failed' LIMIT 1`).Scan(&endpointID, &baseURL, &purpose, &modelID, &timeoutSeconds, &bindingRevision, &endpointRevision, &credentialRevision)
	if err != nil {
		return providerAssignment{}, err
	}
	if strings.TrimSpace(endpointID) == "" || strings.TrimSpace(baseURL) == "" || strings.TrimSpace(modelID) == "" {
		return providerAssignment{}, pgx.ErrNoRows
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}
	return providerAssignment{Role: "embedding", SourceBindingRole: "embedding", BindingRevision: bindingRevision, EndpointRevision: endpointRevision, CredentialRevision: credentialRevision, EndpointID: endpointID, SecretPurpose: purpose, BaseURL: strings.TrimRight(baseURL, "/"), ModelID: modelID, Timeout: time.Duration(timeoutSeconds) * time.Second}, nil
}

func (a *App) restoreMemoryEmbeddingCredential(ctx context.Context, assignment *providerAssignment) error {
	if assignment.SecretPurpose == "" {
		// An explicitly empty v1 reference means an unauthenticated endpoint,
		// not permission to adopt a purpose configured later.
		assignment.Secret = ""
		return nil
	}
	return a.restoreAssignmentCredential(ctx, assignment)
}

// A new intent has an independent contract marker and an initially empty or
// selected row. Missing or corrupt rows never fall back to current settings.
func (a *App) resolveFrozenMemoryEmbedding(ctx context.Context, intentID, endpointID, modelID string) (providerAssignment, bool, error) {
	for attempt := 0; attempt < 2; attempt++ {
		var payload, raw []byte
		var contract string
		var selected bool
		if err := a.DB.Pool().QueryRow(ctx, `SELECT w.payload,w.embedding_snapshot_contract,s.snapshot,COALESCE(s.selected,false) FROM public.platform_workflow_intents w LEFT JOIN public.memory_embedding_model_snapshots s ON s.intent_id=w.intent_id WHERE w.intent_id=$1 AND w.intent_type='memory.embedding'`, intentID).Scan(&payload, &contract, &raw, &selected); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return providerAssignment{}, true, errors.New("memory_embedding_intent_not_found")
			}
			return providerAssignment{}, true, err
		}
		if contract == "" && raw == nil {
			return providerAssignment{}, false, nil
		}
		if contract != memoryEmbeddingModelSnapshotVersion || raw == nil {
			return providerAssignment{}, true, errMemoryEmbeddingModelSnapshotInvalid
		}
		value := decodeObject(payload)
		storedEndpoint, storedModel := stringValue(value["provider_endpoint_id"]), stringValue(value["model_id"])
		if (storedEndpoint == "") != (storedModel == "") {
			return providerAssignment{}, true, errors.New("memory_embedding_assignment_incomplete")
		}
		if (endpointID != "" && endpointID != storedEndpoint) || (modelID != "" && modelID != storedModel) {
			return providerAssignment{}, true, errors.New("memory_embedding_assignment_conflict")
		}
		var stored map[string]any
		if err := json.Unmarshal(raw, &stored); err != nil || stored == nil || selected != (len(stored) > 0) {
			return providerAssignment{}, true, errMemoryEmbeddingModelSnapshotInvalid
		}
		if selected {
			snapshot, err := readMemoryEmbeddingModelSnapshot(raw)
			if err != nil || snapshot.Assignment.EndpointID != storedEndpoint || snapshot.Assignment.ModelID != storedModel {
				return providerAssignment{}, true, errMemoryEmbeddingModelSnapshotInvalid
			}
			return snapshot.Assignment, true, nil
		}
		if storedEndpoint != "" || storedModel != "" {
			return providerAssignment{}, true, errMemoryEmbeddingModelSnapshotInvalid
		}
		assignment, frozen := providerAssignmentSnapshot(ctx, "embedding")
		var err error
		if !frozen {
			assignment, err = embeddingAssignmentForEnqueue(ctx, a.DB.Pool())
		}
		if err != nil {
			return providerAssignment{}, true, err
		}
		snapshot := newMemoryEmbeddingModelSnapshot(assignment)
		if _, err := readMemoryEmbeddingModelSnapshot(jsonBytes(snapshot)); err != nil {
			return providerAssignment{}, true, err
		}
		saved := false
		err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			var livePayload []byte
			var liveContract string
			if err := tx.QueryRow(ctx, `SELECT payload,embedding_snapshot_contract FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='memory.embedding' FOR UPDATE`, intentID).Scan(&livePayload, &liveContract); err != nil {
				return err
			}
			live := decodeObject(livePayload)
			if liveContract != memoryEmbeddingModelSnapshotVersion || stringValue(live["provider_endpoint_id"]) != "" || stringValue(live["model_id"]) != "" {
				return nil
			}
			tag, err := tx.Exec(ctx, `UPDATE public.memory_embedding_model_snapshots SET snapshot=$2,selected=true WHERE intent_id=$1 AND NOT selected AND snapshot='{}'::jsonb`, intentID, jsonBytes(snapshot))
			if err != nil || tag.RowsAffected() != 1 {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=payload || jsonb_build_object('provider_endpoint_id',$2::text,'model_id',$3::text) WHERE intent_id=$1`, intentID, assignment.EndpointID, assignment.ModelID); err != nil {
				return err
			}
			saved = true
			return nil
		})
		if err != nil {
			return providerAssignment{}, true, err
		}
		if saved {
			return snapshot.Assignment, true, nil
		}
	}
	return providerAssignment{}, true, fmt.Errorf("memory_embedding_model_snapshot_not_saved: %w", ErrConflict)
}
