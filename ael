#!/bin/sh
set -eu
project_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
lock="$project_root/.ael/toolchain.lock"
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
for argument in "$@"; do
 case "$argument" in --project|--project=*) exec "$binary" "$@" ;; esac
done
exec "$binary" "$@" --project "$project_root"
