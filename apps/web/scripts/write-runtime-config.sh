#!/bin/sh
set -eu

origin="${FLUCTLIGHT_API_ORIGIN:-}"

# Keep this value safe to embed in a JavaScript string. The public origin is
# expected to be an HTTP(S) URL. The Go API performs the authoritative origin
# validation for requests.
if [ -n "$origin" ] && ! printf '%s' "$origin" | grep -Eq '^https?://[^"\\[:space:]]+$'; then
  echo "FLUCTLIGHT_API_ORIGIN must be an HTTP(S) URL without quotes or control characters" >&2
  exit 1
fi

escaped_origin=$(printf '%s' "$origin" | sed 's/[\\"]/\\\\&/g')
printf 'window.__FLUCTLIGHT_RUNTIME_CONFIG__ = Object.freeze({ apiOrigin: "%s" });\n' "$escaped_origin" \
  > /usr/share/nginx/html/runtime-config.js
