#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"
project="fluctlight-t12-pgvector-$$"
compose_file="infra/compose/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-infra/compose/fluctlight.env.example}"
compose=(docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file")
source "$script_dir/read-compose-env.sh"
db_user="$(printenv POSTGRES_USER || true)"
if [[ -z "$db_user" ]]; then db_user="$(compose_env_value POSTGRES_USER)"; fi
db_name="$(printenv POSTGRES_DB || true)"
if [[ -z "$db_name" ]]; then db_name="$(compose_env_value POSTGRES_DB)"; fi
assert_disposable_compose_project "$project"
compose_started=0

cleanup() {
  if [[ "$compose_started" != 1 ]]; then return; fi
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

compose_started=1
"${compose[@]}" up --build --detach
ready=0
for _ in $(seq 1 60); do
  if "${compose[@]}" exec -T core python -c 'import urllib.request; urllib.request.urlopen("http://127.0.0.1:8080/health/ready")' >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
if [[ "$ready" != 1 ]]; then
  "${compose[@]}" ps
  exit 1
fi

"${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE vector_benchmark_t12 (id integer PRIMARY KEY, embedding vector(3) NOT NULL);
INSERT INTO vector_benchmark_t12 (id, embedding)
SELECT i, ARRAY[
  sin(i::double precision),
  cos(i::double precision),
  (i % 100)::double precision / 100.0
]::vector(3)
FROM generate_series(1, 2000) AS series(i);
CREATE INDEX vector_benchmark_t12_hnsw
  ON vector_benchmark_t12 USING hnsw (embedding vector_cosine_ops);
ANALYZE vector_benchmark_t12;
SQL

plan=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc \
  "SET enable_seqscan=off; EXPLAIN (ANALYZE, FORMAT TEXT)
   SELECT id FROM vector_benchmark_t12
   ORDER BY embedding <=> '[0.1,0.2,0.3]'::vector
   LIMIT 10")
if ! printf '%s\n' "$plan" | grep -qiE 'Index Scan using vector_benchmark_t12_hnsw'; then
  echo "$plan" >&2
  echo "HNSW benchmark did not use the expected index" >&2
  exit 1
fi
latency=$(printf '%s\n' "$plan" | grep -oE 'Execution Time: [0-9.]+ ms' | tail -n 1)
echo "pgvector-benchmark: HNSW index scan passed; $latency"
