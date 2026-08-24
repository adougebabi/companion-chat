"""Current Worker Deployment Versioning evidence and rollback fixtures."""

from __future__ import annotations

from dataclasses import dataclass, field

from .models import WorkerDeploymentVersion


@dataclass(slots=True)
class DeploymentRouter:
    """Small evidence model for coexist, drain, and rollback decisions."""

    active: WorkerDeploymentVersion = field(default_factory=WorkerDeploymentVersion)
    draining: set[str] = field(default_factory=set)
    rolled_back: list[str] = field(default_factory=list)

    def route(self, history_version: str) -> WorkerDeploymentVersion:
        if self.active.can_replay(history_version):
            return self.active
        raise ValueError(f"no compatible Worker Deployment Version for {history_version}")

    def deploy(
        self, version: str, compatible_history_versions: tuple[str, ...]
    ) -> WorkerDeploymentVersion:
        previous = self.active
        self.active = WorkerDeploymentVersion(
            deployment_name=previous.deployment_name,
            version=version,
            compatible_history_versions=compatible_history_versions,
        )
        self.draining.add(previous.version)
        return self.active

    def drain(self, version: str) -> None:
        self.draining.discard(version)

    def rollback(
        self, version: str, compatible_history_versions: tuple[str, ...]
    ) -> WorkerDeploymentVersion:
        self.rolled_back.append(self.active.version)
        self.active = WorkerDeploymentVersion(
            self.active.deployment_name, version, compatible_history_versions
        )
        return self.active
