from __future__ import annotations

import pytest
from fluctlight_core.temporal_gate.history import (
    SavedHistory,
    load_history,
    replay_compatible,
    replay_with_sdk,
    save_history,
)
from fluctlight_core.temporal_gate.models import WorkerDeploymentVersion
from fluctlight_core.temporal_gate.versioning import DeploymentRouter


def test_temporal_sdk_replayer_public_import_and_helper_are_available() -> None:
    from temporalio.worker import Replayer

    assert callable(Replayer)
    assert callable(replay_with_sdk)


def test_saved_history_round_trips_and_routes_by_compatible_version(tmp_path) -> None:
    history = SavedHistory(
        workflow_id="wf_123",
        run_id="run_123",
        history_version="gate-v1",
        events=(
            {"event_type": "workflow_started", "event_id": 1},
            {"event_type": "activity_completed", "event_id": 2},
        ),
    )
    path = tmp_path / "history.json"
    save_history(history, path)

    assert load_history(path) == history
    assert replay_compatible(history, WorkerDeploymentVersion(version="gate-v2")) is True
    assert (
        replay_compatible(
            history, WorkerDeploymentVersion(version="gate-v2", compatible_history_versions=())
        )
        is False
    )
    assert (
        replay_compatible(
            history,
            WorkerDeploymentVersion(
                version="gate-v2", compatible_history_versions=("gate-v1", "gate-v2")
            ),
        )
        is True
    )


def test_deployment_router_supports_coexist_drain_and_rollback() -> None:
    router = DeploymentRouter()
    assert router.route("gate-v1").version == "gate-v1"

    deployed = router.deploy("gate-v2", ("gate-v1", "gate-v2"))
    assert deployed.version == "gate-v2"
    assert "gate-v1" in router.draining
    assert router.route("gate-v1") is deployed
    router.drain("gate-v1")
    assert "gate-v1" not in router.draining

    rolled_back = router.rollback("gate-v1", ("gate-v1", "gate-v2"))
    assert rolled_back.version == "gate-v1"
    assert router.rolled_back == ["gate-v2"]

    with pytest.raises(ValueError, match="no compatible"):
        router.route("gate-v0")


def test_worker_deployment_version_is_explicitly_compatible_or_incompatible() -> None:
    deployment = WorkerDeploymentVersion(
        deployment_name="fluctlight-gate",
        version="gate-v2",
        compatible_history_versions=("gate-v1", "gate-v2"),
    )

    assert deployment.deployment_name == "fluctlight-gate"
    assert deployment.can_replay("gate-v1")
    assert deployment.can_replay("gate-v2")
    assert not deployment.can_replay("gate-v0")
