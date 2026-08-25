from pathlib import Path

from fluctlight_core.operations.backup import (
    ObjectManifestEntry,
    TemporalRestorePlan,
    build_manifest,
    read_manifest,
    sha256_file,
    verify_manifest,
    write_manifest,
)


def test_manifest_round_trip_records_presence_not_secret_values(tmp_path: Path) -> None:
    manifest = build_manifest(
        manifest_id="manifest-1",
        schema_revision="0012_t12_consumer_effects",
        postgres_snapshot_id="snapshot-1",
        object_bucket="fluctlight-media",
        objects=(ObjectManifestEntry("media/a/v1", "v1", 3, "sha"),),
        env={"DATABASE_URL": "postgres://...", "FLUCTLIGHT_SETTINGS_KEY": "secret"},
        temporal=TemporalRestorePlan("temporal", "temporal_visibility", "default", ("workflow-1",)),
        application_version="0.1.0",
    )
    path = tmp_path / "manifest.json"
    write_manifest(path, manifest)
    loaded = read_manifest(path)
    assert loaded.objects[0].key == "media/a/v1"
    assert "secret" not in path.read_text()
    assert (
        verify_manifest(
            loaded,
            expected_schema_revision="0012_t12_consumer_effects",
            observed_object_count=1,
            required_env_fields={"DATABASE_URL"},
        )
        == ()
    )


def test_manifest_reports_missing_components_and_file_hash(tmp_path: Path) -> None:
    path = tmp_path / "dump.sql"
    path.write_text("database")
    assert len(sha256_file(path)) == 64
    manifest = build_manifest(
        manifest_id="manifest-1",
        schema_revision="head",
        postgres_snapshot_id="snapshot-1",
        object_bucket="bucket",
        objects=(),
        env={},
        temporal=TemporalRestorePlan("temporal", "visibility", "default"),
        application_version="0.1.0",
    )
    assert "missing_env_fields:DATABASE_URL" in verify_manifest(
        manifest,
        expected_schema_revision="other",
        observed_object_count=1,
        required_env_fields={"DATABASE_URL"},
    )


def test_manifest_keeps_operator_supplied_object_and_active_workflow_inventory() -> None:
    manifest = build_manifest(
        manifest_id="manifest-2",
        schema_revision="0012_t12_consumer_effects",
        postgres_snapshot_id="snapshot-2",
        object_bucket="bucket",
        objects=(ObjectManifestEntry("media/a/v1", "v1", 3, "a" * 64),),
        env={"DATABASE_URL": "postgres://...", "FLUCTLIGHT_SETTINGS_KEY": "configured"},
        temporal=TemporalRestorePlan("temporal", "visibility", "default", ("workflow-active-1",)),
        application_version="0.1.0",
    )

    assert manifest.objects[0].sha256 == "a" * 64
    assert manifest.temporal.active_workflow_ids == ("workflow-active-1",)
