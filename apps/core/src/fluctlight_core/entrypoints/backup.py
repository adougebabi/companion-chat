"""Safe operator entrypoint for creating/verifying a JSON backup manifest."""

from __future__ import annotations

import argparse
import os
from pathlib import Path
from uuid import uuid4

from fluctlight_core.operations.backup import (
    TemporalRestorePlan,
    build_manifest,
    read_manifest,
    verify_manifest,
    write_manifest,
)


def main() -> None:
    parser = argparse.ArgumentParser(prog="fluctlight-backup")
    subparsers = parser.add_subparsers(dest="command", required=True)
    manifest = subparsers.add_parser("manifest")
    manifest.add_argument("path", type=Path)
    verify = subparsers.add_parser("verify")
    verify.add_argument("path", type=Path)
    args = parser.parse_args()
    if args.command == "manifest":
        env = dict(os.environ)
        value = build_manifest(
            manifest_id=f"manifest_{uuid4().hex}",
            schema_revision=env.get("FLUCTLIGHT_SCHEMA_REVISION", "unknown"),
            postgres_snapshot_id=env.get("POSTGRES_SNAPSHOT_ID", "operator-supplied"),
            object_bucket=env.get("S3_BUCKET", "fluctlight-media"),
            objects=(),
            env=env,
            temporal=TemporalRestorePlan(
                "temporal", "temporal_visibility", env.get("TEMPORAL_NAMESPACE", "default")
            ),
            application_version=env.get("FLUCTLIGHT_VERSION", "unknown"),
            notes=(
                "Object entries and PostgreSQL snapshot identity must be supplied by the operator.",
            ),
        )
        write_manifest(args.path, value)
        return
    loaded = read_manifest(args.path)
    issues = verify_manifest(
        loaded,
        expected_schema_revision=loaded.schema_revision,
        observed_object_count=len(loaded.objects),
        required_env_fields=set(),
    )
    if issues:
        raise SystemExit("manifest verification issues: " + ", ".join(issues))
