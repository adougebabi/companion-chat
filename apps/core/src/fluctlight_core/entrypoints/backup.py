"""Safe operator entrypoint for creating/verifying a JSON backup manifest."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
from uuid import uuid4

from fluctlight_core.operations.backup import (
    ObjectManifestEntry,
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
    manifest.add_argument("--object-inventory", type=Path)
    manifest.add_argument("--active-workflow-id", action="append", default=[])
    verify = subparsers.add_parser("verify")
    verify.add_argument("path", type=Path)
    verify.add_argument("--expected-schema-revision")
    verify.add_argument("--observed-object-count", type=int)
    verify.add_argument("--required-env-field", action="append", default=[])
    args = parser.parse_args()
    if args.command == "manifest":
        env = dict(os.environ)
        objects: tuple[ObjectManifestEntry, ...] = ()
        if args.object_inventory is not None:
            payload = json.loads(args.object_inventory.read_text(encoding="utf-8"))
            if not isinstance(payload, list):
                raise SystemExit("object inventory must be a JSON array")
            objects = tuple(ObjectManifestEntry(**item) for item in payload)
        value = build_manifest(
            manifest_id=f"manifest_{uuid4().hex}",
            schema_revision=env.get("FLUCTLIGHT_SCHEMA_REVISION", "unknown"),
            postgres_snapshot_id=env.get("POSTGRES_SNAPSHOT_ID", "operator-supplied"),
            object_bucket=env.get("S3_BUCKET", "fluctlight-media"),
            objects=objects,
            env=env,
            temporal=TemporalRestorePlan(
                "temporal",
                "temporal_visibility",
                env.get("TEMPORAL_NAMESPACE", "default"),
                tuple(args.active_workflow_id),
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
        expected_schema_revision=args.expected_schema_revision or loaded.schema_revision,
        observed_object_count=(
            args.observed_object_count
            if args.observed_object_count is not None
            else len(loaded.objects)
        ),
        required_env_fields=set(args.required_env_field),
    )
    if issues:
        raise SystemExit("manifest verification issues: " + ", ".join(issues))
