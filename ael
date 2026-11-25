#!/bin/sh
set -eu
project_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
if [ -e "$project_root/.ael/remote.json" ] || [ -L "$project_root/.ael/remote.json" ]; then
 cli=${AEL_CLI:-}
 remote_config_explicit=false
 if [ -n "${AEL_REMOTE_CONFIG_DIR:-}" ]; then remote_config_explicit=true; fi
 client_config=${AEL_CLIENT_CONFIG_DIR:-}
 if [ -z "$client_config" ]; then
  case "$(uname -s)" in
   Darwin) client_config="$HOME/Library/Application Support/ael/clients" ;;
   *) client_config="${XDG_CONFIG_HOME:-$HOME/.config}/ael/clients" ;;
  esac
 fi
 checkout_key=
 if command -v shasum >/dev/null 2>&1; then
  checkout_key=$(printf '%s' "$project_root" | shasum -a 256 | awk '{print $1}')
 elif command -v sha256sum >/dev/null 2>&1; then
  checkout_key=$(printf '%s' "$project_root" | sha256sum | awk '{print $1}')
 fi
 client_mapping="$client_config/$checkout_key.path"
 if [ -n "$checkout_key" ] && [ -f "$client_mapping" ] && ! [ -L "$client_mapping" ]; then
  if ! {
   IFS= read -r bound_cli
   IFS= read -r bound_config
  } < "$client_mapping"; then echo 'AEL_CLIENT_MAPPING_INVALID: restore the machine installation binding' >&2; exit 1; fi
  if [ -z "$cli" ] && [ -z "${AEL_INSTALL_PREFIX:-}" ]; then cli=$bound_cli; fi
  if [ -z "${AEL_REMOTE_CONFIG_DIR:-}" ]; then export AEL_REMOTE_CONFIG_DIR="$bound_config"; fi
 fi
 if [ -z "$cli" ]; then
  cli="${AEL_INSTALL_PREFIX:-$HOME/.local}/bin/ael"
  if [ ! -x "$cli" ]; then cli=$(command -v ael || true); fi
 fi
 command_name=
 skip_value=false
 for argument in "$@"; do
  if [ "$skip_value" = true ]; then skip_value=false; continue; fi
  case "$argument" in
   --*=*) continue ;;
   --session|--project|--from|--bundle-dir|--prefix|--source|--version|--channel|--config-dir) skip_value=true; continue ;;
   --*) continue ;;
   *) command_name=$argument; break ;;
  esac
 done
 portable=false
 [ ! -f "$project_root/.ael/runner-access.json" ] || portable=true
 if { [ "$command_name" = upgrade ] && [ "$portable" = false ]; } || [ "$command_name" = client ]; then
  if [ -n "$cli" ] && [ -x "$cli" ] && ! [ "$cli" -ef "$project_root/ael" ]; then exec "$cli" "$@"; fi
  [ -f "$project_root/.ael/runner-access.json" ] || { echo 'AEL_MACHINE_CLI_REQUIRED: select the shared machine executable' >&2; exit 1; }
 fi
 session_id=${AEL_SESSION_ID:-}
 session_value=false
 for argument in "$@"; do
  if [ "$session_value" = true ]; then session_id=$argument; session_value=false; continue; fi
  case "$argument" in
   --session) session_value=true ;;
   --session=*) session_id=${argument#--session=} ;;
  esac
 done
 if [ -n "$session_id" ] && [ "$command_name" != update ] && [ "$command_name" != init ]; then
  case "$session_id" in *[!a-zA-Z0-9_-]*) echo 'AEL_SESSION_ID_INVALID: use the host session identifier' >&2; exit 1 ;; esac
  session_mapping="$client_config/sessions/$checkout_key/$session_id.path"
  if [ -n "$checkout_key" ] && [ -f "$session_mapping" ] && ! [ -L "$session_mapping" ]; then
   if ! {
    IFS= read -r session_cli
    IFS= read -r session_config
    IFS= read -r session_record
   } < "$session_mapping"; then echo 'AEL_SESSION_MAPPING_INVALID: restore the session binding' >&2; exit 1; fi
   [ -x "$session_cli" ] && ! [ "$session_cli" -ef "$project_root/ael" ] || { echo 'AEL_SESSION_CLI_UNAVAILABLE: restore the cached session executable' >&2; exit 1; }
   cli=$session_cli
   if [ "$remote_config_explicit" = false ]; then export AEL_REMOTE_CONFIG_DIR="$session_config"; fi
   export AEL_SESSION_ID="$session_id"
   export AEL_PINNED_SESSION_FILE="$session_record"
  fi
 fi
 if [ "$portable" = true ] && [ -z "${AEL_CLI:-}" ] && { [ -z "${AEL_PINNED_SESSION_FILE:-}" ] || [ "$command_name" = update ] || [ "$command_name" = init ]; }; then
  cli=
 fi
 if [ "$portable" = true ] && [ "$command_name" = upgrade ]; then cli=; fi
 if [ -n "$cli" ] && [ -x "$cli" ] && ! [ "$cli" -ef "$project_root/ael" ]; then
  for argument in "$@"; do
   case "$argument" in --project|--project=*) exec "$cli" "$@" ;; esac
  done
  exec "$cli" "$@" --project "$project_root"
 fi
 [ -f "$project_root/.ael/runner-access.json" ] || { echo 'AEL_MACHINE_CLI_REQUIRED: install AEL or configure a portable checkout' >&2; exit 1; }
 lock="$project_root/.ael/clone-runtime/toolchain.lock"
else
 lock="$project_root/.ael/toolchain.lock"
fi
[ -f "$lock" ] || { echo 'AEL_TOOLCHAIN_MISSING: .ael/toolchain.lock is missing' >&2; exit 1; }
case "$(uname -s)" in Darwin) target_os=darwin ;; Linux) target_os=linux ;; *) echo 'AEL_UNSUPPORTED_PLATFORM: this bundle supports macOS and Linux' >&2; exit 1 ;; esac
case "$(uname -m)" in arm64|aarch64) target_arch=arm64 ;; x86_64|amd64) target_arch=amd64 ;; *) echo 'AEL_UNSUPPORTED_ARCHITECTURE' >&2; exit 1 ;; esac
platform="$target_os-$target_arch"
record=$(awk -v platform="$platform" '$1 == platform { print $2 " " $3; count++ } END { if (count != 1) exit 1 }' "$lock") || { echo "AEL_PLATFORM_NOT_BUNDLED: $platform" >&2; exit 1; }
expected=${record%% *}
relative=${record#* }
[ "$relative" = "bin/$platform/ael" ] || { echo 'AEL_INVALID_TOOLCHAIN_PATH' >&2; exit 1; }
[ "${#expected}" -eq 64 ] || { echo 'AEL_INVALID_TOOLCHAIN_HASH' >&2; exit 1; }
case "$expected" in *[!0-9a-f]*) echo 'AEL_INVALID_TOOLCHAIN_HASH' >&2; exit 1 ;; esac
binary="$project_root/.ael/$relative"
if [ "$lock" = "$project_root/.ael/clone-runtime/toolchain.lock" ]; then binary="$project_root/.ael/clone-runtime/$expected/ael"; fi
[ -f "$binary" ] && [ ! -L "$binary" ] || { echo "AEL_BINARY_MISSING: $relative" >&2; exit 1; }
if command -v shasum >/dev/null 2>&1; then
 actual=$(shasum -a 256 "$binary" | awk '{print $1}')
elif command -v sha256sum >/dev/null 2>&1; then
 actual=$(sha256sum "$binary" | awk '{print $1}')
else
 echo 'AEL_SHA256_UNAVAILABLE: shasum or sha256sum is required' >&2; exit 1
fi
[ "$actual" = "$expected" ] || { echo "AEL_BINARY_CHECKSUM_MISMATCH: $relative" >&2; exit 1; }
[ -x "$binary" ] || chmod u+x "$binary"
command_name=
skip_value=false
for argument in "$@"; do
 if [ "$skip_value" = true ]; then skip_value=false; continue; fi
 case "$argument" in
  --*=*) continue ;;
  --project|--from|--bundle-dir|--prefix) skip_value=true; continue ;;
  --*) continue ;;
  *) command_name=$argument; break ;;
 esac
done
if { [ "$command_name" = upgrade ] && [ "${portable:-false}" = false ]; } || [ "$command_name" = client ]; then exec "$binary" "$@"; fi
for argument in "$@"; do
 case "$argument" in --project|--project=*) exec "$binary" "$@" ;; esac
done
exec "$binary" "$@" --project "$project_root"
