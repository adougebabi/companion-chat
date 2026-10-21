package browser

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode"
)

type surfaceRoute struct {
	method  string
	pattern string
}

var surfaceRoutes = []surfaceRoute{
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/places/:placeId/defaults"}, {http.MethodPost, "/api/fluctlights/:fluctlightId/world/places/:placeId/defaults"}, {http.MethodGet, "/api/fluctlights/:fluctlightId/world/places/:placeId/defaults/history"},
	{http.MethodGet, "/api/ops/backups/:backupId"}, {http.MethodPost, "/api/ops/backups/:backupId/retry"}, {http.MethodPost, "/api/ops/backups/:backupId/cancel"},
	{http.MethodPost, "/api/studio/packs/:packId/groups/:index/adopt"},
	{http.MethodGet, "/api/data/revisions"},
	{http.MethodGet, "/api/studio/packs/:packId"},
	{http.MethodPost, "/api/diagnostic-records/media-prompts/:recordId/retry"},
	{http.MethodPost, "/api/workflow-controls/:workflowId/commands/:commandId/retry"},
	{http.MethodGet, "/api/workflow-controls/:workflowId"}, {http.MethodGet, "/api/workflow-controls/:workflowId/history"},
	{http.MethodPost, "/api/workflow-controls/:workflowId/pause"}, {http.MethodPost, "/api/workflow-controls/:workflowId/resume"}, {http.MethodPost, "/api/workflow-controls/:workflowId/cancel"}, {http.MethodPost, "/api/workflow-controls/:workflowId/restart"}, {http.MethodPost, "/api/workflow-controls/:workflowId/reset"},
	{http.MethodGet, "/api/diagnostic-records/:kind"}, {http.MethodGet, "/api/diagnostic-records/:kind/:recordRef"}, {http.MethodPost, "/api/diagnostic-records/clear"},
	{http.MethodPost, "/api/configuration-commands/recover"},
	{http.MethodPost, "/api/configuration-commands/cancel-unaccepted"},
	{http.MethodGet, "/api/configuration-commands/by-key"},
	{http.MethodGet, "/api/configuration-catalog"}, {http.MethodPost, "/api/settings/media/test-connection"},
	{http.MethodGet, "/api/moment-suggestions"},
	{http.MethodGet, "/api/moment-feed/:momentId/npc-interactions"}, {http.MethodGet, "/api/moment-feed/:momentId/npc-interactions/:interactionId"}, {http.MethodPost, "/api/moment-feed/:momentId/npc-interactions/:interactionId/retry"},
	{http.MethodGet, "/api/moment-feed"}, {http.MethodGet, "/api/moment-feed/:momentId"}, {http.MethodGet, "/api/moment-feed/:momentId/comments"},
	{http.MethodPost, "/api/moment-feed/:momentId/comments"}, {http.MethodPost, "/api/moment-feed/:momentId/reaction"}, {http.MethodPost, "/api/moment-feed/:momentId/hide"}, {http.MethodPost, "/api/moment-feed/:momentId/restore"},
	{http.MethodGet, "/api/media-library"},
	{http.MethodDelete, "/api/media-assets/:assetId"}, {http.MethodGet, "/api/media-assets/:assetId/references"}, {http.MethodGet, "/api/media-deletions/:deletionId"}, {http.MethodPost, "/api/media-deletions/:deletionId/retry"},
	{http.MethodGet, "/api/studio/materials/:materialId"}, {http.MethodGet, "/api/studio/materials/:materialId/history"}, {http.MethodPost, "/api/studio/materials/:materialId/release"}, {http.MethodPost, "/api/studio/materials/:materialId/retain"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/media-generations"}, {http.MethodGet, "/api/fluctlights/:fluctlightId/media-generations"}, {http.MethodGet, "/api/fluctlights/:fluctlightId/media-generations/:requestId"},
	{http.MethodGet, "/api/media-reference-grants"}, {http.MethodPost, "/api/media-reference-grants"}, {http.MethodGet, "/api/media-reference-grants/:grantId"}, {http.MethodPost, "/api/media-reference-grants/:grantId/revoke"}, {http.MethodGet, "/api/media-reference-grants/:grantId/history"}, {http.MethodGet, "/api/media-reference-grants/:grantId/uses"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/display-avatar"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/display-avatar"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/display-avatar/history"},
	{http.MethodGet, "/api/media-assets/:assetId"},
	{http.MethodPut, "/api/media-assets/:assetId"},
	{http.MethodGet, "/api/media-uploads/by-key"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/items"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/items/:itemId"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/world/items/:itemId"},
	{http.MethodGet, "/api/studio/jobs"},
	{http.MethodPost, "/api/studio/jobs"},
	{http.MethodGet, "/api/studio/jobs/:jobId"},
	{http.MethodPost, "/api/studio/jobs/:jobId/retry"},
	{http.MethodPost, "/api/studio/jobs/:jobId/discard"},
	{http.MethodPost, "/api/studio/jobs/:jobId/save-material"},
	{http.MethodGet, "/api/studio/materials"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/npcs/:npcId/opportunities/:opportunityId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/npcs/:npcId/encounters"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/npcs/:npcId/presences"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/npcs/:npcId/opportunities"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/npcs/:npcId/opportunities/:opportunityId/retry"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/promises"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/promises/:promiseId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/promises/:promiseId/history"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/promises/:promiseId/reschedule"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/promises/:promiseId/cancel"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/promises/:promiseId/retry"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/inventory/preparation"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/inventory/preparation"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/inventory/tags"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/inventory/tags"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/inventory/wearing-history"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/inventory/outfits"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/inventory/outfits/:outfitId"},
	{http.MethodPatch, "/api/fluctlights/:fluctlightId/inventory/outfits/:outfitId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/inventory/outfits/:outfitId/revisions"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/inventory/outfits/:outfitId"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/inventory/requests"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/inventory/outfits"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/inventory"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/inventory/items"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/inventory/items/:itemId"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/inventory/items/:itemId"},
	{http.MethodDelete, "/api/fluctlights/:fluctlightId/inventory/items/:itemId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/inventory/items/:itemId/history"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/visual-sessions/:sessionId/review"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/visual-identity/timeline"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/visual-brief"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/visual-brief"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/visual-brief/history"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/visual-identity"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/visual-identity/generate"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/foundation-revisions"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/foundation-revisions/:revisionId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/foundation-revisions/:revisionId/history"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/capability-requests"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/capability-requests/:requestId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/capability-requests/:requestId/history"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/capability-requests/:requestId/review"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/media"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/sidebar"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/nearby"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/diary"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/shared-experiences"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/self-claims"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/self-claims/:claimId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/self-claims/:claimId/revisions"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/self-claims/:claimId/reviews"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/self-claims/:claimId/review"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/self-claims/:claimId/forget"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/self-claims/:claimId/restore"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/diary/:entryId"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/diary/generate"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/diary/:entryId/annotation"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/diary/:entryId/replies"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/diary/:entryId/replies"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/diary"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/diary/:entryId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/world"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/catalog"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/catalog/:worldId"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/world/catalog"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/world/catalog/:worldId"},
	{http.MethodDelete, "/api/fluctlights/:fluctlightId/world/catalog/:worldId"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/world/active"},
	{http.MethodDelete, "/api/fluctlights/:fluctlightId/world/places/:placeId"},
	{http.MethodDelete, "/api/fluctlights/:fluctlightId/npcs/:npcId"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/world/places"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/world/places/:placeId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/npcs"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/npcs/:npcId"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/npcs/:npcId"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/npcs/:npcId/encounters"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/events"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/events/:eventId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/templates"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/world/templates/:templateId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/templates/:templateId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/templates/:templateId/runs"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/world/templates/:templateId/runs"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/templates/:templateId/runs/:runId"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/world/templates/:templateId/runs/:runId/retry"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/places/:placeId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/autonomy-policy"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/autonomy-policy"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/desires"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/owner-requests"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/promises"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/moment-suggestions"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/persona-switch"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/visual-identity"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/wardrobe/wear"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/wardrobe/items/:itemId/use"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/wardrobe/items/:itemId/photos"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/schedules/:scheduleId/items"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/schedules/:scheduleId/items/:itemId"},
	{http.MethodDelete, "/api/fluctlights/:fluctlightId/schedules/:scheduleId/items/:itemId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/reflections/:reflectionId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/reflections"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/reflections/:reflectionId/feedback"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/reflections/:reflectionId/feedback"},
	{http.MethodGet, "/api/notifications"},
	{http.MethodGet, "/api/notifications/unread-count"},
	{http.MethodPost, "/api/notifications/read-all"},
	{http.MethodPost, "/api/notifications/:notificationId/read"},
	{http.MethodGet, "/api/pending"},
	{http.MethodGet, "/api/pending/count"},
	{http.MethodGet, "/api/studio/resources"},
	{http.MethodPost, "/api/studio/resources"},
	{http.MethodGet, "/api/studio/resources/:resourceId"},
	{http.MethodGet, "/api/studio/resources/:resourceId/versions/:version"},
	{http.MethodGet, "/api/studio/resources/:resourceId/versions"},
	{http.MethodPost, "/api/studio/resources/:resourceId/copy"},
	{http.MethodPost, "/api/studio/resources/:resourceId/restore"},
	{http.MethodDelete, "/api/studio/resources/:resourceId"},
	{http.MethodPut, "/api/studio/resources/:resourceId"},
	{http.MethodPost, "/api/studio/resources/:resourceId/publish"},
	{http.MethodPost, "/api/studio/resources/:resourceId/apply-preview"},
	{http.MethodPost, "/api/studio/resources/:resourceId/apply"},
	{http.MethodGet, "/api/studio/previews"},
	{http.MethodPost, "/api/studio/previews"},
	{http.MethodPost, "/api/studio/packs"},
	{http.MethodPost, "/api/studio/packs/validate"},
	{http.MethodGet, "/api/studio/packs/export"},
	{http.MethodGet, "/api/ops/backups"},
	{http.MethodPost, "/api/ops/backups"},
	{http.MethodPost, "/api/ops/backups/:backupId/restore"},
	{http.MethodGet, "/api/diagnostics/harm-intercepts"},
	{http.MethodGet, "/api/diagnostics/tasks/:taskId"},
	{http.MethodGet, "/api/settings/agents"},
	{http.MethodPost, "/api/memories/:memoryId/suggestions"},
	{http.MethodGet, "/api/owner-requests/:requestId"},
	{http.MethodPost, "/api/moments"},
	{http.MethodPost, "/api/media"},
	{http.MethodDelete, "/api/actor-groups/:groupId"},
}

func (s *Server) routeSurface(response http.ResponseWriter, request *http.Request) bool {
	for _, route := range surfaceRoutes {
		if route.method == request.Method && surfacePathMatches(request.URL.Path, route.pattern) {
			s.dispatchSurface(response, request)
			return true
		}
	}
	return false
}

func (s *Server) dispatchSurface(response http.ResponseWriter, request *http.Request) {
	session, ok := s.requireSession(response, request)
	if !ok {
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead && !mutationGuard(response, request, s.trustedOrigin) {
		return
	}
	if request.Method == http.MethodPost && request.URL.Path == "/api/media" {
		s.uploadOwnerMedia(response, request, session)
		return
	}
	var body map[string]any
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		parsed, valid := surfaceBody(response, request)
		if !valid {
			return
		}
		if (strings.HasSuffix(request.URL.Path, "/display-avatar") || strings.HasSuffix(request.URL.Path, "/visual-identity")) && !validDisplayAvatar(parsed) {
			writeError(response, 400, "invalid_request", "Invalid display avatar selection")
			return
		}
		if (strings.HasSuffix(request.URL.Path, "/media-generations") || strings.HasSuffix(request.URL.Path, "/photos")) && !validOwnerMediaRequest(request.URL.Path, parsed) {
			writeError(response, 400, "invalid_request", "Invalid Owner image request")
			return
		}
		if strings.HasPrefix(request.URL.Path, "/api/media-reference-grants") && !validMediaReferenceGrant(request.URL.Path, parsed) {
			writeError(response, 400, "invalid_request", "Invalid media reference authorization")
			return
		}
		if request.Method == http.MethodPut && strings.HasPrefix(request.URL.Path, "/api/media-assets/") && !validMediaMetadata(parsed) {
			writeError(response, 400, "invalid_request", "Invalid media metadata command")
			return
		}
		if request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/api/media-assets/") && !validMediaDelete(parsed) {
			writeError(response, 400, "invalid_request", "Invalid media deletion confirmation")
			return
		}
		if strings.HasPrefix(request.URL.Path, "/api/media-deletions/") && !validMediaDeletionRetry(parsed) {
			writeError(response, 400, "invalid_request", "Invalid media deletion retry")
			return
		}
		if strings.HasSuffix(request.URL.Path, "/persona-switch") && !validPersonaSwitchBody(parsed) {
			writeError(response, http.StatusBadRequest, "invalid_request", "Request validation failed")
			return
		}
		if strings.Contains(request.URL.Path, "/reflections/") && strings.HasSuffix(request.URL.Path, "/feedback") && (!onlyKeys(parsed, "text", "stance", "idempotencyKey") || !validateString(parsed["text"], 1, 2000) || !validateString(parsed["idempotencyKey"], 1, 256) || (parsed["stance"] != nil && parsed["stance"] != "agree" && parsed["stance"] != "disagree")) {
			writeError(response, 400, "invalid_request", "Invalid reflection feedback")
			return
		}
		if request.URL.Path == "/api/notifications/read-all" && (!onlyKeys(parsed, "before", "idempotencyKey") || !validateString(parsed["before"], 1, 64) || !validateString(parsed["idempotencyKey"], 1, 256)) {
			writeError(response, 400, "invalid_request", "Invalid notification snapshot")
			return
		}
		if strings.HasSuffix(request.URL.Path, "/autonomy-policy") && !validAutonomyPolicyBody(parsed) {
			writeError(response, 400, "invalid_request", "Invalid autonomy policy")
			return
		}
		if strings.Contains(request.URL.Path, "/opportunities/") && strings.HasSuffix(request.URL.Path, "/retry") && (!onlyKeys(parsed, "expectedRevision", "reason", "idempotencyKey") || !validateInteger(parsed["expectedRevision"], 1) || !validateString(parsed["reason"], 1, 500) || !validateString(parsed["idempotencyKey"], 1, 256)) {
			writeError(response, 400, "invalid_request", "Invalid NPC opportunity retry")
			return
		}
		if !validMomentFeedBody(request.URL.Path, parsed) {
			writeError(response, 400, "invalid_request", "Invalid Moment command")
			return
		}
		if !validPromiseSurfaceBody(request.URL.Path, parsed) {
			writeError(response, 400, "invalid_request", "Invalid promise command")
			return
		}
		if !validWorldSurfaceBody(request.Method, request.URL.Path, parsed) || !validLifeSurfaceBody(request.Method, request.URL.Path, parsed) {
			writeError(response, http.StatusBadRequest, "invalid_request", "Request validation failed")
			return
		}
		if !validDiarySurfaceBody(request.Method, request.URL.Path, parsed) {
			writeError(response, 400, "invalid_request", "Invalid diary request")
			return
		}
		if strings.Contains(request.URL.Path, "/self-claims/") && !validSelfCommandBody(request.URL.Path, parsed) {
			writeError(response, 400, "invalid_request", "Invalid self governance command")
			return
		}
		if strings.Contains(request.URL.Path, "/capability-requests/") && !validateCapabilityRequestReview(parsed) {
			writeError(response, 400, "invalid_request", "Invalid capability review")
			return
		}
		if strings.HasSuffix(request.URL.Path, "/visual-brief") && !validVisualBrief(parsed) {
			writeError(response, 400, "invalid_request", "Invalid visual brief")
			return
		}
		if strings.HasSuffix(request.URL.Path, "/visual-identity/generate") && !validVisualGeneration(parsed) {
			writeError(response, 400, "invalid_request", "Invalid visual generation request")
			return
		}
		if strings.Contains(request.URL.Path, "/visual-sessions/") && !validManualVisualReview(parsed) {
			writeError(response, 400, "invalid_request", "Invalid manual visual review")
			return
		}
		if strings.Contains(request.URL.Path, "/inventory/items") && !validInventoryCommand(request.Method, parsed) {
			writeError(response, 400, "invalid_request", "Invalid inventory command")
			return
		}
		if strings.HasSuffix(request.URL.Path, "/inventory/requests") && !validInventoryAction(parsed) {
			writeError(response, 400, "invalid_request", "Invalid inventory action request")
			return
		}
		if strings.HasSuffix(request.URL.Path, "/inventory/preparation") && (!onlyKeys(parsed, "expectedRevision", "reason", "idempotencyKey") || !validateRevisionReason(parsed) || !validateString(parsed["idempotencyKey"], 1, 256)) {
			writeError(response, 400, "invalid_request", "Invalid wardrobe preparation")
			return
		}
		if strings.HasSuffix(request.URL.Path, "/inventory/tags") && (!onlyKeys(parsed, "tags", "renames", "expectedRevision", "reason", "idempotencyKey") || !validateRevisionReason(parsed) || !isStringArray(parsed["tags"]) || !validateString(parsed["idempotencyKey"], 1, 256)) {
			writeError(response, 400, "invalid_request", "Invalid wardrobe tags")
			return
		}
		if strings.Contains(request.URL.Path, "/inventory/outfits") && !validInventoryOutfitSave(request.Method, parsed) {
			writeError(response, 400, "invalid_request", "Invalid outfit definition")
			return
		}
		if !validStudioJobBody(request.URL.Path, parsed) {
			writeError(response, 400, "invalid_request", "Invalid Studio job command")
			return
		}
		if strings.Contains(request.URL.Path, "/world/items/") && !validWorldItemConfig(parsed) {
			writeError(response, 400, "invalid_request", "Invalid world item configuration")
			return
		}
		if strings.HasPrefix(request.URL.Path, "/api/studio/resources/") && (request.Method == http.MethodDelete || strings.HasSuffix(request.URL.Path, "/copy") || strings.HasSuffix(request.URL.Path, "/restore")) && !validStudioCatalogCommand(request.URL.Path, parsed) {
			writeError(response, 400, "invalid_request", "Invalid resource lifecycle command")
			return
		}
		body = surfaceCommandBody(request.URL.Path, parsed)
	}
	endpoint := "/internal" + strings.TrimPrefix(request.URL.Path, "/api")
	if request.URL.RawQuery != "" {
		endpoint += "?" + request.URL.RawQuery
	}
	result, err := s.backend.DoJSON(request.Context(), request.Method, endpoint, session, body)
	if err != nil {
		s.writeSurfaceError(response, err)
		return
	}
	if result == nil {
		result = map[string]any{}
	}
	if surfaceNotProvided(request.URL.Path, result) {
		writeError(response, http.StatusNotFound, "not_provided", "Notifications are not connected")
		return
	}
	replay, _ := result["_replay"].(bool)
	status := surfaceStatus(result["_status"])
	delete(result, "_status")
	delete(result, "_replay")
	delete(result, "__error")
	if replay {
		response.Header().Set("Idempotent-Replayed", "true")
	}
	if status == http.StatusNoContent {
		response.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(response, status, result)
}

func (s *Server) writeSurfaceError(response http.ResponseWriter, err error) {
	var coreErr *CoreError
	if errors.As(err, &coreErr) && coreErr.Status >= 400 {
		if coreErr.Replay {
			response.Header().Set("Idempotent-Replayed", "true")
		}
		message := coreErr.Message
		if message == "" {
			message = "Owner surface request failed"
		}
		writeErrorWithDetails(response, coreErr.Status, coreErr.Code, message, coreErr.Details)
		return
	}
	if s.publicUnauthorized(response, err) {
		return
	}
	writeError(response, http.StatusBadGateway, "surface_failed", "Owner surface request failed")
}

func surfaceNotProvided(path string, result map[string]any) bool {
	if strings.HasSuffix(path, "/notifications/unread-count") {
		_, ok := result["count"]
		return !ok
	}
	if strings.HasPrefix(path, "/api/notifications/") && strings.HasSuffix(path, "/read") {
		_, ok := result["id"]
		return !ok
	}
	return false
}

func surfaceStatus(value any) int {
	switch typed := value.(type) {
	case int:
		if typed >= 200 {
			return typed
		}
	case float64:
		if typed >= 200 {
			return int(typed)
		}
	}
	return http.StatusOK
}

func surfaceBody(response http.ResponseWriter, request *http.Request) (map[string]any, bool) {
	mediaType, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			writeError(response, http.StatusBadRequest, "invalid_request", "Request validation failed")
			return nil, false
		}
		body := map[string]any{}
		if request.MultipartForm != nil {
			if values := request.MultipartForm.Value["idempotencyKey"]; len(values) > 0 {
				body["idempotencyKey"] = values[0]
			}
		}
		return body, true
	}
	if request.URL.Path == "/api/studio/packs" || request.URL.Path == "/api/studio/packs/validate" {
		defer request.Body.Close()
		data, err := io.ReadAll(io.LimitReader(request.Body, (4<<20)+1))
		var body map[string]any
		if err != nil || len(data) > 4<<20 || json.Unmarshal(data, &body) != nil || body == nil {
			writeError(response, 422, "resource_pack_invalid", "配置包请求格式无效或超过4MiB")
			return nil, false
		}
		return body, true
	}
	return readBody(response, request)
}

func surfacePathMatches(path, pattern string) bool {
	pathParts, patternParts := splitPath(path), splitPath(pattern)
	if len(pathParts) != len(patternParts) {
		return false
	}
	for index, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			if pathParts[index] == "" {
				return false
			}
			continue
		}
		if pathParts[index] != part {
			return false
		}
	}
	return true
}

// Definition content is an opaque schema; only transport envelope keys change.
func surfaceCommandBody(path string, parsed map[string]any) map[string]any {
	body := snakeMap(parsed)
	if strings.HasSuffix(path, "/inventory/tags") {
		if value, ok := parsed["renames"]; ok {
			body["renames"] = value
		}
	}
	if path == "/api/studio/packs" || path == "/api/studio/packs/validate" {
		body["pack"] = parsed["pack"]
	}
	if path == "/api/diagnostic-records/clear" {
		body["filters"] = parsed["filters"]
	}
	parts := splitPath(path)
	if path == "/api/studio/jobs" || (len(parts) >= 3 && parts[0] == "api" && parts[1] == "studio" && parts[2] == "resources" && (len(parts) == 3 || len(parts) == 4)) {
		if raw, ok := parsed["body"]; ok {
			body["body"] = raw
		}
	}
	return body
}

func snakeMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(value))
	for key, child := range value {
		out[camelToSnake(key)] = snakeValue(child)
	}
	return out
}

func snakeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return snakeMap(typed)
	case []any:
		items := make([]any, len(typed))
		for index, child := range typed {
			items[index] = snakeValue(child)
		}
		return items
	default:
		return value
	}
}

func camelToSnake(key string) string {
	var builder strings.Builder
	for index, char := range key {
		if unicode.IsUpper(char) {
			if index > 0 {
				builder.WriteByte('_')
			}
			builder.WriteRune(unicode.ToLower(char))
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func validPersonaSwitchBody(body map[string]any) bool {
	if len(body) != 6 || !validateString(body["targetPersonaId"], 1, 128) || !validateString(body["idempotencyKey"], 1, 256) || !validateString(body["reason"], 1, 1000) || body["confirm"] != true {
		return false
	}
	for _, key := range []string{"expectedRevision", "expectedRuntimeRevision"} {
		value, ok := body[key].(float64)
		if !ok || value < 0 || value > 2147483647 || value != float64(int64(value)) {
			return false
		}
	}
	return true
}
