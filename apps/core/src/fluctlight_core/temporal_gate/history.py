"""Saved Event History replay helpers for the v1 to v2 gate."""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from .models import WorkerDeploymentVersion


@dataclass(frozen=True, slots=True)
class SavedHistory:
    workflow_id: str
    run_id: str
    history_version: str
    events: tuple[dict[str, Any], ...]

    def as_dict(self) -> dict[str, Any]:
        return {
            "workflow_id": self.workflow_id,
            "run_id": self.run_id,
            "history_version": self.history_version,
            "events": list(self.events),
        }


def save_history(history: SavedHistory, path: str | Path) -> None:
    Path(path).write_text(json.dumps(history.as_dict(), indent=2, sort_keys=True) + "\n")


def load_history(path: str | Path) -> SavedHistory:
    payload = json.loads(Path(path).read_text())
    return SavedHistory(
        workflow_id=payload["workflow_id"],
        run_id=payload["run_id"],
        history_version=payload["history_version"],
        events=tuple(payload["events"]),
    )


def replay_compatible(history: SavedHistory, deployment: WorkerDeploymentVersion) -> bool:
    """Check routing before invoking the SDK Replayer in a live gate."""

    return deployment.can_replay(history.history_version)


async def replay_with_sdk(history_payload: dict[str, Any]) -> None:
    """Replay a real exported history with the Python SDK Replayer.

    The server exports the exact protobuf history; this helper accepts the SDK
    history object or a JSON fixture supplied by the runner and keeps replay a
    separate, explicit gate from normal Worker startup.
    """

    from temporalio.worker import Replayer

    from .temporal_workflows import GateWorkflow

    history = history_payload.get("history", history_payload)
    await Replayer(workflows=[GateWorkflow]).replay_workflow(history)
