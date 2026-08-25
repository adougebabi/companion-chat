"""Start one Python Worker process with the three approved Temporal task queues."""

from __future__ import annotations

import asyncio
import logging
import os
import signal
from hashlib import sha256

import boto3  # type: ignore[import-untyped]
from redis.asyncio import Redis
from sqlalchemy import insert
from temporalio.api.workflowservice.v1 import (
    DescribeWorkerDeploymentRequest,
    SetWorkerDeploymentCurrentVersionRequest,
)
from temporalio.client import Client
from temporalio.common import VersioningBehavior
from temporalio.worker import Worker, WorkerDeploymentConfig, WorkerDeploymentVersion

from fluctlight_core.actors.service import AuthService
from fluctlight_core.autonomy import AutonomyExecutor, AutonomyService
from fluctlight_core.autonomy.workflows import (
    AutonomyActionWorkflow,
    configure_autonomy_service,
    process_autonomy_action,
)
from fluctlight_core.cognition.service import CognitionService
from fluctlight_core.cognition.workflows import (
    CognitionProcessingWorkflow,
    configure_cognition_service,
    process_cognition,
)
from fluctlight_core.conversations.service import ConversationService
from fluctlight_core.diagnostics.service import DiagnosticsService
from fluctlight_core.inner_state import CognitionStateApplier, InnerStateService
from fluctlight_core.life_world.service import LifeWorldService
from fluctlight_core.media.service import MediaService
from fluctlight_core.media.workflows import (
    MediaGenerationWorkflow,
    configure_media_service,
    process_media_generation,
)
from fluctlight_core.memory.service import MemoryService
from fluctlight_core.memory.workflows import (
    MemoryEmbeddingWorkflow,
    configure_embedding_service,
    process_memory_embedding,
)
from fluctlight_core.moments.service import MomentsService
from fluctlight_core.platform import schema as platform_schema
from fluctlight_core.platform.configuration import PlatformSettings, RuntimeRole
from fluctlight_core.platform.dispatcher import CommittedIntentDispatcher
from fluctlight_core.platform.object_storage import S3ObjectStorage
from fluctlight_core.platform.persistence import UnitOfWorkFactory, create_engine, verify_revision
from fluctlight_core.platform.redis_streams import DURABLE_CONSUMER_GROUPS, RedisStreams
from fluctlight_core.platform.temporal import TASK_QUEUES
from fluctlight_core.platform.workflows import PlatformControlWorkflow
from fluctlight_core.providers.adapters import OpenAICompatibleAdapter
from fluctlight_core.providers.runtime import ConfiguredProviderRuntime
from fluctlight_core.providers.service import ProviderConfigurationService
from fluctlight_core.reflection.service import ReflectionCoordinator
from fluctlight_core.relationships.service import RelationshipService
from fluctlight_core.settings.crypto import SecretCodec
from fluctlight_core.settings.service import SettingsService

EXPECTED_REVISION = "0012_t12_consumer_effects"
logger = logging.getLogger(__name__)


def deployment_config(build_id: str | None = None) -> WorkerDeploymentConfig:
    resolved_build_id = build_id or os.environ.get("FLUCTLIGHT_BUILD_ID", "platform-v1")
    return WorkerDeploymentConfig(
        version=WorkerDeploymentVersion(deployment_name="fluctlight", build_id=resolved_build_id),
        use_worker_versioning=True,
        default_versioning_behavior=VersioningBehavior.AUTO_UPGRADE,
    )


async def bootstrap_streams_with_retry(
    streams: RedisStreams, *, attempts: int = 30, delay_seconds: float = 1.0
) -> None:
    last_error: Exception | None = None
    for attempt in range(attempts):
        try:
            await streams.bootstrap_groups()
            return
        except Exception as exc:
            last_error = exc
            if attempt + 1 == attempts:
                break
            try:
                await streams.client.connection_pool.disconnect(inuse_connections=True)
            except Exception:
                pass
            await asyncio.sleep(delay_seconds)
    raise RuntimeError("Redis stream group bootstrap failed") from last_error


async def activate_deployment_version(client: Client, build_id: str | None = None) -> None:
    deployment_name = "fluctlight"
    build_id = build_id or os.environ.get("FLUCTLIGHT_BUILD_ID", "platform-v1")
    description = await client.workflow_service.describe_worker_deployment(
        DescribeWorkerDeploymentRequest(
            namespace=client.namespace,
            deployment_name=deployment_name,
        )
    )
    await client.workflow_service.set_worker_deployment_current_version(
        SetWorkerDeploymentCurrentVersionRequest(
            namespace=client.namespace,
            deployment_name=deployment_name,
            version=f"{deployment_name}.{build_id}",
            conflict_token=description.conflict_token,
            identity=f"fluctlight-worker:{build_id}",
        )
    )


async def run_worker(settings: PlatformSettings) -> None:
    settings.require_role(RuntimeRole.WORKER)
    build_id = os.environ.get("FLUCTLIGHT_BUILD_ID", "platform-v1")
    engine = create_engine(settings.database_url)
    await verify_revision(engine, EXPECTED_REVISION)
    unit_of_work = UnitOfWorkFactory(engine)
    auth = AuthService(unit_of_work)
    settings_service = SettingsService(unit_of_work, SecretCodec(settings.settings_key), auth)
    provider_adapter = OpenAICompatibleAdapter()
    provider_service = ProviderConfigurationService(
        unit_of_work,
        auth,
        settings_service,
        provider_adapter.preflight,
    )
    provider_runtime = ConfiguredProviderRuntime(
        unit_of_work,
        settings_service,
        adapter=provider_adapter,
        provenance_recorder=provider_service.record_provenance,
    )
    memory_service = MemoryService(unit_of_work)
    relationships = RelationshipService(unit_of_work)
    inner_state = InnerStateService(unit_of_work)
    diagnostics = DiagnosticsService(unit_of_work)
    object_client = boto3.client(
        "s3",
        endpoint_url=settings.s3_endpoint,
        region_name=settings.s3_region,
        aws_access_key_id=settings.s3_access_key,
        aws_secret_access_key=settings.s3_secret_key,
        use_ssl=settings.s3_use_ssl,
    )
    media_service = MediaService(
        unit_of_work,
        S3ObjectStorage(object_client, settings.s3_bucket),
    )
    autonomy_service = AutonomyService(unit_of_work)
    configure_embedding_service(memory_service, provider_runtime, unit_of_work)
    configure_media_service(media_service, settings_service)
    configure_autonomy_service(
        autonomy_service,
        AutonomyExecutor(
            conversations=ConversationService(unit_of_work),
            memory=memory_service,
            relationships=relationships,
            life_world=LifeWorldService(unit_of_work),
            media=media_service,
            moments=MomentsService(unit_of_work),
        ),
    )
    configure_cognition_service(
        CognitionService(
            unit_of_work,
            provider_runtime,
            provider_runtime,
            reflection_provider=provider_runtime,
            reflection_applier=ReflectionCoordinator(memory_service, relationships),
            state_applier=CognitionStateApplier(inner_state),
            diagnostics=diagnostics,
        )
    )
    redis = Redis.from_url(settings.redis_url, decode_responses=True)
    streams = RedisStreams(redis)
    await bootstrap_streams_with_retry(streams)

    async def replay_outbox() -> None:
        """Read PostgreSQL pages before publishing so Redis never runs in a DB transaction."""

        offset = 0
        while True:
            async with unit_of_work.begin(command_id=f"redis-rebuild-read:{offset}") as tx:
                events = await streams.read_outbox_page(tx.session, offset=offset)
            if not events:
                return
            await streams.publish_events(events)
            offset += len(events)

    await replay_outbox()
    client = await Client.connect(settings.temporal_address, namespace=settings.temporal_namespace)
    dispatcher = CommittedIntentDispatcher(
        client,
        UnitOfWorkFactory(engine),
        {
            "platform": PlatformControlWorkflow,
            "cognition": CognitionProcessingWorkflow,
            "autonomy": AutonomyActionWorkflow,
            "media": MediaGenerationWorkflow,
            "memory": MemoryEmbeddingWorkflow,
        },
    )
    workers = [
        Worker(
            client,
            task_queue=queue,
            activities=[
                process_cognition,
                process_memory_embedding,
                process_media_generation,
                process_autonomy_action,
            ],
            workflows=[
                PlatformControlWorkflow,
                CognitionProcessingWorkflow,
                AutonomyActionWorkflow,
                MediaGenerationWorkflow,
                MemoryEmbeddingWorkflow,
            ],
            max_concurrent_workflow_tasks=1 if queue != "interaction" else 2,
            max_cached_workflows=50,
            deployment_config=deployment_config(build_id),
        )
        for queue in TASK_QUEUES
    ]
    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    for signal_number in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(signal_number, stop.set)

    async def dispatch_loop() -> None:
        while not stop.is_set():
            try:
                await dispatcher.dispatch_once()
            except Exception as exc:
                logger.warning("worker dispatch loop retry: error=%s", type(exc).__name__)
            await asyncio.sleep(1)

    async def outbox_loop() -> None:
        while not stop.is_set():
            try:
                async with unit_of_work.begin(command_id="outbox-publisher") as tx:
                    events = await streams.read_pending_outbox(tx.session)
                if events:
                    await streams.publish_events(events)
                    async with unit_of_work.begin(command_id="outbox-publisher-mark") as tx:
                        marked = await streams.mark_published(
                            tx.session, (event.id for event in events)
                        )
                        if marked:
                            await tx.commit()
            except Exception:
                try:
                    await replay_outbox()
                except Exception as exc:
                    logger.warning("worker outbox loop retry: error=%s", type(exc).__name__)
            await asyncio.sleep(1)

    async def consume_loop(group: str) -> None:
        effect_types = {
            "bff-notifications": "notification_queued",
            "cache-projections": "aggregate_projected",
            "integration-observers": "integration_observed",
        }

        async def apply_effect(session, event_id: str, fields: dict[str, str]) -> dict[str, str]:
            effect_type = effect_types[group]
            sequence = int(fields["aggregate_sequence"])
            digest = sha256(fields.get("payload", "").encode("utf-8")).hexdigest()
            effect_id = sha256(f"{group}:{event_id}".encode()).hexdigest()
            await session.execute(
                insert(platform_schema.consumer_effects).values(
                    id=f"effect_{effect_id}",
                    consumer_group=group,
                    event_id=event_id,
                    effect_type=effect_type,
                    aggregate_type=fields["aggregate_type"],
                    aggregate_id=fields["aggregate_id"],
                    aggregate_sequence=sequence,
                    correlation_id=fields.get("correlation_id", "")[:128],
                    fluctlight_id=fields.get("fluctlight_id") or None,
                    payload_digest=digest,
                )
            )
            return {
                "status": "applied",
                "event_id": event_id,
                "effect_type": effect_type,
            }

        while not stop.is_set():
            try:
                await streams.consume_transactional(
                    group=group,
                    consumer=f"worker-{group}",
                    unit_of_work=unit_of_work,
                    handler=apply_effect,
                )
            except Exception as exc:
                logger.warning(
                    "worker consumer loop retry: group=%s error=%s",
                    group,
                    type(exc).__name__,
                )
            await asyncio.sleep(1)

    tasks = [asyncio.create_task(worker.run()) for worker in workers]
    await asyncio.sleep(1)
    await activate_deployment_version(client, build_id)
    tasks.append(asyncio.create_task(dispatch_loop()))
    tasks.append(asyncio.create_task(outbox_loop()))
    tasks.extend(asyncio.create_task(consume_loop(group)) for group in DURABLE_CONSUMER_GROUPS)
    stop_wait = asyncio.create_task(stop.wait())
    done, _ = await asyncio.wait([*tasks, stop_wait], return_when=asyncio.FIRST_COMPLETED)
    if stop_wait not in done:
        for task in done:
            if task.cancelled():
                logger.error("worker task cancelled unexpectedly")
                continue
            try:
                task.result()
            except Exception as exc:
                logger.error("worker task failed: error=%s", type(exc).__name__)
            else:
                logger.error("worker task exited unexpectedly")
        stop.set()
    else:
        stop_wait.cancel()
    await asyncio.gather(stop_wait, return_exceptions=True)
    for task in tasks:
        task.cancel()
    await asyncio.gather(*tasks, return_exceptions=True)
    await redis.aclose()
    await engine.dispose()


def main() -> None:
    asyncio.run(run_worker(PlatformSettings.from_environ()))
