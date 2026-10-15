package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresSurface struct{ pool *pgxpool.Pool }

func newPostgresSurface(pool *pgxpool.Pool) SurfaceStore {
	return &postgresSurface{pool: pool}
}

type surfaceDBTxKey struct{}

// SurfaceDBTx returns the owner-surface transaction when the caller is inside
// SurfaceStore.Transact. Wardrobe corrections join that transaction so the
// state write and the owner audit commit or roll back together.
func SurfaceDBTx(ctx context.Context) (pgx.Tx, bool) {
	if ctx == nil {
		return nil, false
	}
	tx, ok := ctx.Value(surfaceDBTxKey{}).(pgx.Tx)
	return tx, ok && tx != nil
}

func (s *postgresSurface) Transact(ctx context.Context, fn func(context.Context, surfaceTx) error) error {
	if s == nil || s.pool == nil {
		return errors.New("owner surface store is unavailable")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	txCtx := context.WithValue(ctx, surfaceDBTxKey{}, tx)
	if err := fn(txCtx, postgresSurfaceTx{tx}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (s *postgresSurface) ListKind(ctx context.Context, owner, kind, fid string) ([]surfaceRecord, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("owner surface store is unavailable")
	}
	return listSurfaceRecords(ctx, s.pool, owner, kind, fid, "", "", 200)
}

func (s *postgresSurface) Revision(ctx context.Context, owner, fid, scope string) (int, error) {
	if s == nil || s.pool == nil {
		return 0, errors.New("owner surface store is unavailable")
	}
	var revision int
	err := s.pool.QueryRow(ctx, `SELECT revision FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind='scope' AND resource_key=$3`, owner, fid, scope).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return revision, err
}

type postgresQuery interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func listSurfaceRecords(ctx context.Context, query postgresQuery, owner, kind, fid, status, cursor string, limit int) ([]surfaceRecord, error) {
	cursorAt, cursorID, ok := decodeCursor(cursor)
	if !ok {
		return nil, &SurfaceError{Status: 400, Code: "surface_cursor_invalid", Message: "Cursor is invalid"}
	}
	if limit < 1 {
		limit = 50
	}
	var cursorTime any
	if cursor != "" {
		cursorTime = cursorAt
	}
	rows, err := query.Query(ctx, `
		SELECT id, owner_actor_id, fluctlight_id, kind, resource_key, status, payload, revision, created_at, updated_at
		FROM public.owner_surface_records
		WHERE owner_actor_id=$1 AND kind=$2
		  AND ($3='' OR fluctlight_id=$3)
		  AND ($4='' OR status=$4)
		  AND ($5::timestamptz IS NULL OR created_at < $5 OR (created_at=$5 AND id < $6))
		ORDER BY created_at DESC, id DESC
		LIMIT $7`, owner, kind, fid, status, cursorTime, cursorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSurfaceRecords(rows)
}

type postgresSurfaceTx struct{ tx pgx.Tx }

func (t postgresSurfaceTx) Idempotency(ctx context.Context, owner, key string) (string, map[string]any, bool, error) {
	var digest string
	var raw []byte
	err := t.tx.QueryRow(ctx, `SELECT digest, response FROM public.owner_surface_idempotency WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, key).Scan(&digest, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, false, nil
	}
	if err != nil {
		return "", nil, false, err
	}
	return digest, decodeObject(raw), true, nil
}

func (t postgresSurfaceTx) SaveIdempotency(ctx context.Context, owner, key, digest string, body map[string]any) error {
	_, err := t.tx.Exec(ctx, `INSERT INTO public.owner_surface_idempotency(owner_actor_id, idempotency_key, digest, response) VALUES($1,$2,$3,$4)`, owner, key, digest, jsonBytes(body))
	return err
}

func (t postgresSurfaceTx) Revision(ctx context.Context, owner, fid, scope string) (int, error) {
	var revision int
	err := t.tx.QueryRow(ctx, `SELECT revision FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind='scope' AND resource_key=$3`, owner, fid, scope).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return revision, err
}

func (t postgresSurfaceTx) Bump(ctx context.Context, owner, fid, scope string, expected int) (int, error) {
	current, err := t.Revision(ctx, owner, fid, scope)
	if err != nil {
		return 0, err
	}
	if current != expected {
		return 0, revisionConflictError{Current: current}
	}
	if current == 0 {
		_, err = t.tx.Exec(ctx, `INSERT INTO public.owner_surface_records(id, owner_actor_id, fluctlight_id, kind, resource_key, status, payload, revision) VALUES($1,$2,$3,'scope',$4,'scope','{}'::jsonb,1)`, "scope_"+shortHash(owner+"|"+fid+"|"+scope), owner, fid, scope)
		if err == nil {
			return 1, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, revisionConflictError{Current: current}
		}
		return 0, err
	}
	var revision int
	err = t.tx.QueryRow(ctx, `UPDATE public.owner_surface_records SET revision=revision+1, updated_at=now() WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind='scope' AND resource_key=$3 AND revision=$4 RETURNING revision`, owner, fid, scope, expected).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, revisionConflictError{Current: current}
	}
	return revision, err
}

func (t postgresSurfaceTx) Save(ctx context.Context, rec surfaceRecord) error {
	if rec.ResourceKey == "" {
		rec.ResourceKey = rec.ID
	}
	if rec.Payload == nil {
		rec.Payload = map[string]any{}
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	_, err := t.tx.Exec(ctx, `
		INSERT INTO public.owner_surface_records(id, owner_actor_id, fluctlight_id, kind, resource_key, status, payload, revision, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (owner_actor_id, fluctlight_id, kind, resource_key) DO UPDATE SET
		  id=EXCLUDED.id, status=EXCLUDED.status, payload=EXCLUDED.payload, revision=EXCLUDED.revision, updated_at=EXCLUDED.updated_at`,
		rec.ID, rec.OwnerID, rec.FluctlightID, rec.Kind, rec.ResourceKey, rec.Status, jsonBytes(rec.Payload), rec.Revision, rec.CreatedAt, rec.UpdatedAt)
	return err
}

func (t postgresSurfaceTx) DeleteKey(ctx context.Context, owner, fid, kind, resourceKey string) error {
	_, err := t.tx.Exec(ctx, `DELETE FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind=$3 AND resource_key=$4`, owner, fid, kind, resourceKey)
	return err
}

func (t postgresSurfaceTx) Get(ctx context.Context, owner, id string) (surfaceRecord, bool, error) {
	rows, err := t.tx.Query(ctx, `SELECT id, owner_actor_id, fluctlight_id, kind, resource_key, status, payload, revision, created_at, updated_at FROM public.owner_surface_records WHERE owner_actor_id=$1 AND id=$2`, owner, id)
	if err != nil {
		return surfaceRecord{}, false, err
	}
	defer rows.Close()
	records, err := scanSurfaceRecords(rows)
	if err != nil || len(records) == 0 {
		return surfaceRecord{}, false, err
	}
	return records[0], true, nil
}

func (t postgresSurfaceTx) GetKey(ctx context.Context, owner, fid, kind, resourceKey string) (surfaceRecord, bool, error) {
	rows, err := t.tx.Query(ctx, `SELECT id, owner_actor_id, fluctlight_id, kind, resource_key, status, payload, revision, created_at, updated_at FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind=$3 AND resource_key=$4`, owner, fid, kind, resourceKey)
	if err != nil {
		return surfaceRecord{}, false, err
	}
	defer rows.Close()
	records, err := scanSurfaceRecords(rows)
	if err != nil || len(records) == 0 {
		return surfaceRecord{}, false, err
	}
	return records[0], true, nil
}

func (t postgresSurfaceTx) List(ctx context.Context, owner, kind, fid, status, cursor string, limit int) ([]surfaceRecord, string, error) {
	if limit < 1 {
		limit = 50
	}
	records, err := listSurfaceRecords(ctx, t.tx, owner, kind, fid, status, cursor, limit+1)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(records) > limit {
		last := records[limit-1]
		next = encodeCursor(last.CreatedAt, last.ID)
		records = records[:limit]
	}
	return records, next, nil
}

func (t postgresSurfaceTx) Audit(ctx context.Context, owner, fid, action, reason, recordID string) (string, error) {
	id := randomID("audit_")
	identity, stored := splitAuditAction(action)
	_, err := t.tx.Exec(ctx, `INSERT INTO public.owner_surface_audits(id, owner_actor_id, fluctlight_id, identity, actor, action, reason, record_id) VALUES($1,$2,$3,$4,'owner',$5,$6,$7)`, id, owner, fid, identity, stored, reason, recordID)
	return id, err
}

func scanSurfaceRecords(rows pgx.Rows) ([]surfaceRecord, error) {
	records := make([]surfaceRecord, 0)
	for rows.Next() {
		var rec surfaceRecord
		var raw []byte
		if err := rows.Scan(&rec.ID, &rec.OwnerID, &rec.FluctlightID, &rec.Kind, &rec.ResourceKey, &rec.Status, &raw, &rec.Revision, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		rec.Payload = decodeObject(raw)
		rec.CreatedAt = rec.CreatedAt.UTC()
		rec.UpdatedAt = rec.UpdatedAt.UTC()
		records = append(records, rec)
	}
	return records, rows.Err()
}
