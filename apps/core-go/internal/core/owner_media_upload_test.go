package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/minio/minio-go/v7"
	"io"
	"os"
	"testing"
)

func ownerMediaPNG(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestOwnerMediaPreparationValidatesActualBytesAndPrivateSpool(t *testing.T) {
	bytesValue := ownerMediaPNG(t)
	p, err := prepareOwnerMediaUpload(OwnerMediaUploadInput{Filename: `..\folder/actual.png`, MIME: "image/png", IdempotencyKey: "original", Content: bytes.NewReader(bytesValue), Size: int64(len(bytesValue))})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { p.file.Close(); os.Remove(p.file.Name()) }()
	info, err := p.file.Stat()
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("upload spool is not private", err)
	}
	digest := sha256.Sum256(bytesValue)
	if p.sha != hex.EncodeToString(digest[:]) || p.kind != "image" || p.filename != "actual.png" || p.mime != "image/png" {
		t.Fatal("invalid actual bytes projection", p)
	}
	read, err := io.ReadAll(p.file)
	if err != nil || !bytes.Equal(read, bytesValue) {
		t.Fatal("spool is not rewound", err)
	}
	for _, tc := range []struct {
		name, mime string
		content    []byte
		size       int64
	}{
		{"mismatch", "image/jpeg", bytesValue, int64(len(bytesValue))}, {"malformed", "not a MIME", bytesValue, int64(len(bytesValue))}, {"empty", "image/png", nil, 0}, {"size mismatch", "image/png", bytesValue, 999}, {"too large", "image/png", bytesValue, OwnerMediaUploadMaxBytes + 1}, {"unsupported", "text/plain", []byte("text is not media"), 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := prepareOwnerMediaUpload(OwnerMediaUploadInput{Filename: "candidate.png", MIME: tc.mime, IdempotencyKey: "original", Content: bytes.NewReader(tc.content), Size: tc.size})
			if err == nil {
				p.file.Close()
				os.Remove(p.file.Name())
				t.Fatal("unsafe upload accepted")
			}
		})
	}
}
func TestOwnerMediaStorageErrorCodesAreBounded(t *testing.T) {
	if ownerMediaStorageErrorCode(errors.New("https://private-host secret=credential")) != "media_object_write_failed" {
		t.Fatal("raw error exposed")
	}
	if ownerMediaStorageErrorCode(minio.ErrorResponse{Code: "AccessDenied", Message: "private detail"}) != "AccessDenied" {
		t.Fatal("safe cause lost")
	}
}
