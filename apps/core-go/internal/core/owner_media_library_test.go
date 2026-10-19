package core

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// This fixture is a controlled S3 transport; it is not evidence of MinIO or
// a production Provider. The test still uses the actual SDK, PG and domain.
func TestOwnerMediaUploadRecoversSameObjectAndFencesOwnerAndMetadata(t *testing.T) {
	f := newIndependentToolE2EFixture(t, "owner_media")
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'controlled','test'),($2,'controlled','test')`, f.ownerID, f.foreignOwnerID); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var content []byte
	puts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPut {
			var err error
			content, err = io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			puts++
			w.Header().Set("ETag", `"controlled-etag"`)
			w.Header().Set("x-amz-version-id", "stored-v1")
			return
		}
		if content == nil {
			w.WriteHeader(404)
			if r.Method != http.MethodHead {
				io.WriteString(w, `<Error><Code>NoSuchKey</Code></Error>`)
			}
			return
		}
		if version := r.URL.Query().Get("versionId"); version != "" && version != "stored-v1" {
			t.Errorf("wrong exact version %q", version)
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"controlled-etag"`)
		w.Header().Set("x-amz-version-id", "stored-v1")
		w.Header().Set("Last-Modified", "Mon, 19 Oct 2026 00:00:00 GMT")
		if r.Method == http.MethodGet {
			_, _ = w.Write(content)
		}
	}))
	defer server.Close()
	storage, err := minio.New(strings.TrimPrefix(server.URL, "https://"), &minio.Options{Secure: true, Transport: server.Client().Transport, Creds: credentials.NewStaticV4("controlled", "controlled", ""), Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	f.app.Storage = storage
	f.app.S3Bucket = "controlled-media"
	png := ownerMediaPNG(t)
	input := func() OwnerMediaUploadInput {
		return OwnerMediaUploadInput{Filename: "original.png", MIME: "image/png", IdempotencyKey: "upload-original", Content: bytes.NewReader(png), Size: int64(len(png))}
	}
	// Fail only the final result commit, after object I/O, without changing input.
	if _, err := f.repository.Pool().Exec(f.ctx, `CREATE FUNCTION public.reject_owner_upload_result() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN IF NEW.media_scope='owner_upload' THEN RAISE EXCEPTION 'controlled result failure';END IF;RETURN NEW;END$$;CREATE TRIGGER reject_owner_upload_result BEFORE INSERT ON public.media_assets FOR EACH ROW EXECUTE FUNCTION public.reject_owner_upload_result()`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.UploadOwnerMedia(f.ctx, f.ownerID, input()); err == nil {
		t.Fatal("result failure became success")
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `DROP TRIGGER reject_owner_upload_result ON public.media_assets;DROP FUNCTION public.reject_owner_upload_result()`); err != nil {
		t.Fatal(err)
	}
	receipt, err := f.app.UploadOwnerMedia(f.ctx, f.ownerID, input())
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.app.UploadOwnerMedia(f.ctx, f.ownerID, input())
	if err != nil || replay["asset_id"] != receipt["asset_id"] || replay["replayed"] != true {
		t.Fatal("replay changed asset", replay, err)
	}
	mu.Lock()
	if puts != 1 {
		t.Error("recovery duplicated object write", puts)
	}
	mu.Unlock()
	lookup, err := f.app.DispatchOwnerMediaSurface(f.ctx, "GET", "/internal/media-uploads/by-key", url.Values{"key": {"upload-original"}}, f.ownerID, nil)
	if err != nil || mapValue(lookup["asset"])["id"] != receipt["asset_id"] {
		t.Fatal("lookup lacks real receipt", lookup, err)
	}
	id := stringValue(receipt["asset_id"])
	detail, err := f.app.ownerMediaAssetWith(f.ctx, f.repository.Pool(), f.ownerID, id, false)
	if err != nil || detail["readable"] != true || detail["factual"] != false {
		t.Fatal("invalid upload projection", detail, err)
	}

	avatarBefore, err := f.app.DispatchOwnerMediaSurface(f.ctx, "GET", "/internal/fluctlights/"+f.fluctlightID+"/display-avatar", nil, f.ownerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	avatarBody := map[string]any{"mode": "asset", "asset_id": id, "expected_asset_revision": detail["asset_revision"], "expected_revision": avatarBefore["revision"], "reason": "Explicit display selection only", "idempotency_key": "display-original"}
	avatar, err := f.app.DispatchOwnerMediaSurface(f.ctx, "PUT", "/internal/fluctlights/"+f.fluctlightID+"/display-avatar", nil, f.ownerID, avatarBody)
	if err != nil || mapValue(avatar["avatar"])["asset_id"] != id {
		t.Fatal("display selection did not consume actual asset", avatar, err)
	}
	projection := map[string]any{}
	f.app.projectAvatar(f.ctx, f.ownerID, f.fluctlightID, projection)
	if mapValue(projection["avatar"])["asset_id"] != id {
		t.Fatal("detail did not use actual display selection", projection)
	}
	var canonicalCount int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&canonicalCount); err != nil || canonicalCount != 0 {
		t.Fatal("display selection changed canonical identity", canonicalCount, err)
	}
	foreignBody := cloneMap(avatarBody)
	foreignBody["idempotency_key"] = "foreign-display"
	foreignBody["expected_revision"] = avatar["revision"]
	if _, err := f.app.DispatchOwnerMediaSurface(f.ctx, "PUT", "/internal/fluctlights/"+f.fluctlightID+"/display-avatar", nil, f.foreignOwnerID, foreignBody); err == nil {
		t.Fatal("foreign Owner changed display selection")
	}
	reset, err := f.app.DispatchOwnerMediaSurface(f.ctx, "PUT", "/internal/fluctlights/"+f.fluctlightID+"/display-avatar", nil, f.ownerID, map[string]any{"mode": "canonical", "expected_revision": avatar["revision"], "reason": "Return display to canonical source", "idempotency_key": "display-canonical"})
	if err != nil || reset["mode"] != "canonical" || reset["avatar"] != nil {
		t.Fatal("canonical selection fabricated a canonical asset", reset, err)
	}
	var references int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.media_references WHERE owner_fluctlight_id=$1 AND target_type='display_avatar'`, f.fluctlightID).Scan(&references); err != nil || references != 0 {
		t.Fatal("old current display reference remained", references, err)
	}
	history, err := f.app.ownerDisplayAvatarHistory(f.ctx, f.ownerID, f.fluctlightID, nil)
	if err != nil || len(arrayValue(history["items"])) != 2 {
		t.Fatal("display history lost original selection", history, err)
	}
	if err := f.app.AuthorizeAsset(f.ctx, f.foreignOwnerID, id); err == nil {
		t.Fatal("foreign Owner read uploaded bytes")
	}
	if _, err := f.app.DispatchOwnerMediaSurface(f.ctx, "GET", "/internal/media-assets/"+id, nil, f.foreignOwnerID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign Owner read metadata", err)
	}
	if _, err := f.app.DispatchOwnerMediaSurface(f.ctx, "GET", "/internal/media-assets/"+id, url.Values{"fluctlight_id": {f.fluctlightID}}, f.ownerID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatal("Owner upload appeared on unrelated role", err)
	}
	command := map[string]any{"expected_asset_revision": detail["asset_revision"], "expected_metadata_revision": detail["metadata_revision"], "title": "Renamed", "hidden": true, "reason": "Library display only", "idempotency_key": "metadata-original"}
	saved, err := f.app.DispatchOwnerMediaSurface(f.ctx, "PUT", "/internal/media-assets/"+id, nil, f.ownerID, command)
	if err != nil || saved["hidden"] != true || saved["sha256"] != detail["sha256"] {
		t.Fatal("metadata save failed", saved, err)
	}
	stale := cloneMap(command)
	stale["idempotency_key"] = "new-stale-identity"
	if _, err := f.app.DispatchOwnerMediaSurface(f.ctx, "PUT", "/internal/media-assets/"+id, nil, f.ownerID, stale); err == nil {
		t.Fatal("stale metadata overwrote newer revision")
	}

	// Different keys must serialize before taking a fresh metadata snapshot.
	concurrent := cloneMap(command)
	concurrent["expected_metadata_revision"] = saved["metadata_revision"]
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, key := range []string{"metadata-race-a", "metadata-race-b"} {
		body := cloneMap(concurrent)
		body["idempotency_key"] = key
		go func() {
			<-start
			_, err := f.app.DispatchOwnerMediaSurface(f.ctx, "PUT", "/internal/media-assets/"+id, nil, f.ownerID, body)
			results <- err
		}()
	}
	close(start)
	success, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		var se *SurfaceError
		if err == nil {
			success++
		} else if errors.As(err, &se) && se.Code == "revision_conflict" {
			conflicts++
		} else {
			t.Fatal("unexpected concurrent result", err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal("concurrent CAS admitted stale annotations", success, conflicts)
	}
	page, err := f.app.ownerMediaLibrary(f.ctx, f.ownerID, url.Values{"scope": {"owner_upload"}, "hidden": {"true"}, "kind": {"image"}})
	if err != nil || len(arrayValue(page["items"])) != 1 {
		t.Fatal("hidden filter did not query server", page, err)
	}

	resource, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "place", "name": "Explicit reference target", "body": map[string]any{"description": "Fictional studio source"}, "reason": "Reference fixture", "idempotency_key": "reference-target"})
	if err != nil {
		t.Fatal(err)
	}
	grantBody := map[string]any{"asset_id": id, "expected_asset_revision": detail["asset_revision"], "target_scope": "studio_resource", "target_id": resource["id"], "purpose": "person", "confirm": true, "reason": "Owner explicitly permits this unverified image for this resource only", "idempotency_key": "grant-original"}
	grant, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", "/internal/media-reference-grants", nil, f.ownerID, grantBody)
	if err != nil {
		t.Fatal(err)
	}
	reference := map[string]any{"asset_id": id, "purpose": "person", "grant_id": grant["id"], "grant_revision": grant["revision"]}
	concept := map[string]any{"source": "studio_definition", "studio_owner_actor_id": f.ownerID, "studio_source": map[string]any{"resource_id": resource["id"]}, "reference_asset": cloneMap(reference)}
	mime, actual, err := f.app.readExplicitStudioReference(f.ctx, f.ownerID, reference, concept)
	if err != nil || mime != "image/png" || !bytes.Equal(actual, png) {
		t.Fatal("explicit reference did not use exact stored bytes", mime, len(actual), err)
	}
	if err := f.app.validateExplicitMediaReference(f.ctx, f.repository.Pool(), f.ownerID, "another-resource", reference); err == nil {
		t.Fatal("reference grant crossed target resource")
	}
	if err := f.app.validateExplicitMediaReference(f.ctx, f.repository.Pool(), f.foreignOwnerID, stringValue(resource["id"]), reference); err == nil {
		t.Fatal("reference grant crossed Owner")
	}
	referenceJob, err := f.app.DispatchStudioJobSurface(f.ctx, "POST", "/internal/studio/jobs", nil, f.ownerID, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "image_preview", "name": resource["name"], "body": resource["body"], "instruction": "A fictional image using explicitly allowed reference, no private Actor claims", "reference_asset": reference, "reason": "Reference admission only, no model executed", "idempotency_key": "explicit-reference-job"})
	if err != nil {
		t.Fatal(err)
	}
	if mapValue(mapValue(referenceJob["snapshot"])["reference_asset"])["grant_id"] != grant["id"] {
		t.Fatal("reference authorization not frozen in actual job")
	}
	uses, err := f.app.mediaReferenceGrantRecords(f.ctx, f.ownerID, stringValue(grant["id"]), "uses", nil)
	if err != nil || len(arrayValue(uses["items"])) != 1 {
		t.Fatal("actual grant use missing", uses, err)
	}

	uploaded := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseUpload := func() { releaseOnce.Do(func() { close(release) }) }
	promptCalls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload/image" {
			close(uploaded)
			<-release
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"name":"controlled-reference.png"}`)
			return
		}
		mu.Lock()
		promptCalls++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"prompt_id":"must-not-submit-after-revocation"}`)
	}))
	defer func() { releaseUpload(); provider.Close() }()
	f.app.Provider.HTTP = provider.Client()
	submission := make(chan error, 1)
	go func() {
		_, err := f.app.uploadComfyReferenceImage(f.ctx, provider.URL, id, "image/png", png)
		if err == nil {
			response, submitErr := f.app.submitComfyPrompt(f.ctx, mediaIntent{StudioOwner: f.ownerID, StudioJobID: stringValue(referenceJob["id"])}, concept, map[string]any{}, "controlled prompt", provider.URL)
			err = submitErr
			if response != nil {
				response.Body.Close()
			}
		}
		submission <- err
	}()
	select {
	case <-uploaded:
	case <-time.After(5 * time.Second):
		t.Fatal("controlled reference upload did not start")
	}
	revoked, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", "/internal/media-reference-grants/"+stringValue(grant["id"])+"/revoke", nil, f.ownerID, map[string]any{"expected_revision": grant["revision"], "reason": "Stop new reference consumption", "idempotency_key": "revoke-reference"})
	if err != nil || revoked["status"] != "revoked" {
		t.Fatal("revocation did not persist", revoked, err)
	}

	releaseUpload()
	if err := <-submission; err == nil {
		t.Fatal("Provider prompt submitted after grant was revoked during image upload")
	}
	mu.Lock()
	if promptCalls != 0 {
		t.Error("revoked reference started a new Provider job", promptCalls)
	}
	mu.Unlock()
	if _, _, err := f.app.readExplicitStudioReference(f.ctx, f.ownerID, reference, concept); err == nil {
		t.Fatal("revoked grant still transmitted bytes")
	}
	if _, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", "/internal/media-reference-grants", nil, f.ownerID, grantBody); err != nil {
		t.Fatal("original grant command could not replay", err)
	}
	currentGrant, err := readMediaReferenceGrant(f.ctx, f.repository.Pool(), f.ownerID, stringValue(grant["id"]), false)
	if err != nil || currentGrant["status"] != "revoked" {
		t.Fatal("old grant receipt reactivated revoked authority", currentGrant, err)
	}
	f.app.S3Bucket = "another-scope"
	changed, err := f.app.ownerMediaAssetWith(f.ctx, f.repository.Pool(), f.ownerID, id, false)
	if err != nil || changed["readable"] != false || changed["read_state"] != "storage_scope_changed" {
		t.Fatal("changed storage scope read wrong bytes", changed, err)
	}
	if _, err := f.app.UploadOwnerMedia(f.ctx, f.ownerID, input()); err == nil {
		t.Fatal("original command recreated in new storage scope")
	}
}
