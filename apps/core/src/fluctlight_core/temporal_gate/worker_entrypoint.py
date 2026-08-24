"""Python Worker process polling the three independent Temporal queues."""

from __future__ import annotations

import asyncio
import os
import signal

from temporalio.api.workflowservice.v1 import (
    DescribeWorkerDeploymentRequest,
    SetWorkerDeploymentCurrentVersionRequest,
)
from temporalio.client import Client
from temporalio.common import VersioningBehavior
from temporalio.worker import Worker, WorkerDeploymentConfig, WorkerDeploymentVersion

from .queues import QUEUE_POLICIES
from .temporal_workflows import (
    GateWorkflow,
    fake_h3_activity,
    persist_gate_result_activity,
)


def deployment_config() -> WorkerDeploymentConfig:
    return WorkerDeploymentConfig(
        version=WorkerDeploymentVersion(
            deployment_name=os.environ.get("TEMPORAL_WORKER_DEPLOYMENT_NAME", "fluctlight-gate"),
            build_id=os.environ.get("TEMPORAL_WORKER_DEPLOYMENT_VERSION", "gate-v1"),
        ),
        use_worker_versioning=True,
        default_versioning_behavior=VersioningBehavior.AUTO_UPGRADE,
    )


async def activate_deployment_version(client: Client) -> None:
    deployment_name = os.environ.get("TEMPORAL_WORKER_DEPLOYMENT_NAME", "fluctlight-gate")
    build_id = os.environ.get("TEMPORAL_WORKER_DEPLOYMENT_VERSION", "gate-v1")
    description = await client.workflow_service.describe_worker_deployment(
        DescribeWorkerDeploymentRequest(
            namespace=client.namespace,
            deployment_name=deployment_name,
        )
    )
    version = f"{deployment_name}.{build_id}"
    await client.workflow_service.set_worker_deployment_current_version(
        SetWorkerDeploymentCurrentVersionRequest(
            namespace=client.namespace,
            deployment_name=deployment_name,
            version=version,
            conflict_token=description.conflict_token,
            identity=f"gate-worker:{build_id}",
        )
    )


async def run_worker() -> None:
    client = await Client.connect(
        os.environ.get("TEMPORAL_ADDRESS", "temporal:7233"),
        namespace=os.environ.get("TEMPORAL_NAMESPACE", "default"),
    )
    workers = [
        Worker(
            client,
            task_queue=queue,
            workflows=[GateWorkflow],
            activities=[fake_h3_activity, persist_gate_result_activity],
            max_concurrent_workflow_tasks=QUEUE_POLICIES[queue].concurrency,
            max_concurrent_activities=QUEUE_POLICIES[queue].concurrency,
            max_cached_workflows=50,
            deployment_config=deployment_config(),
        )
        for queue in QUEUE_POLICIES
    ]
    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    for signum in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(signum, stop.set)
    tasks = [asyncio.create_task(worker.run()) for worker in workers]
    await asyncio.sleep(1)
    await activate_deployment_version(client)
    await stop.wait()
    for task in tasks:
        task.cancel()
    await asyncio.gather(*tasks, return_exceptions=True)


def main() -> None:
    asyncio.run(run_worker())


if __name__ == "__main__":
    main()
