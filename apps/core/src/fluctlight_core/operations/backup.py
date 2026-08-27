"""Backup/restore manifest contracts without persisting secret values."""

from __future__ import annotations

import hashlib
import json
from dataclasses import asdict, dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any


@dataclass(frozen=True, slots=True)
class ObjectManifestEntry:
    key: str
    version_id: str | None
    byte_size: int
    sha256: str


@dataclass(frozen=True, slots=True)
class TemporalRestorePlan:
    default_database: str
    visibility_database: str
    namespace: str
    active_workflow_ids: tuple[str, ...] = ()


@dataclass(frozen=True, slots=True)
class BackupManifest:
    manifest_id: str
    created_at: datetime
    schema_revision: str
    postgres_snapshot_id: str
    object_bucket: str
    objects: tuple[ObjectManifestEntry, ...]
    env_fields_present: tuple[str, ...]
    settings_key_present: bool
    temporal: TemporalRestorePlan
    application_version: str
    notes: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        if self.created_at.tzinfo is None or self.created_at.utcoffset() is None:
            raise ValueError("manifest created_at must be timezone-aware")
        if not self.schema_revision or not self.postgres_snapshot_id or not self.object_bucket:
            raise ValueError("manifest authority fields are required")
        if not self.application_version:
            raise ValueError("application_version is required")

    def as_dict(self) -> dict[str, Any]:
        payload = asdict(self)
        payload["created_at"] = self.created_at.isoformat()
        payload["objects"] = [asdict(item) for item in self.objects]
        payload["temporal"] = asdict(self.temporal)
        return payload

    @classmethod
    def from_dict(cls, payload: dict[str, Any]) -> BackupManifest:
        return cls(
            manifest_id=str(payload["manifest_id"]),
            created_at=datetime.fromisoformat(payload["created_at"]),
            schema_revision=str(payload["schema_revision"]),
            postgres_snapshot_id=str(payload["postgres_snapshot_id"]),
            object_bucket=str(payload["object_bucket"]),
            objects=tuple(ObjectManifestEntry(**item) for item in payload.get("objects", [])),
            env_fields_present=tuple(payload.get("env_fields_present", [])),
            settings_key_present=bool(payload.get("settings_key_present")),
            temporal=TemporalRestorePlan(**payload["temporal"]),
            application_version=str(payload["application_version"]),
            notes=tuple(payload.get("notes", [])),
        )


@dataclass(frozen=True, slots=True)
class CleanupPlan:
    diagnostic_cutoff: datetime | None
    orphan_asset_ids: tuple[str, ...]
    tombstoned_asset_ids: tuple[str, ...]
    dry_run: bool = True


def build_manifest(
    *,
    manifest_id: str,
    schema_revision: str,
    postgres_snapshot_id: str,
    object_bucket: str,
    objects: tuple[ObjectManifestEntry, ...],
    env: dict[str, str],
    temporal: TemporalRestorePlan,
    application_version: str,
    notes: tuple[str, ...] = (),
) -> BackupManifest:
    fields = tuple(sorted(key for key, value in env.items() if value.strip()))
    return BackupManifest(
        manifest_id=manifest_id,
        created_at=datetime.now(UTC),
        schema_revision=schema_revision,
        postgres_snapshot_id=postgres_snapshot_id,
        object_bucket=object_bucket,
        objects=objects,
        env_fields_present=fields,
        settings_key_present=bool(env.get("FLUCTLIGHT_SETTINGS_KEY", "").strip()),
        temporal=temporal,
        application_version=application_version,
        notes=notes,
    )


def verify_manifest(
    manifest: BackupManifest,
    *,
    expected_schema_revision: str,
    observed_object_count: int,
    required_env_fields: set[str],
) -> tuple[str, ...]:
    issues: list[str] = []
    if manifest.schema_revision != expected_schema_revision:
        issues.append("schema_revision_mismatch")
    if observed_object_count != len(manifest.objects):
        issues.append("object_count_mismatch")
    missing = sorted(required_env_fields - set(manifest.env_fields_present))
    if missing:
        issues.append(f"missing_env_fields:{','.join(missing)}")
    if not manifest.settings_key_present:
        issues.append("settings_key_missing_reentry_required")
    return tuple(issues)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def write_manifest(path: Path, manifest: BackupManifest) -> None:
    path.write_text(
        json.dumps(manifest.as_dict(), ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )


def read_manifest(path: Path) -> BackupManifest:
    return BackupManifest.from_dict(json.loads(path.read_text(encoding="utf-8")))
