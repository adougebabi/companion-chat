package browser

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"testing"
)

// Optional transports are installed only for the positive matrix. The ordinary
// fakeBackend intentionally lacks them, so unavailable-service cases stay real.
type matrixTransportBackend struct {
	*fakeBackend
	replayed bool
	bytes    []byte
}

func (b *matrixTransportBackend) UploadOwnerMedia(_ context.Context, session string, request *http.Request) (map[string]any, error) {
	if session != "opaque" {
		return nil, fmt.Errorf("unexpected upload session")
	}
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		return nil, err
	}
	defer request.MultipartForm.RemoveAll()
	if request.FormValue("idempotencyKey") != "upload-1" {
		return nil, fmt.Errorf("missing original upload identity")
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	b.bytes, err = io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if header.Filename != "fixture.txt" || string(b.bytes) != "actual multipart fixture bytes" {
		return nil, fmt.Errorf("multipart bytes were lost")
	}
	return map[string]any{"id": "asset-upload-fixture", "replayed": b.replayed}, nil
}

func (b *matrixTransportBackend) ExportDiagnostics(_ context.Context, session string, _ url.Values, w http.ResponseWriter) error {
	if session != "opaque" {
		return fmt.Errorf("unexpected diagnostic session")
	}
	_, err := io.WriteString(w, "{\"type\":\"manifest\",\"snapshot\":\"100:100:\"}\n{\"type\":\"end\",\"complete\":true,\"counts\":{}}\n")
	return err
}

func multipartUploadFixture(t *testing.T) (string, string) {
	t.Helper()
	var data bytes.Buffer
	w := multipart.NewWriter(&data)
	if err := w.WriteField("idempotencyKey", "upload-1"); err != nil {
		t.Fatal(err)
	}
	file, err := w.CreateFormFile("file", "fixture.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write([]byte("actual multipart fixture bytes")); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	return data.String(), w.FormDataContentType()
}

func browserPackFixture(importing bool) string {
	// Match Core's canonical JSON digest; digest and body always describe the
	// same public definition, rather than an arbitrary hash accepted by a fake.
	digest := func(value any) string { raw, _ := json.Marshal(value); return fmt.Sprintf("%x", sha256.Sum256(raw)) }
	entry := map[string]any{"key": "resource-1@published:1", "source_resource_id": "resource-1", "source": "published", "source_version": "1", "source_revision": 1, "kind": "place", "name": "Reading room", "body": map[string]any{"description": "A room with a reading table"}}
	entry["checksum"] = digest(entry)
	pack := map[string]any{"schema": "fluctlight-resource-pack.v1", "created_at": "2026-10-19T00:00:00Z", "resources": []any{entry}, "groups": []any{map[string]any{"name": "Imported group"}}, "preferences": map[string]any{}}
	checksum := digest(pack)
	pack["checksum"] = checksum
	body := map[string]any{"pack": pack}
	if importing {
		body["expectedDigest"], body["reason"], body["idempotencyKey"] = checksum, "Owner imports a verified pack", "pack-import"
	}
	raw, _ := json.Marshal(body)
	return string(raw)
}

func TestMultipartUploadBoundaryPreservesBytesAndReplayStatus(t *testing.T) {
	for _, replay := range []bool{false, true} {
		backend := &matrixTransportBackend{fakeBackend: &fakeBackend{}, replayed: replay}
		body, contentType := multipartUploadFixture(t)
		headers := map[string]string{"Content-Type": contentType, "Origin": "https://fluctlight.test", "X-CSRF-Token": "csrf"}
		cookies := map[string]string{sessionCookieName: "opaque", csrfCookieName: "csrf"}
		response := invokeBrowser(newBrowserTestHandler(backend), http.MethodPost, "https://api.test/api/media", body, headers, cookies)
		want := http.StatusCreated
		if replay {
			want = http.StatusOK
		}
		if response.Code != want || len(backend.bytes) == 0 {
			t.Fatalf("upload replay=%v status=%d body=%s", replay, response.Code, response.Body.String())
		}
	}
	unavailable := surfaceInvoke(newBrowserTestHandler(&fakeBackend{}), http.MethodPost, "/api/media", "", true, true)
	if unavailable.code != 503 || unavailable.json(t)["code"] != "media_upload_unavailable" {
		t.Fatalf("unavailable upload=%d %s", unavailable.code, unavailable.body)
	}
}
