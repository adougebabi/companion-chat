"""Start one Python Worker process with the three approved Temporal task queues."""

from __future__ import annotations

import asyncio
import signal

from temporalio.client import Client
from temporalio.common import VersioningBehavior
from temporalio.worker import Worker, WorkerDeploymentConfig, WorkerDeploymentVersion

from fluctlight_core.platform.configuration import PlatformSettings, RuntimeRole
from fluctlight_core.platform.temporal import TASK_QUEUES
from fluctlight_core.platform.workflows import PlatformControlWorkflow


def deployment_config() -> WorkerDeploymentConfig:
    return WorkerDeploymentConfig(
        version=WorkerDeploymentVersion(deployment_name="fluctlight", build_id="platform-v1"),
        use_worker_versioning=True,
        default_versioning_behavior=VersioningBehavior.AUTO_UPGRADE,
    )


async def run_worker(settings: PlatformSettings) -> None:
    settings.require_role(RuntimeRole.WORKER)
    client = await Client.connect(settings.temporal_address, namespace=settings.temporal_namespace)
    workers = [
        Worker(
            client,
            task_queue=queue,
            workflows=[PlatformControlWorkflow],
            max_concurrent_workflow_tasks=1 if queue != "interaction" else 2,
            max_cached_workflows=50,
            deployment_config=deployment_config(),
        )
        for queue in TASK_QUEUES
    ]
    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    for signal_number in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(signal_number, stop.set)
    tasks = [asyncio.create_task(worker.run()) for worker in workers]
    await stop.wait()
    for task in tasks:
        task.cancel()
    await asyncio.gather(*tasks, return_exceptions=True)


def main() -> None:
    asyncio.run(run_worker(PlatformSettings.from_environ()))
