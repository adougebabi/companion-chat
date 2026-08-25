#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

project="fluctlight-t12-backup-$$"
compose_file="infra/compose/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-infra/compose/fluctlight.env.example}"
compose=(docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file")
source "$(dirname "$0")/read-compose-env.sh"
db_user="$(printenv POSTGRES_USER || true)"
if [[ -z "$db_user" ]]; then db_user="$(compose_env_value POSTGRES_USER)"; fi
db_name="$(printenv POSTGRES_DB || true)"
if [[ -z "$db_name" ]]; then db_name="$(compose_env_value POSTGRES_DB)"; fi
bucket="$(printenv S3_BUCKET || true)"
if [[ -z "$bucket" ]]; then bucket="$(compose_env_value S3_BUCKET)"; fi
assert_disposable_compose_project "$project"
compose_started=0

cleanup() {
  if [[ "$compose_started" != 1 ]]; then return; fi
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

compose_started=1
"${compose[@]}" up --build --detach
for _ in $(seq 1 60); do
  if "${compose[@]}" exec -T core python -c 'import urllib.request; urllib.request.urlopen("http://127.0.0.1:8080/health/ready")' >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

"${compose[@]}" exec -T postgres psql -U "$db_user" -d postgres -c 'CREATE DATABASE restore_check' >/dev/null
"${compose[@]}" exec -T postgres pg_dump -U "$db_user" -d "$db_name" | "${compose[@]}" exec -T postgres psql -U "$db_user" -d restore_check >/dev/null
restored_tables=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d restore_check -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'")
if [[ "${restored_tables//[[:space:]]/}" -lt 1 ]]; then
  echo "restored application database has no public tables" >&2
  exit 1
fi
source_tables=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name")
restored_table_names=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d restore_check -Atc "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name")
if [[ "$source_tables" != "$restored_table_names" ]]; then
  echo "application restore table set mismatch" >&2
  exit 1
fi
count_query=$(printf '%s\n' "$source_tables" | awk 'NF {printf "SELECT '\''%s'\'' AS table_name, count(*)::bigint AS row_count FROM public.%s UNION ALL ", $0, $0}' | sed 's/ UNION ALL $//')
source_counts=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc "$count_query" | sort)
restored_counts=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d restore_check -Atc "$count_query" | sort)
if [[ "$source_counts" != "$restored_counts" ]]; then
  echo "application restore table row counts mismatch" >&2
  exit 1
fi
schema_revision=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc "SELECT version_num FROM alembic_version" | tr -d '[:space:]')
if [[ "$schema_revision" != "0012_t12_consumer_effects" ]]; then
  echo "unexpected application migration head: $schema_revision" >&2
  exit 1
fi
vector_version=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc "SELECT extversion FROM pg_extension WHERE extname = 'vector'")
if [[ -z "${vector_version//[[:space:]]/}" ]]; then
  echo "pgvector extension is not installed in the application database" >&2
  exit 1
fi
vector_type=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc "SELECT udt_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'memory_embeddings' AND column_name = 'embedding_vector'" | tr -d '[:space:]')
fts_index_count=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc "SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = 'ix_memories_search_document'" | tr -d '[:space:]')
if [[ "$vector_type" != "vector" || "$fts_index_count" != "1" ]]; then
  echo "pgvector/FTS schema gate failed: vector_type=$vector_type fts_index_count=$fts_index_count" >&2
  exit 1
fi

"${compose[@]}" exec -T postgres psql -U "$db_user" -d postgres -c 'CREATE DATABASE temporal_restore_check' >/dev/null
"${compose[@]}" exec -T postgres psql -U "$db_user" -d postgres -c 'CREATE DATABASE temporal_visibility_restore_check' >/dev/null
"${compose[@]}" exec -T postgres pg_dump -U "$db_user" -d temporal | "${compose[@]}" exec -T postgres psql -U "$db_user" -d temporal_restore_check >/dev/null
"${compose[@]}" exec -T postgres pg_dump -U "$db_user" -d temporal_visibility | "${compose[@]}" exec -T postgres psql -U "$db_user" -d temporal_visibility_restore_check >/dev/null
temporal_tables=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d temporal_restore_check -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'" | tr -d '[:space:]')
visibility_tables=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d temporal_visibility_restore_check -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'" | tr -d '[:space:]')
if [[ "$temporal_tables" -lt 1 || "$visibility_tables" -lt 1 ]]; then
  echo "Temporal database restore checks did not restore public tables" >&2
  exit 1
fi

printf 't12-backup-object' | "${compose[@]}" run --rm -T minio-init mc pipe "local/$bucket/t12-backup-object.txt" >/dev/null
"${compose[@]}" run --rm minio-init mc mb --ignore-existing --with-versioning "local/$bucket-restore" >/dev/null
"${compose[@]}" run --rm minio-init mc cp "local/$bucket/t12-backup-object.txt" "local/$bucket-restore/t12-backup-object.txt" >/dev/null
source_object=$("${compose[@]}" run --rm minio-init mc cat "local/$bucket/t12-backup-object.txt")
restored_object=$("${compose[@]}" run --rm minio-init mc cat "local/$bucket-restore/t12-backup-object.txt")
if [[ "$source_object" != "$restored_object" ]]; then
  echo "MinIO restore object content mismatch" >&2
  exit 1
fi

"${compose[@]}" exec -T core uv run --no-sync fluctlight-backup manifest /tmp/t12-manifest.json
"${compose[@]}" exec -T core uv run --no-sync fluctlight-backup verify /tmp/t12-manifest.json
echo "backup-restore-check: head=$schema_revision application tables=${restored_tables//[[:space:]]/} and row counts matched; pgvector=${vector_version//[[:space:]]/}/$vector_type with FTS GIN; temporal_tables=$temporal_tables visibility_tables=$visibility_tables; MinIO object restore and manifest verify passed"
