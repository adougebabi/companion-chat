package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
)

const OwnerMediaUploadMaxBytes int64 = 64 << 20

type OwnerMediaUploadInput struct {
	Filename, MIME, IdempotencyKey string
	Content                        io.Reader
	Size                           int64
}
type preparedOwnerUpload struct {
	file                              *os.File
	filename, kind, mime, sha, digest string
	size                              int64
}

func canonicalUploadMIME(value string) string {
	base, _, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	switch strings.ToLower(base) {
	case "image/jpg":
		return "image/jpeg"
	case "audio/x-wav", "audio/wave":
		return "audio/wav"
	case "application/ogg":
		return "audio/ogg"
	}
	return strings.ToLower(base)
}

func prepareOwnerMediaUpload(input OwnerMediaUploadInput) (preparedOwnerUpload, error) {
	if input.Content == nil || strings.TrimSpace(input.IdempotencyKey) == "" || len(input.IdempotencyKey) > 256 || input.Size < 0 || input.Size > OwnerMediaUploadMaxBytes {
		return preparedOwnerUpload{}, invalid("A bounded file and stable idempotency key are required")
	}
	file, err := os.CreateTemp("", "fluctlight-upload-*")
	if err != nil {
		return preparedOwnerUpload{}, err
	}
	keep := false
	defer func() {
		if !keep {
			file.Close()
			os.Remove(file.Name())
		}
	}()
	digest := sha256.New()
	size, err := io.Copy(io.MultiWriter(file, digest), io.LimitReader(input.Content, OwnerMediaUploadMaxBytes+1))
	if err != nil {
		return preparedOwnerUpload{}, err
	}
	if size == 0 || size > OwnerMediaUploadMaxBytes || input.Size > 0 && size != input.Size {
		return preparedOwnerUpload{}, &SurfaceError{Status: 413, Code: "media_upload_size_invalid", Message: "文件为空、长度不符或超过64MiB"}
	}
	header := make([]byte, 512)
	n, readErr := file.ReadAt(header, 0)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return preparedOwnerUpload{}, readErr
	}
	detected := canonicalUploadMIME(http.DetectContentType(header[:n]))
	declared := canonicalUploadMIME(input.MIME)
	if strings.TrimSpace(input.MIME) != "" && declared == "" {
		return preparedOwnerUpload{}, &SurfaceError{Status: 422, Code: "media_upload_mime_mismatch", Message: "提交的媒体类型不可读取"}
	}
	allowed := map[string]string{"image/png": "image", "image/jpeg": "image", "image/gif": "image", "image/webp": "image", "video/mp4": "video", "video/webm": "video", "audio/mpeg": "audio", "audio/wav": "audio", "audio/ogg": "audio"}
	kind, ok := allowed[detected]
	if !ok {
		return preparedOwnerUpload{}, &SurfaceError{Status: 415, Code: "media_upload_type_unsupported", Message: "支持PNG、JPEG、GIF、WebP、MP4、WebM、MP3、WAV或Ogg媒体"}
	}
	if declared != "" && declared != "application/octet-stream" && declared != detected {
		return preparedOwnerUpload{}, &SurfaceError{Status: 422, Code: "media_upload_mime_mismatch", Message: "文件内容类型与提交的类型不一致"}
	}
	name := strings.TrimSpace(filepath.Base(strings.ReplaceAll(input.Filename, "\\", "/")))
	if name == "" || name == "." {
		name = "未命名上传素材"
	}
	if len([]rune(name)) > 200 {
		return preparedOwnerUpload{}, invalid("Filename must be at most 200 characters")
	}
	sha := hex.EncodeToString(digest.Sum(nil))
	requestDigest := stableDigest(jsonString(map[string]any{"filename": name, "mime": detected, "bytes": size, "sha256": sha}))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return preparedOwnerUpload{}, err
	}
	keep = true
	return preparedOwnerUpload{file, name, kind, detected, sha, requestDigest, size}, nil
}

func (a *App) requireMediaOwner(ctx context.Context, owner string) error {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return &SurfaceError{Status: 503, Code: "media_service_unavailable", Message: "媒体持久服务当前不可用"}
	}
	var allowed bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.owner_accounts WHERE human_actor_id=$1)`, owner).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ErrUnauthorized
	}
	return nil
}

func (a *App) UploadOwnerMedia(ctx context.Context, owner string, input OwnerMediaUploadInput) (map[string]any, error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if err := a.requireMediaOwner(ctx, owner); err != nil {
		return nil, err
	}
	if a.Storage == nil || a.S3Bucket == "" {
		return nil, &SurfaceError{Status: 503, Code: "media_storage_not_configured", Message: "媒体对象存储尚未配置"}
	}
	prepared, err := prepareOwnerMediaUpload(input)
	if err != nil {
		return nil, err
	}
	defer func() { prepared.file.Close(); os.Remove(prepared.file.Name()) }()
	id := "owner_upload_" + stableDigest(owner+"\x1f"+input.IdempotencyKey)
	assetID := "upload_asset_" + stableDigest(id)
	key := "media/" + assetID + "/v1"
	storageScope := a.ownerUploadStorageScope()
	release, acquired, err := a.tryDailyReviewExecutionLock(ctx, owner, "owner-media-key:"+stableDigest(input.IdempotencyKey))
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, &SurfaceError{Status: 409, Code: "media_upload_in_progress", Message: "原上传仍在处理，请查询或重试同一提交", Details: map[string]any{"upload_id": id}}
	}
	defer release()
	var record map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT to_jsonb(u) FROM public.owner_media_uploads u WHERE owner_actor_id=$1 AND idempotency_key=$2 FOR UPDATE`, owner, input.IdempotencyKey).Scan(&raw)
		if err == nil {
			record = decodeObject(raw)
			if record["request_digest"] != prepared.digest {
				return &SurfaceError{Status: 422, Code: "media_upload_key_reused", Message: "原上传身份已绑定另一份文件；请先核对原提交结果"}
			}
			if record["storage_scope"] != storageScope {
				return &SurfaceError{Status: 409, Code: "media_upload_storage_scope_changed", Message: "原上传的对象存储范围已变化，请恢复原配置或执行受控恢复；原提交保持"}
			}
			id, assetID, key = stringValue(record["id"]), stringValue(record["asset_id"]), stringValue(record["object_key"])
			if record["status"] == "ready" {
				return nil
			}
			_, err = tx.Exec(ctx, `UPDATE public.owner_media_uploads SET status='uploading',error_code=NULL,revision=revision+1,updated_at=now() WHERE id=$1`, id)
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.owner_media_uploads(id,owner_actor_id,idempotency_key,request_digest,asset_id,filename,kind,mime_type,byte_size,sha256,bucket,object_key,storage_scope,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'uploading')`, id, owner, input.IdempotencyKey, prepared.digest, assetID, prepared.filename, prepared.kind, prepared.mime, prepared.size, prepared.sha, a.S3Bucket, key, storageScope); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if record["status"] == "ready" {
		return a.ownerUploadReceipt(ctx, owner, id, true)
	}
	etag, version, err := a.putOrRecoverOwnerUpload(ctx, key, prepared)
	if err != nil {
		code := ownerMediaStorageErrorCode(err)
		a.failOwnerUpload(ctx, id, code)
		return nil, &SurfaceError{Status: 503, Code: "media_upload_object_failed", Message: "对象写入或校验未完成；保留原文件与提交身份重试", Details: map[string]any{"upload_id": id, "storage_error_code": code, "retry_same_identity": true}}
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var digest, status string
		if err := tx.QueryRow(ctx, `SELECT request_digest,status FROM public.owner_media_uploads WHERE id=$1 AND owner_actor_id=$2 FOR UPDATE`, id, owner).Scan(&digest, &status); err != nil {
			return err
		}
		if digest != prepared.digest {
			return ErrConflict
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,object_version,etag,provider_request_id,workflow_id,status,ready_at,media_scope,upload_owner_actor_id,owner_upload_id) VALUES($1,NULL,'1',$2,$3,$4,$5,$6,$7,$8,$9,NULL,NULL,'ready',now(),'owner_upload',$10,$11) ON CONFLICT(id) DO NOTHING`, assetID, prepared.kind, prepared.mime, prepared.size, prepared.sha, a.S3Bucket, key, nilIfEmpty(version), nilIfEmpty(etag), owner, id); err != nil {
			return err
		}
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT media_scope='owner_upload' AND upload_owner_actor_id=$2 AND owner_upload_id=$3 AND status='ready' AND sha256=$4 AND byte_size=$5 AND mime_type=$6 AND bucket=$7 AND object_key=$8 FROM public.media_assets WHERE id=$1`, assetID, owner, id, prepared.sha, prepared.size, prepared.mime, a.S3Bucket, key).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return errors.New("owner_upload_asset_conflict")
		}
		if _, err := tx.Exec(ctx, `UPDATE public.owner_media_uploads SET status='ready',error_code=NULL,revision=revision+1,updated_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		_, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, "", "media_upload", "所有者上传原始媒体素材", id)
		return err
	})
	if err != nil {
		a.failOwnerUpload(ctx, id, "media_result_persistence_ambiguous")
		return nil, &SurfaceError{Status: 503, Code: "media_upload_result_unknown", Message: "对象已处理但结果提交尚未确认，请查询原上传或使用原身份重试", Details: map[string]any{"upload_id": id, "retry_same_identity": true}}
	}
	return a.ownerUploadReceipt(ctx, owner, id, false)
}

func ownerMediaStorageErrorCode(err error) string {
	code := minio.ToErrorResponse(err).Code
	switch code {
	case "NoSuchBucket", "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch", "ServiceUnavailable", "SlowDown":
		return code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "storage_timeout"
	}
	var timed net.Error
	if errors.As(err, &timed) && timed.Timeout() {
		return "storage_timeout"
	}
	switch err.Error() {
	case "upload_object_metadata_conflict", "upload_object_checksum_conflict", "upload_object_size_conflict", "bucket_versioning_required":
		return err.Error()
	}
	return "media_object_write_failed"
}

func (a *App) ownerUploadStorageScope() string {
	if a.Storage == nil {
		return ""
	}
	return stableDigest(a.Storage.EndpointURL().String() + "\x1f" + a.S3Bucket)
}

func (a *App) failOwnerUpload(ctx context.Context, id, code string) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	// Keep an already committed ready receipt authoritative if the original
	// transaction committed but its acknowledgement was lost.
	_, _ = a.DB.Pool().Exec(cleanup, `UPDATE public.owner_media_uploads SET status='failed',error_code=$2,revision=revision+1,updated_at=now() WHERE id=$1 AND status<>'ready'`, id, code)
}

func (a *App) putOrRecoverOwnerUpload(ctx context.Context, key string, p preparedOwnerUpload) (string, string, error) {
	stat, err := a.Storage.StatObject(ctx, a.S3Bucket, key, minio.StatObjectOptions{})
	if err == nil {
		if stat.VersionID == "" || stat.VersionID == "null" {
			return "", "", errors.New("bucket_versioning_required")
		}
		if stat.Size != p.size || canonicalUploadMIME(stat.ContentType) != p.mime {
			return "", "", errors.New("upload_object_metadata_conflict")
		}
		object, err := a.Storage.GetObject(ctx, a.S3Bucket, key, minio.GetObjectOptions{VersionID: stat.VersionID})
		if err != nil {
			return "", "", err
		}
		defer object.Close()
		digest := sha256.New()
		size, err := io.Copy(digest, io.LimitReader(object, OwnerMediaUploadMaxBytes+1))
		if err != nil || size != p.size || hex.EncodeToString(digest.Sum(nil)) != p.sha {
			if err == nil {
				err = errors.New("upload_object_checksum_conflict")
			}
			return "", "", err
		}
		return stat.ETag, stat.VersionID, nil
	}
	code := minio.ToErrorResponse(err).Code
	if code != "NoSuchKey" && code != "NoSuchObject" && code != "NotFound" {
		return "", "", err
	}
	if _, err := p.file.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	info, err := a.Storage.PutObject(ctx, a.S3Bucket, key, p.file, p.size, minio.PutObjectOptions{ContentType: p.mime})
	if err != nil {
		return "", "", err
	}
	if info.Size != p.size {
		return "", "", errors.New("upload_object_size_conflict")
	}
	if info.VersionID == "" || info.VersionID == "null" {
		return "", "", errors.New("bucket_versioning_required")
	}
	// Verify the exact stored version, including bytes, before publishing ready.
	stat, err = a.Storage.StatObject(ctx, a.S3Bucket, key, minio.StatObjectOptions{VersionID: info.VersionID})
	if err != nil {
		return "", "", err
	}
	if stat.Size != p.size || canonicalUploadMIME(stat.ContentType) != p.mime {
		return "", "", errors.New("upload_object_metadata_conflict")
	}
	object, err := a.Storage.GetObject(ctx, a.S3Bucket, key, minio.GetObjectOptions{VersionID: info.VersionID})
	if err != nil {
		return "", "", err
	}
	defer object.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, io.LimitReader(object, OwnerMediaUploadMaxBytes+1))
	if err != nil {
		return "", "", err
	}
	if size != p.size || hex.EncodeToString(digest.Sum(nil)) != p.sha {
		return "", "", errors.New("upload_object_checksum_conflict")
	}
	return stat.ETag, info.VersionID, nil
}

func (a *App) ownerUploadReceipt(ctx context.Context, owner, id string, replayed bool) (map[string]any, error) {
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT jsonb_build_object('upload_id',u.id,'asset_id',m.id,'url','/api/media/'||m.id,'status','ready','origin','owner_upload','version',m.version,'kind',m.kind,'mime_type',m.mime_type,'byte_size',m.byte_size,'sha256',m.sha256,'factual',false,'uploaded_by','owner','label','用户上传素材·非角色事实','replayed',$3::boolean) FROM public.owner_media_uploads u JOIN public.media_assets m ON m.id=u.asset_id AND m.owner_upload_id=u.id AND m.upload_owner_actor_id=u.owner_actor_id AND m.media_scope='owner_upload' WHERE u.id=$1 AND u.owner_actor_id=$2 AND u.status='ready' AND m.status='ready'`, id, owner, replayed).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &SurfaceError{Status: 409, Code: "media_upload_receipt_unavailable", Message: "原上传结果当前不可用，请查询媒体状态；不会创建第二份资产"}
	}
	return decodeObject(raw), err
}
