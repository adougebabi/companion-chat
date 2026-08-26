#!/usr/bin/env bash

compose_env_value() {
  local name="$1"
  awk -F= -v name="$name" '
    $1 == name {
      sub(/^[^=]*=/, "")
      print
      exit
    }
  ' "$env_file"
}

assert_disposable_compose_project() {
  local project_name="$1"
  case "$project_name" in
    *-[0-9]*) ;;
    *)
      echo "refusing non-unique disposable Compose project: $project_name" >&2
      return 2
      ;;
  esac
  if docker ps -aq --filter "name=^${project_name}-" | grep -q .; then
    echo "refusing to reuse existing Compose containers for $project_name" >&2
    return 2
  fi
  if docker volume ls -q --filter "name=^${project_name}_" | grep -q .; then
    echo "refusing to reuse existing Compose volumes for $project_name" >&2
    return 2
  fi
  if docker network ls -q --filter "name=^${project_name}_" | grep -q .; then
    echo "refusing to reuse existing Compose networks for $project_name" >&2
    return 2
  fi
}

# Disposable validation projects must not contend with the user's long-running
# local/NAS BFF and Web ports. Service-to-service checks use the Compose network.
export BFF_HOST_PORT="${BFF_HOST_PORT:-0}"
export WEB_HOST_PORT="${WEB_HOST_PORT:-0}"
