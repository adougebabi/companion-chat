"""FastAPI composition root for internal Core platform routes."""

from __future__ import annotations

import logging
from collections.abc import Awaitable, Callable
from contextlib import asynccontextmanager
from dataclasses import dataclass
from datetime import UTC, date, datetime
from hashlib import sha256
from uuid import uuid4

import boto3  # type: ignore[import-untyped]
from fastapi import FastAPI, Header, HTTPException, Path
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse, Response
from pydantic import BaseModel, Field
from sqlalchemy import insert, select, text
from sqlalchemy.ext.asyncio import AsyncEngine
from temporalio.client import Client

from fluctlight_core.actors.directory import ActorDirectoryService
from fluctlight_core.actors.security import MIN_OWNER_PASSWORD_LENGTH
from fluctlight_core.actors.service import AuthError, AuthService
from fluctlight_core.autonomy.bridge import CognitionAutonomyBridge
from fluctlight_core.autonomy.service import AutonomyService
from fluctlight_core.autonomy.workflows import AutonomyActionWorkflow
from fluctlight_core.cognition.service import CognitionService
from fluctlight_core.cognition.turn_responder import CognitionTurnResponder
from fluctlight_core.cognition.workflows import CognitionProcessingWorkflow
from fluctlight_core.conversations.contracts import (
    ConversationAuthorizationError,
    ConversationConflictError,
    ConversationNotFoundError,
    ConversationTurn,
)
from fluctlight_core.conversations.service import ConversationService
from fluctlight_core.diagnostics.contracts import DiagnosticEvent, DiagnosticSeverity
from fluctlight_core.diagnostics.service import DiagnosticsAuthorizationError, DiagnosticsService
from fluctlight_core.fluctlights.contracts import (
    CreateFluctlight,
    FluctlightStatus,
    FoundationRevisionRequest,
    Identity,
    InitializationMode,
    RevisionSource,
)
from fluctlight_core.fluctlights.creation import (
    CreationError,
    CreationLifecycleService,
    InitialAgencyService,
)
from fluctlight_core.fluctlights.policy import RevisionConflictError
from fluctlight_core.fluctlights.service import FluctlightLifecycleError, FluctlightService
from fluctlight_core.inner_state import CognitionStateApplier, InnerStateService
from fluctlight_core.life_world.contracts import (
    ActionStatus,
    PresenceOverlay,
    ScheduleItem,
    ScheduleValidationError,
    ScheduleVersion,
    WorldEvent,
)
from fluctlight_core.life_world.lifecycle import ScheduleLifecycleRegistrar
from fluctlight_core.life_world.service import LifeWorldService
from fluctlight_core.life_world.workflows import CurrentDayScheduleWorkflow, DailyLifeReviewWorkflow
from fluctlight_core.media.service import MediaService
from fluctlight_core.media.workflows import MediaGenerationWorkflow
from fluctlight_core.memory.service import MemoryService
from fluctlight_core.memory.workflows import MemoryEmbeddingWorkflow
from fluctlight_core.moments.contracts import (
    MomentComment,
    MomentReaction,
    MomentStatus,
    ReactionKind,
)
from fluctlight_core.moments.service import MomentsService
from fluctlight_core.platform import schema as platform_schema
from fluctlight_core.platform.configuration import ConfigurationError, PlatformSettings, RuntimeRole
from fluctlight_core.platform.object_storage import RangeNotSatisfiable, S3ObjectStorage
from fluctlight_core.platform.persistence import (
    MigrationRevisionError,
    UnitOfWork,
    UnitOfWorkFactory,
    create_engine,
    verify_revision,
)
from fluctlight_core.platform.temporal import RestartSpec, TemporalRuntime
from fluctlight_core.platform.workflows import PlatformControlWorkflow
from fluctlight_core.providers.adapters import OpenAICompatibleAdapter
from fluctlight_core.providers.contracts import ModelRole
from fluctlight_core.providers.runtime import ConfiguredProviderRuntime, InitializationAnalysisError
from fluctlight_core.providers.service import (
    ProviderConfigurationError,
    ProviderConfigurationService,
    RoleAssignment,
)
from fluctlight_core.reflection.service import ReflectionCoordinator
from fluctlight_core.reflection.workflows import ReflectionWorkflow
from fluctlight_core.relationships.service import RelationshipService
from fluctlight_core.settings.crypto import SecretCodec, SecretConfigurationError
from fluctlight_core.settings.service import SafeSettingsView, SettingsError, SettingsService
from fluctlight_core.transport.conversations import (
    ConversationCreateRequest,
    ConversationTurnRequest,
    ReadPositionRequest,
    page_response,
    turn_stream_response,
)

EXPECTED_REVISION = "0019_compound_effects"
logger = logging.getLogger(__name__)


class _OwnerWorkflowAuthorizer:
    def __init__(self, actor_id: str) -> None:
        self._actor_id = actor_id

    def can_manage_workflows(self, actor_id: str) -> bool:
        return actor_id == self._actor_id


class _WorkflowAudit:
    def __init__(self, unit_of_work: UnitOfWorkFactory) -> None:
        self._unit_of_work = unit_of_work

    async def record(
        self,
        *,
        action: str,
        workflow_id: str,
        actor_id: str,
        authorized: bool,
        details: dict[str, object],
    ) -> None:
        async with self._unit_of_work.begin(command_id=f"workflow-audit:{uuid4().hex}") as tx:
            await tx.session.execute(
                insert(platform_schema.workflow_management_audit).values(
                    id=f"workflow_audit_{uuid4().hex}",
                    action=action,
                    workflow_id=workflow_id,
                    actor_id=actor_id,
                    authorized=str(authorized).lower(),
                    details=details,
                )
            )
            await tx.commit()


@dataclass(slots=True)
class ApiDependencies:
    settings: PlatformSettings
    engine: AsyncEngine
    verify_database: Callable[[], Awaitable[None]]
    auth: AuthService | None = None
    settings_service: SettingsService | None = None
    providers: ProviderConfigurationService | None = None
    diagnostics: DiagnosticsService | None = None
    conversations: ConversationService | None = None
    media: MediaService | None = None
    fluctlights: FluctlightService | None = None
    creations: CreationLifecycleService | None = None
    moments: MomentsService | None = None
    inner_state: InnerStateService | None = None
    relationships: RelationshipService | None = None
    memory: MemoryService | None = None
    life_world: LifeWorldService | None = None
    cognition: CognitionService | None = None
    autonomy: AutonomyService | None = None
    directory: ActorDirectoryService | None = None


class PlatformPingResponse(BaseModel):
    status: str
    role: str


class SetupRequest(BaseModel):
    setup_token: str = Field(min_length=16)
    password: str = Field(min_length=MIN_OWNER_PASSWORD_LENGTH)


class LoginRequest(BaseModel):
    password: str = Field(min_length=MIN_OWNER_PASSWORD_LENGTH)


class PasswordResetRequest(BaseModel):
    password: str = Field(min_length=MIN_OWNER_PASSWORD_LENGTH)


class FluctlightCreateRequest(BaseModel):
    id: str | None = Field(default=None, min_length=1, max_length=128)
    name: str | None = Field(default=None, max_length=256)


class FluctlightCreationAnalysisRequest(BaseModel):
    description: str = Field(min_length=1, max_length=12_000)


class FluctlightCreationActivationRequest(BaseModel):
    request_id: str = Field(min_length=1, max_length=256)
    initialization_mode: InitializationMode
    identity: dict[str, object]
    personality: dict[str, object] | None = None
    behavioral_policy: dict[str, object] | None = None
    life_profile: dict[str, object] | None = None
    foundation_provenance: dict[str, object] | None = None
    initial_goals: list[dict[str, object]] | None = None
    initial_intentions: list[dict[str, object]] | None = None


class MomentCommentRequest(BaseModel):
    text: str = Field(min_length=1, max_length=32_000)


class MomentReactionRequest(BaseModel):
    kind: ReactionKind = ReactionKind.LIKE


class FluctlightStatusRequest(BaseModel):
    status: FluctlightStatus
    expected_revision: int = Field(ge=0)
    reason: str = Field(min_length=1, max_length=1024)


class FluctlightRetireRequest(BaseModel):
    expected_revision: int = Field(ge=0)
    reason: str = Field(min_length=1, max_length=1024)


class FoundationRevisionRequestModel(BaseModel):
    changes: dict[str, object] = Field(min_length=1)
    expected_revision: int = Field(ge=0)
    reason: str = Field(min_length=1, max_length=1024)


class FoundationRevisionAcceptRequest(BaseModel):
    expected_revision: int = Field(ge=0)
    reason: str = Field(min_length=1, max_length=1024)


class FoundationRevisionRollbackRequest(FoundationRevisionAcceptRequest):
    target_revision: int = Field(ge=0)


class MemoryRevisionRequest(BaseModel):
    expected_revision: int = Field(ge=0)
    content: str = Field(min_length=1, max_length=4096)
    evidence_refs: list[str] = Field(min_length=1, max_length=32)


class MemoryForgetRequest(BaseModel):
    expected_revision: int = Field(ge=0)
    evidence_refs: list[str] = Field(min_length=1, max_length=32)


class RelationshipRollbackRequest(BaseModel):
    target_actor_id: str = Field(min_length=1, max_length=128)
    target_revision: int = Field(ge=0)
    expected_revision: int = Field(ge=0)
    evidence_refs: list[str] = Field(min_length=1, max_length=32)


class AutonomousActionGovernanceRequest(BaseModel):
    status: ActionStatus
    reason: str = Field(min_length=1, max_length=1024)


class LifeEventRequest(BaseModel):
    kind: str = Field(min_length=1, max_length=128)
    start_at: datetime
    end_at: datetime
    scene: str | None = Field(default=None, max_length=512)
    activity: str | None = Field(default=None, max_length=512)
    location: str | None = Field(default=None, max_length=512)
    evidence_refs: list[str] = Field(min_length=1, max_length=32)


class PresenceRequest(BaseModel):
    current_task: str | None = Field(default=None, max_length=512)
    user_presence: str | None = Field(default=None, max_length=128)


class ScheduleItemRequest(BaseModel):
    start_at: datetime
    end_at: datetime
    activity: str = Field(min_length=1, max_length=128)
    scene: str = Field(min_length=1, max_length=128)
    item_type: str = Field(default="planned", min_length=1, max_length=128)
    status: str = Field(default="planned", min_length=1, max_length=128)
    priority: float = Field(default=0.5, ge=0, le=1)
    flexibility: float = Field(default=0.5, ge=0, le=1)
    interruption_cost: float = Field(default=0.5, ge=0, le=1)


class ScheduleRequest(BaseModel):
    local_date: date
    timezone: str = Field(min_length=1, max_length=128)
    items: list[ScheduleItemRequest] = Field(min_length=1, max_length=128)
    evidence_refs: list[str] = Field(min_length=1, max_length=32)
    expected_revision: int | None = Field(default=None, ge=0)
    completed_before: datetime | None = None


class ScheduleCancelRequest(BaseModel):
    expected_revision: int = Field(ge=0)


class WorkflowResetRequest(BaseModel):
    history_point: int = Field(ge=1)


class ActorGroupRequest(BaseModel):
    name: str = Field(min_length=1, max_length=128)


class ActorGroupMemberRequest(BaseModel):
    actor_id: str = Field(min_length=1, max_length=128)


class SessionResponse(BaseModel):
    authenticated: bool
    actor_id: str | None = None
    session_token: str | None = None


class SetupStatusResponse(BaseModel):
    setup_available: bool


class SettingsPatchRequest(BaseModel):
    values: dict[str, object] = {}
    secrets: dict[str, str | None] = {}
    clear_secrets: set[str] = set()


class SafeSettingsResponse(BaseModel):
    values: dict[str, object]
    configured_secrets: list[str]


class ProviderEndpointRequest(BaseModel):
    endpoint_id: str = Field(min_length=1, max_length=128)
    kind: str = Field(min_length=1, max_length=64)
    base_url: str = Field(min_length=8)
    secret_purpose: str = Field(min_length=1, max_length=128)


class ModelRoleRequest(BaseModel):
    role: ModelRole
    endpoint_id: str = Field(min_length=1, max_length=128)
    model_id: str = Field(min_length=1, max_length=256)
    token_budget: int = Field(gt=0)
    timeout_seconds: int = Field(gt=0)


class ProviderPreflightResponse(BaseModel):
    role: ModelRole
    available: bool
    capability_version: str | None = None


class ProviderModelsResponse(BaseModel):
    endpoint_id: str
    models: list[str]


class DiagnosticEventResponse(BaseModel):
    id: str
    event_type: str
    severity: str
    fluctlight_id: str | None = None
    causation_id: str | None = None
    correlation_id: str
    payload: dict[str, object]
    created_at: str | None = None


class DiagnosticModelRunResponse(BaseModel):
    id: str
    role: str
    endpoint_id: str | None = None
    model_id: str
    prompt: dict[str, object]
    response: dict[str, object] | None = None
    status: str
    error_code: str | None = None
    correlation_id: str
    created_at: str


def create_app(dependencies: ApiDependencies | None = None) -> FastAPI:
    resolved: ApiDependencies | None = dependencies

    @asynccontextmanager
    async def lifespan(_: FastAPI):
        nonlocal resolved
        if resolved is None:
            settings = PlatformSettings.from_environ()
            settings.require_role(RuntimeRole.API)
            engine = create_engine(settings.database_url)

            async def verify_database() -> None:
                await verify_revision(engine, EXPECTED_REVISION)

            auth = AuthService(UnitOfWorkFactory(engine))
            unit_of_work = UnitOfWorkFactory(engine)
            settings_service = SettingsService(
                unit_of_work, SecretCodec(settings.settings_key), auth
            )
            provider_adapter = OpenAICompatibleAdapter()
            provider_service = ProviderConfigurationService(
                unit_of_work,
                auth,
                settings_service,
                provider_adapter.preflight,
                provider_adapter.list_models,
            )
            diagnostics = DiagnosticsService(unit_of_work)
            provider_runtime = ConfiguredProviderRuntime(
                unit_of_work,
                settings_service,
                adapter=provider_adapter,
                provenance_recorder=provider_service.record_provenance,
                diagnostics=diagnostics,
            )
            inner_state = InnerStateService(unit_of_work)
            schedule_lifecycle = ScheduleLifecycleRegistrar(unit_of_work)
            memory = MemoryService(unit_of_work)
            relationships = RelationshipService(unit_of_work)
            reflection = ReflectionCoordinator(memory, relationships)

            async def initialize_inner_state(fluctlight_id: str, tx: UnitOfWork) -> None:
                await inner_state.initialize(fluctlight_id, tx=tx)
                await schedule_lifecycle.register(fluctlight_id, tx=tx)

            fluctlights = FluctlightService(unit_of_work, state_initializer=initialize_inner_state)

            async def fluctlight_activity_status(fluctlight_id: str) -> str:
                return (await fluctlights.get(fluctlight_id)).status.value

            autonomy = AutonomyService(unit_of_work, status_resolver=fluctlight_activity_status)

            cognition = CognitionService(
                unit_of_work,
                provider_runtime,
                provider_runtime,
                reflection_provider=provider_runtime,
                reflection_applier=reflection,
                state_applier=CognitionStateApplier(inner_state),
                autonomy_freezer=CognitionAutonomyBridge(autonomy, settings_service),
                diagnostics=diagnostics,
            )
            object_client = boto3.client(
                "s3",
                endpoint_url=settings.s3_endpoint,
                region_name=settings.s3_region,
                aws_access_key_id=settings.s3_access_key,
                aws_secret_access_key=settings.s3_secret_key,
                use_ssl=settings.s3_use_ssl,
            )

            life_world = LifeWorldService(unit_of_work)
            resolved = ApiDependencies(
                settings=settings,
                engine=engine,
                verify_database=verify_database,
                auth=auth,
                directory=ActorDirectoryService(unit_of_work, auth),
                settings_service=settings_service,
                providers=provider_service,
                diagnostics=diagnostics,
                conversations=ConversationService(
                    unit_of_work,
                    CognitionTurnResponder(cognition, fluctlights.get),
                ),
                media=MediaService(
                    unit_of_work,
                    S3ObjectStorage(object_client, settings.s3_bucket),
                    fluctlights.can_actor_access,
                ),
                fluctlights=fluctlights,
                creations=CreationLifecycleService(
                    fluctlights,
                    provider_runtime,
                    InitialAgencyService(inner_state),
                ),
                moments=MomentsService(unit_of_work),
                inner_state=inner_state,
                relationships=relationships,
                memory=memory,
                life_world=life_world,
                cognition=cognition,
                autonomy=autonomy,
            )
        yield
        if dependencies is None and resolved is not None:
            await resolved.engine.dispose()

    app = FastAPI(title="Fluctlight Core Platform", version="0.1.0", lifespan=lifespan)

    @app.exception_handler(RequestValidationError)
    async def request_validation_error_handler(_, exc: RequestValidationError) -> JSONResponse:
        errors = [
            {
                "location": [str(part) for part in error.get("loc", ())],
                "type": str(error.get("type", "validation_error")),
                "message": str(error.get("msg", "request validation failed")),
            }
            for error in exc.errors()
        ]
        logger.error("core.request_validation_failed errors=%s", errors)
        return JSONResponse(
            status_code=422,
            content={
                "detail": {
                    "code": "core_request_validation_failed",
                    "message": "Core request validation failed",
                    "details": {"validation_errors": errors},
                }
            },
        )

    def require_dependencies() -> ApiDependencies:
        if resolved is None:
            raise HTTPException(status_code=503, detail="Core platform is starting")
        return resolved

    def require_service_key(key: str | None) -> ApiDependencies:
        current = require_dependencies()
        if key != current.settings.core_service_key:
            raise HTTPException(status_code=401, detail="invalid internal Core service credential")
        return current

    def require_auth_service(current: ApiDependencies) -> AuthService:
        if current.auth is None:
            raise HTTPException(status_code=503, detail="Core authentication is starting")
        return current.auth

    def require_directory_service(current: ApiDependencies) -> ActorDirectoryService:
        if current.directory is None:
            raise HTTPException(status_code=503, detail="Core Actor directory is starting")
        return current.directory

    def require_settings_service(current: ApiDependencies) -> SettingsService:
        if current.settings_service is None:
            raise HTTPException(status_code=503, detail="Core settings are starting")
        return current.settings_service

    def require_provider_service(current: ApiDependencies) -> ProviderConfigurationService:
        if current.providers is None:
            raise HTTPException(status_code=503, detail="Core providers are starting")
        return current.providers

    def require_diagnostics_service(current: ApiDependencies) -> DiagnosticsService:
        if current.diagnostics is None:
            raise HTTPException(status_code=503, detail="Core diagnostics are starting")
        return current.diagnostics

    def require_conversation_service(current: ApiDependencies) -> ConversationService:
        if current.conversations is None:
            raise HTTPException(status_code=503, detail="Core conversations are starting")
        return current.conversations

    def require_media_service(current: ApiDependencies) -> MediaService:
        if current.media is None:
            raise HTTPException(status_code=503, detail="Core media is starting")
        return current.media

    def require_fluctlight_service(current: ApiDependencies) -> FluctlightService:
        if current.fluctlights is None:
            raise HTTPException(status_code=503, detail="Core Fluctlights are starting")
        return current.fluctlights

    def require_creation_service(current: ApiDependencies) -> CreationLifecycleService:
        if current.creations is None:
            raise HTTPException(status_code=503, detail="Core creation lifecycle is starting")
        return current.creations

    def require_moments_service(current: ApiDependencies) -> MomentsService:
        if current.moments is None:
            raise HTTPException(status_code=503, detail="Core Moments are starting")
        return current.moments

    def require_inner_state_service(current: ApiDependencies) -> InnerStateService:
        if current.inner_state is None:
            raise HTTPException(status_code=503, detail="Core inner state is starting")
        return current.inner_state

    def require_relationship_service(current: ApiDependencies) -> RelationshipService:
        if current.relationships is None:
            raise HTTPException(status_code=503, detail="Core relationships are starting")
        return current.relationships

    def require_memory_service(current: ApiDependencies) -> MemoryService:
        if current.memory is None:
            raise HTTPException(status_code=503, detail="Core memory is starting")
        return current.memory

    def require_life_world_service(current: ApiDependencies) -> LifeWorldService:
        if current.life_world is None:
            raise HTTPException(status_code=503, detail="Core life world is starting")
        return current.life_world

    def require_cognition_service(current: ApiDependencies) -> CognitionService:
        if current.cognition is None:
            raise HTTPException(status_code=503, detail="Core cognition is starting")
        return current.cognition

    def require_autonomy_service(current: ApiDependencies) -> AutonomyService:
        if current.autonomy is None:
            raise HTTPException(status_code=503, detail="Core autonomy is starting")
        return current.autonomy

    async def require_workflow_runtime(current: ApiDependencies, actor_id: str) -> TemporalRuntime:
        client = await Client.connect(
            current.settings.temporal_address,
            namespace=current.settings.temporal_namespace,
        )
        workflow_types = {
            "platform": PlatformControlWorkflow,
            "cognition": CognitionProcessingWorkflow,
            "autonomy": AutonomyActionWorkflow,
            "media": MediaGenerationWorkflow,
            "memory": MemoryEmbeddingWorkflow,
            "reflection": ReflectionWorkflow,
            "schedule": CurrentDayScheduleWorkflow,
            "daily_review": DailyLifeReviewWorkflow,
        }

        async def restart_spec(workflow_id: str) -> RestartSpec:
            async with UnitOfWorkFactory(current.engine).begin(
                command_id=f"workflow-restart-spec:{workflow_id}"
            ) as tx:
                row = (
                    (
                        await tx.session.execute(
                            select(platform_schema.workflow_intents).where(
                                platform_schema.workflow_intents.c.workflow_id == workflow_id
                            )
                        )
                    )
                    .mappings()
                    .one_or_none()
                )
            if row is None:
                raise KeyError("workflow has no committed restart intent")
            prefix = str(row["intent_type"]).split(".", 1)[0]
            workflow = workflow_types.get(prefix)
            if workflow is None:
                raise ValueError("workflow restart type is unavailable")
            payload = dict(row["payload"])
            payload.setdefault("intent_id", str(row["intent_id"]))
            return RestartSpec(workflow, (payload,), str(row["task_queue"]))

        return TemporalRuntime(
            client,
            _OwnerWorkflowAuthorizer(actor_id),
            _WorkflowAudit(UnitOfWorkFactory(current.engine)),
            restart_specs=restart_spec,
        )

    def safe_settings(view: SafeSettingsView) -> SafeSettingsResponse:
        return SafeSettingsResponse(
            values=view.values, configured_secrets=sorted(view.configured_secrets)
        )

    @app.get("/health/live", response_model=PlatformPingResponse)
    async def live() -> PlatformPingResponse:
        return PlatformPingResponse(status="ok", role="api")

    @app.get("/health/ready", response_model=PlatformPingResponse)
    async def ready() -> PlatformPingResponse:
        current = require_dependencies()
        try:
            await current.verify_database()
            async with current.engine.connect() as connection:
                await connection.execute(text("SELECT 1"))
        except (ConfigurationError, MigrationRevisionError, OSError, RuntimeError) as exc:
            raise HTTPException(status_code=503, detail="Core database is not ready") from exc
        return PlatformPingResponse(status="ready", role="api")

    @app.get("/internal/platform/ping", response_model=PlatformPingResponse)
    async def ping(
        x_fluctlight_service_key: str | None = Header(default=None),
    ) -> PlatformPingResponse:
        current = require_service_key(x_fluctlight_service_key)
        return PlatformPingResponse(status="ok", role=current.settings.role.value)

    @app.post("/internal/auth/setup", response_model=SessionResponse)
    async def setup(
        request: SetupRequest, x_fluctlight_service_key: str | None = Header(default=None)
    ) -> SessionResponse:
        service = require_auth_service(require_service_key(x_fluctlight_service_key))
        try:
            session = await service.setup(
                setup_token=request.setup_token, password=request.password
            )
        except AuthError as exc:
            raise HTTPException(status_code=exc.status_code, detail=exc.code) from exc
        return SessionResponse(
            authenticated=True, actor_id=session.actor_id, session_token=session.token
        )

    @app.post("/internal/auth/login", response_model=SessionResponse)
    async def login(
        request: LoginRequest, x_fluctlight_service_key: str | None = Header(default=None)
    ) -> SessionResponse:
        service = require_auth_service(require_service_key(x_fluctlight_service_key))
        try:
            session = await service.login(password=request.password)
        except AuthError as exc:
            raise HTTPException(status_code=exc.status_code, detail=exc.code) from exc
        return SessionResponse(
            authenticated=True, actor_id=session.actor_id, session_token=session.token
        )

    @app.get("/internal/auth/session", response_model=SessionResponse)
    async def session(
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> SessionResponse:
        service = require_auth_service(require_service_key(x_fluctlight_service_key))
        try:
            actor = await service.resolve(x_fluctlight_human_session)
        except AuthError:
            return SessionResponse(authenticated=False)
        return SessionResponse(authenticated=True, actor_id=actor.actor_id)

    @app.get("/internal/auth/setup-status", response_model=SetupStatusResponse)
    async def setup_status(
        x_fluctlight_service_key: str | None = Header(default=None),
    ) -> SetupStatusResponse:
        service = require_auth_service(require_service_key(x_fluctlight_service_key))
        return SetupStatusResponse(setup_available=await service.setup_available())

    @app.post("/internal/auth/revoke-all", status_code=204, response_model=None)
    async def revoke_all(
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            await require_auth_service(current).revoke_all(actor)
        except AuthError as exc:
            raise HTTPException(status_code=exc.status_code, detail=exc.code) from exc

    @app.post("/internal/auth/revoke-current", status_code=204, response_model=None)
    async def revoke_current(
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            await require_auth_service(current).revoke_current(actor)
        except AuthError as exc:
            raise HTTPException(status_code=exc.status_code, detail=exc.code) from exc

    @app.post("/internal/auth/reset-password", status_code=204, response_model=None)
    async def reset_password(
        request: PasswordResetRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            await require_auth_service(current).reset_password(actor, password=request.password)
        except AuthError as exc:
            raise HTTPException(status_code=exc.status_code, detail=exc.code) from exc

    @app.get("/internal/settings", response_model=SafeSettingsResponse)
    async def read_settings(
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> SafeSettingsResponse:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            return safe_settings(await require_settings_service(current).read(actor))
        except (AuthError, SettingsError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc

    @app.put("/internal/settings", response_model=SafeSettingsResponse)
    async def update_settings(
        request: SettingsPatchRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> SafeSettingsResponse:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            view = await require_settings_service(current).update(
                actor,
                values=request.values,
                secrets=request.secrets,
                clear_secrets=frozenset(request.clear_secrets),
            )
            return safe_settings(view)
        except SecretConfigurationError as exc:
            raise HTTPException(status_code=422, detail="settings_secret_error") from exc
        except (AuthError, SettingsError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc

    @app.put("/internal/providers/endpoints", status_code=204, response_model=None)
    async def configure_provider_endpoint(
        request: ProviderEndpointRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            await require_provider_service(current).configure_endpoint(
                actor,
                endpoint_id=request.endpoint_id,
                kind=request.kind,
                base_url=request.base_url,
                secret_purpose=request.secret_purpose,
            )
        except (AuthError, ProviderConfigurationError) as exc:
            raise HTTPException(status_code=403, detail="provider_configuration_failed") from exc

    @app.get("/internal/providers")
    async def provider_bindings(
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[dict[str, object]]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            return await require_provider_service(current).list_bindings(actor)
        except (AuthError, ProviderConfigurationError) as exc:
            raise HTTPException(status_code=403, detail="provider_configuration_failed") from exc

    @app.get("/internal/providers/endpoints")
    async def provider_endpoints(
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[dict[str, object]]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            return await require_provider_service(current).list_endpoints(actor)
        except (AuthError, ProviderConfigurationError) as exc:
            raise HTTPException(status_code=403, detail="provider_configuration_failed") from exc

    @app.get(
        "/internal/providers/endpoints/{endpoint_id}/models",
        response_model=ProviderModelsResponse,
    )
    async def provider_endpoint_models(
        endpoint_id: str = Path(min_length=1, max_length=128),
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> ProviderModelsResponse:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            models = await require_provider_service(current).list_models(
                actor, endpoint_id=endpoint_id
            )
        except (AuthError, ProviderConfigurationError, SecretConfigurationError) as exc:
            raise HTTPException(status_code=422, detail="provider_models_unavailable") from exc
        return ProviderModelsResponse(endpoint_id=endpoint_id, models=list(models))

    @app.put("/internal/providers/roles", response_model=ProviderPreflightResponse)
    async def configure_model_role(
        request: ModelRoleRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> ProviderPreflightResponse:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            report = await require_provider_service(current).configure_role(
                actor,
                RoleAssignment(
                    role=request.role,
                    endpoint_id=request.endpoint_id,
                    model_id=request.model_id,
                    token_budget=request.token_budget,
                    timeout_seconds=request.timeout_seconds,
                ),
            )
        except (AuthError, ProviderConfigurationError, SecretConfigurationError) as exc:
            raise HTTPException(status_code=422, detail="provider_preflight_failed") from exc
        return ProviderPreflightResponse(
            role=report.role,
            available=report.available,
            capability_version=report.capability_version,
        )

    @app.get("/internal/diagnostics", response_model=list[DiagnosticEventResponse])
    async def read_diagnostics(
        limit: int = 100,
        correlation_id: str | None = None,
        fluctlight_id: str | None = None,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[DiagnosticEventResponse]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise DiagnosticsAuthorizationError("forbidden")
            rows = await require_diagnostics_service(current).query_events(
                actor_id=actor.actor_id,
                owner_actor_id=actor.actor_id,
                correlation_id=correlation_id,
                fluctlight_id=fluctlight_id,
                limit=limit,
            )
        except (AuthError, DiagnosticsAuthorizationError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        return [DiagnosticEventResponse(**row) for row in rows]

    @app.delete("/internal/diagnostics", response_model=dict[str, int])
    async def clear_diagnostics(
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, int]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise DiagnosticsAuthorizationError("forbidden")
            count = await require_diagnostics_service(current).clear_events(
                actor_id=actor.actor_id, owner_actor_id=actor.actor_id
            )
        except (AuthError, DiagnosticsAuthorizationError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        return {"cleared": count}

    @app.get("/internal/diagnostics/model-runs", response_model=list[DiagnosticModelRunResponse])
    async def read_diagnostic_model_runs(
        limit: int = 100,
        correlation_id: str | None = None,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[DiagnosticModelRunResponse]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise DiagnosticsAuthorizationError("forbidden")
            rows = await require_diagnostics_service(current).query_model_runs(
                actor_id=actor.actor_id,
                owner_actor_id=actor.actor_id,
                correlation_id=correlation_id,
                limit=limit,
            )
        except (AuthError, DiagnosticsAuthorizationError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        return [DiagnosticModelRunResponse(**row) for row in rows]

    @app.get("/internal/diagnostics/export")
    async def export_diagnostics(
        limit: int = 500,
        correlation_id: str | None = None,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise DiagnosticsAuthorizationError("forbidden")
            diagnostics = require_diagnostics_service(current)
            events = await diagnostics.query_events(
                actor_id=actor.actor_id,
                owner_actor_id=actor.actor_id,
                correlation_id=correlation_id,
                limit=limit,
            )
            runs = await diagnostics.query_model_runs(
                actor_id=actor.actor_id,
                owner_actor_id=actor.actor_id,
                correlation_id=correlation_id,
                limit=limit,
            )
        except (AuthError, DiagnosticsAuthorizationError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        return {"events": events, "model_runs": runs, "correlation_id": correlation_id}

    @app.get("/internal/diagnostics/workflows")
    async def list_workflows(
        query: str = "",
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[dict[str, object]]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise PermissionError("forbidden")
            runtime = await require_workflow_runtime(current, actor.actor_id)
            executions = await runtime.list(actor_id=actor.actor_id, query=query)
        except (AuthError, PermissionError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        except Exception as exc:
            raise HTTPException(status_code=502, detail="workflow_runtime_unavailable") from exc

        def execution_ids(value: object) -> tuple[str | None, str | None]:
            if not isinstance(value, dict):
                return None, None
            workflow_id = value.get("workflow_id") or value.get("workflowId")
            run_id = value.get("run_id") or value.get("runId")
            if isinstance(workflow_id, str):
                return workflow_id, run_id if isinstance(run_id, str) else None
            for nested in value.values():
                found_workflow_id, found_run_id = execution_ids(nested)
                if found_workflow_id is not None:
                    return found_workflow_id, found_run_id
            return None, None

        result: list[dict[str, object]] = []
        for item in executions:
            value = item.to_json_dict() if hasattr(item, "to_json_dict") else {}
            workflow_id, run_id = execution_ids(value)
            workflow_id = (
                workflow_id or getattr(item, "id", None) or getattr(item, "workflow_id", None)
            )
            if not isinstance(workflow_id, str) or not workflow_id:
                continue
            result.append(
                {
                    "workflow_id": workflow_id,
                    "run_id": run_id if isinstance(run_id, str) else None,
                    "execution": value,
                }
            )
        return result

    @app.get("/internal/diagnostics/workflows/{workflow_id}/status")
    async def workflow_status(
        workflow_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise PermissionError("forbidden")
            result = await (await require_workflow_runtime(current, actor.actor_id)).query(
                actor_id=actor.actor_id, workflow_id=workflow_id, query="status"
            )
        except (AuthError, PermissionError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        except Exception as exc:
            raise HTTPException(status_code=422, detail="workflow_status_failed") from exc
        return result if isinstance(result, dict) else {"status": str(result)}

    @app.get("/internal/diagnostics/workflows/{workflow_id}/history")
    async def workflow_history(
        workflow_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise PermissionError("forbidden")
            history = await (await require_workflow_runtime(current, actor.actor_id)).history(
                actor_id=actor.actor_id, workflow_id=workflow_id
            )
        except (AuthError, PermissionError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        except Exception as exc:
            raise HTTPException(status_code=422, detail="workflow_history_failed") from exc
        events = getattr(history, "events", ())
        return {
            "workflow_id": workflow_id,
            "event_count": len(events),
            "event_types": [str(getattr(event, "event_type", "")) for event in events[-50:]],
        }

    async def workflow_signal_command(
        workflow_id: str,
        signal_name: str,
        service_key: str | None,
        human_session: str | None,
    ) -> None:
        current = require_service_key(service_key)
        try:
            actor = await require_auth_service(current).resolve(human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise PermissionError("forbidden")
            await (await require_workflow_runtime(current, actor.actor_id)).signal(
                actor_id=actor.actor_id, workflow_id=workflow_id, signal=signal_name
            )
        except (AuthError, PermissionError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        except Exception as exc:
            raise HTTPException(status_code=422, detail="workflow_signal_failed") from exc

    @app.post(
        "/internal/diagnostics/workflows/{workflow_id}/pause",
        status_code=204,
        response_model=None,
    )
    async def pause_workflow(
        workflow_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        await workflow_signal_command(
            workflow_id, "pause", x_fluctlight_service_key, x_fluctlight_human_session
        )

    @app.post(
        "/internal/diagnostics/workflows/{workflow_id}/resume",
        status_code=204,
        response_model=None,
    )
    async def resume_workflow(
        workflow_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        await workflow_signal_command(
            workflow_id, "resume", x_fluctlight_service_key, x_fluctlight_human_session
        )

    @app.post(
        "/internal/diagnostics/workflows/{workflow_id}/cancel",
        status_code=204,
        response_model=None,
    )
    async def cancel_workflow(
        workflow_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise PermissionError("forbidden")
            await (await require_workflow_runtime(current, actor.actor_id)).cancel(
                actor_id=actor.actor_id, workflow_id=workflow_id
            )
        except (AuthError, PermissionError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        except Exception as exc:
            raise HTTPException(status_code=422, detail="workflow_cancel_failed") from exc

    @app.post("/internal/diagnostics/workflows/{workflow_id}/reset")
    async def reset_workflow(
        workflow_id: str,
        request: WorkflowResetRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, str]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise PermissionError("forbidden")
            await (await require_workflow_runtime(current, actor.actor_id)).reset(
                actor_id=actor.actor_id,
                workflow_id=workflow_id,
                history_point=request.history_point,
            )
        except (AuthError, PermissionError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        except Exception as exc:
            raise HTTPException(status_code=422, detail="workflow_reset_failed") from exc
        return {"workflow_id": workflow_id, "operation": "reset"}

    @app.post("/internal/diagnostics/workflows/{workflow_id}/restart")
    async def restart_workflow(
        workflow_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, str]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_auth_service(current).is_owner(actor):
                raise PermissionError("forbidden")
            await (await require_workflow_runtime(current, actor.actor_id)).restart(
                actor_id=actor.actor_id, workflow_id=workflow_id
            )
        except (AuthError, PermissionError) as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        except Exception as exc:
            raise HTTPException(status_code=422, detail="workflow_restart_failed") from exc
        return {"workflow_id": workflow_id, "operation": "restart"}

    @app.post("/internal/conversations")
    async def create_conversation(
        request: ConversationCreateRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if len(request.participant_actor_ids) > 1:
                raise ConversationAuthorizationError(
                    "multi-Fluctlight/group conversations are not enabled"
                )
            for participant_id in request.participant_actor_ids:
                if not await require_fluctlight_service(current).can_actor_access(
                    participant_id, actor.actor_id
                ):
                    raise ConversationAuthorizationError(
                        "conversation participant is not accessible"
                    )
            page = await require_conversation_service(current).get_or_create_direct(
                owner_actor_id=actor.actor_id,
                fluctlight_actor_id=request.participant_actor_ids[0],
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except ConversationConflictError as exc:
            raise HTTPException(status_code=409, detail="conversation_conflict") from exc
        except ConversationAuthorizationError as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        return page_response(page)

    @app.get("/internal/fluctlights/{fluctlight_id}/conversation")
    async def fluctlight_direct_conversation(
        fluctlight_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_fluctlight_service(current).can_actor_access(
                fluctlight_id, actor.actor_id
            ):
                raise ConversationAuthorizationError("Fluctlight is not accessible")
            page = await require_conversation_service(current).get_or_create_direct(
                owner_actor_id=actor.actor_id,
                fluctlight_actor_id=fluctlight_id,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except ConversationAuthorizationError as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        return page_response(page)

    @app.post("/internal/fluctlights")
    async def create_fluctlight(
        request: FluctlightCreateRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            fluctlight_id = request.id or f"fluctlight_{uuid4().hex}"
            name = request.name
            snapshot = await require_fluctlight_service(current).create(
                CreateFluctlight(
                    actor_id=actor.actor_id,
                    id=fluctlight_id,
                    identity=Identity(id=fluctlight_id, name=name),
                )
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (ValueError, RuntimeError) as exc:
            raise HTTPException(status_code=409, detail="fluctlight_create_failed") from exc
        return {
            "id": snapshot.id,
            "identity": snapshot.identity.as_payload(),
            "status": snapshot.status.value,
        }

    @app.post("/internal/fluctlight-creations/analysis")
    async def analyze_fluctlight_creation(
        request: FluctlightCreationAnalysisRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            await require_auth_service(current).resolve(x_fluctlight_human_session)
            return (
                await require_creation_service(current).analyze_description(request.description)
            ).as_payload()
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except InitializationAnalysisError as exc:
            logger.error(
                "fluctlight.creation.analysis_failed code=%s status=%s",
                exc.code,
                exc.status_code,
            )
            raise HTTPException(status_code=exc.status_code, detail=exc.code) from exc
        except CreationError as exc:
            logger.error(
                "fluctlight.creation.analysis_rejected code=%s message=%s details=%s",
                exc.code,
                str(exc),
                exc.details,
            )
            await require_diagnostics_service(current).emit_event(
                DiagnosticEvent(
                    event_type="fluctlight.initialization.failed",
                    severity=DiagnosticSeverity.ERROR,
                    correlation_id=f"initialization:{sha256(request.description.encode()).hexdigest()}",
                    payload={"error_code": exc.code},
                )
            )
            raise HTTPException(
                status_code=422,
                detail={"code": exc.code, "message": str(exc), "details": exc.details},
            ) from exc
        except RuntimeError as exc:
            logger.exception(
                "fluctlight.creation.analysis_unexpected error_type=%s",
                type(exc).__name__,
            )
            raise HTTPException(
                status_code=503,
                detail="initialization_provider_unavailable",
            ) from exc
        except Exception as exc:
            logger.exception(
                "fluctlight.creation.analysis_unhandled error_type=%s",
                type(exc).__name__,
            )
            raise HTTPException(
                status_code=500,
                detail={
                    "code": "initialization_unhandled_error",
                    "message": "Fluctlight analysis failed unexpectedly",
                    "details": {"error_type": type(exc).__name__},
                },
            ) from exc

    @app.post("/internal/fluctlight-creations/activate")
    async def activate_fluctlight_creation(
        request: FluctlightCreationActivationRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            snapshot = await require_creation_service(current).activate(
                actor_id=actor.actor_id,
                request_id=request.request_id,
                initialization_mode=request.initialization_mode,
                identity=dict(request.identity),
                personality=dict(request.personality) if request.personality is not None else None,
                behavioral_policy=(
                    dict(request.behavioral_policy)
                    if request.behavioral_policy is not None
                    else None
                ),
                life_profile=dict(request.life_profile)
                if request.life_profile is not None
                else None,
                foundation_provenance=(
                    dict(request.foundation_provenance)
                    if request.foundation_provenance is not None
                    else None
                ),
                initial_goals=[dict(item) for item in request.initial_goals]
                if request.initial_goals is not None
                else None,
                initial_intentions=[dict(item) for item in request.initial_intentions]
                if request.initial_intentions is not None
                else None,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except CreationError as exc:
            await require_diagnostics_service(current).emit_event(
                DiagnosticEvent(
                    event_type="fluctlight.activation.failed",
                    severity=DiagnosticSeverity.ERROR,
                    correlation_id=f"activation:{actor.actor_id}:{request.request_id}",
                    payload={"error_code": exc.code},
                )
            )
            raise HTTPException(
                status_code=422,
                detail={"code": exc.code, "message": str(exc), "details": exc.details},
            ) from exc
        return snapshot.as_payload()

    @app.get("/internal/fluctlights")
    async def list_fluctlights(
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[dict[str, object]]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            snapshots = await require_fluctlight_service(current).list_for_actor(actor.actor_id)
            unread_counts = await require_conversation_service(current).direct_unread_counts(
                owner_actor_id=actor.actor_id,
                fluctlight_actor_ids=tuple(snapshot.id for snapshot in snapshots),
            )
            last_activity = await require_conversation_service(current).direct_last_activity(
                owner_actor_id=actor.actor_id,
                fluctlight_actor_ids=tuple(snapshot.id for snapshot in snapshots),
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        last_activity_iso = {
            fluctlight_id: occurred_at.isoformat() if occurred_at is not None else None
            for fluctlight_id, occurred_at in last_activity.items()
        }
        return [
            {
                "id": snapshot.id,
                "identity": snapshot.identity.as_payload(),
                "personality": snapshot.personality.as_payload(),
                "behavioral_policy": snapshot.behavioral_policy.as_payload(),
                "status": snapshot.status.value,
                "current_revision": snapshot.current_revision,
                "unread_count": unread_counts.get(snapshot.id, 0),
                "last_conversation_at": last_activity_iso.get(snapshot.id),
            }
            for snapshot in snapshots
        ]

    @app.get("/internal/actor-groups")
    async def list_actor_groups(
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[dict[str, object]]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            groups = await require_directory_service(current).list_groups(actor)
        except AuthError as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        return [
            {"id": group.id, "name": group.name, "actor_ids": list(group.actor_ids)}
            for group in groups
        ]

    @app.post("/internal/actor-groups")
    async def create_actor_group(
        request: ActorGroupRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            group = await require_directory_service(current).create_group(actor, name=request.name)
        except AuthError as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        except ValueError as exc:
            raise HTTPException(status_code=422, detail="actor_group_create_failed") from exc
        return {"id": group.id, "name": group.name, "actor_ids": []}

    @app.post("/internal/actor-groups/{group_id}/members", status_code=204, response_model=None)
    async def assign_actor_group_member(
        group_id: str,
        request: ActorGroupMemberRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            await require_directory_service(current).assign_member(
                actor, group_id=group_id, member_actor_id=request.actor_id
            )
        except AuthError as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        except KeyError as exc:
            raise HTTPException(status_code=422, detail="actor_group_assign_failed") from exc

    @app.delete(
        "/internal/actor-groups/{group_id}/members/{actor_id}",
        status_code=204,
        response_model=None,
    )
    async def remove_actor_group_member(
        group_id: str,
        actor_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            await require_directory_service(current).remove_member(
                actor, group_id=group_id, member_actor_id=actor_id
            )
        except AuthError as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        except KeyError as exc:
            raise HTTPException(status_code=422, detail="actor_group_remove_failed") from exc

    @app.get("/internal/fluctlights/{fluctlight_id}")
    async def get_fluctlight(
        fluctlight_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            service = require_fluctlight_service(current)
            if not await service.can_actor_access(fluctlight_id, actor.actor_id):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            snapshot = await service.get(fluctlight_id)
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        return {
            "id": snapshot.id,
            "identity": snapshot.identity.as_payload(),
            "personality": snapshot.personality.as_payload(),
            "behavioral_policy": snapshot.behavioral_policy.as_payload(),
            "life_profile": snapshot.life_profile.as_payload(),
            "provenance": snapshot.provenance.as_payload(),
            "status": snapshot.status.value,
            "current_revision": snapshot.current_revision,
        }

    @app.get("/internal/fluctlights/{fluctlight_id}/detail")
    async def get_fluctlight_detail(
        fluctlight_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            service = require_fluctlight_service(current)
            if not await service.can_actor_access(fluctlight_id, actor.actor_id):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            snapshot = await service.get(fluctlight_id)
            state = await require_inner_state_service(current).read(fluctlight_id)
            goals, intentions = await require_inner_state_service(current).goals_and_intentions(
                fluctlight_id
            )
            relationships = await require_relationship_service(current).list_for_fluctlight(
                fluctlight_id
            )
            memories = await require_memory_service(current).recent_for_detail(
                owner_fluctlight_id=fluctlight_id,
                authorized_actor_ids=(fluctlight_id, actor.actor_id),
            )
            context = await require_life_world_service(current).resolve_context(
                fluctlight_id, datetime.now(UTC)
            )
            schedule = await require_life_world_service(current).accepted_schedule(
                fluctlight_id, datetime.now(UTC)
            )
            events = await require_life_world_service(current).list_events(fluctlight_id)
            cognition_history = await require_cognition_service(current).recent_history(
                fluctlight_id
            )
            revisions = await service.revision_history(fluctlight_id)
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        return {
            "id": snapshot.id,
            "identity": snapshot.identity.as_payload(),
            "personality": snapshot.personality.as_payload(),
            "behavioral_policy": snapshot.behavioral_policy.as_payload(),
            "life_profile": snapshot.life_profile.as_payload(),
            "provenance": snapshot.provenance.as_payload(),
            "status": snapshot.status.value,
            "current_revision": snapshot.current_revision,
            "inner_state": {
                "pad": state.pad.as_payload(),
                "mood": state.mood.as_payload(),
                "drives": [drive.as_payload() for drive in state.drives],
                "revision": state.revision,
                "last_updated_at": state.last_updated_at.isoformat(),
            },
            "relationships": [
                {
                    "target_actor_id": relationship.target_actor_id,
                    "metrics": dict(relationship.metrics),
                    "trend": relationship.trend.value,
                    "summary": relationship.summary,
                    "revision": relationship.revision,
                }
                for relationship in relationships
            ],
            "goals": [
                {
                    "id": goal.id,
                    "description": goal.description,
                    "status": goal.status.value,
                    "importance": goal.importance,
                    "urgency": goal.urgency,
                    "progress": goal.progress,
                }
                for goal in goals
            ],
            "intentions": [
                {
                    "id": intention.id,
                    "goal_id": intention.goal_id,
                    "action": intention.action,
                    "status": intention.status.value,
                    "confidence": intention.confidence,
                }
                for intention in intentions
            ],
            "memories": [
                {
                    "id": memory.id,
                    "owner_fluctlight_id": memory.owner_fluctlight_id,
                    "type": memory.type.value,
                    "content": memory.content,
                    "importance": memory.importance,
                    "status": memory.status.value,
                    "revision": memory.revision,
                }
                for memory in memories
            ],
            "context": {
                "source": context.source.value,
                "scene": context.scene,
                "activity": context.activity,
                "location": context.location,
                "instant": context.instant.isoformat(),
            },
            "schedule": (
                {
                    "id": schedule.id,
                    "local_date": schedule.local_date.isoformat(),
                    "timezone": schedule.timezone,
                    "revision": schedule.revision,
                    "items": [
                        {
                            "id": item.id,
                            "start_at": item.start_at.isoformat(),
                            "end_at": item.end_at.isoformat(),
                            "activity": item.activity,
                            "scene": item.scene,
                            "status": item.status,
                        }
                        for item in schedule.items
                    ],
                }
                if schedule is not None
                else None
            ),
            "events": [
                {
                    "id": event.id,
                    "kind": event.kind,
                    "start_at": event.start_at.isoformat(),
                    "end_at": event.end_at.isoformat(),
                    "scene": event.scene,
                    "activity": event.activity,
                    "location": event.location,
                    "status": event.status.value,
                    "evidence_refs": list(event.evidence_refs),
                }
                for event in events
            ],
            "cognition_history": cognition_history,
            "foundation_revisions": [
                {
                    "id": revision.id,
                    "revision": revision.revision,
                    "source": revision.source.value,
                    "status": revision.status.value,
                    "changes": dict(revision.changes),
                    "created_at": revision.created_at.isoformat(),
                    "accepted_at": revision.accepted_at.isoformat()
                    if revision.accepted_at
                    else None,
                    "reason": revision.reason,
                }
                for revision in revisions
            ],
        }

    @app.put("/internal/fluctlights/{fluctlight_id}/status")
    async def govern_fluctlight_status(
        fluctlight_id: str,
        request: FluctlightStatusRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            service = require_fluctlight_service(current)
            if not await service.can_actor_access(fluctlight_id, actor.actor_id):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            snapshot = await service.set_activity_status(
                fluctlight_id=fluctlight_id,
                actor_id=actor.actor_id,
                expected_revision=request.expected_revision,
                status=request.status,
                reason=request.reason,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (FluctlightLifecycleError, RevisionConflictError) as exc:
            raise HTTPException(status_code=422, detail="fluctlight_status_failed") from exc
        return {
            "id": snapshot.id,
            "status": snapshot.status.value,
            "current_revision": snapshot.current_revision,
        }

    @app.post("/internal/fluctlights/{fluctlight_id}/retire")
    async def retire_fluctlight(
        fluctlight_id: str,
        request: FluctlightRetireRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            service = require_fluctlight_service(current)
            if not await service.can_actor_access(fluctlight_id, actor.actor_id):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            snapshot = await service.retire(
                fluctlight_id=fluctlight_id,
                actor_id=actor.actor_id,
                expected_revision=request.expected_revision,
                reason=request.reason,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (FluctlightLifecycleError, RevisionConflictError) as exc:
            raise HTTPException(status_code=422, detail="fluctlight_retire_failed") from exc
        return {
            "id": snapshot.id,
            "status": snapshot.status.value,
            "retired_at": snapshot.retired_at.isoformat() if snapshot.retired_at else None,
        }

    @app.post("/internal/fluctlights/{fluctlight_id}/foundation-revisions")
    async def submit_foundation_revision(
        fluctlight_id: str,
        request: FoundationRevisionRequestModel,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            service = require_fluctlight_service(current)
            if not await service.can_actor_access(fluctlight_id, actor.actor_id):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            revision = await service.submit_revision(
                FoundationRevisionRequest(
                    fluctlight_id=fluctlight_id,
                    actor_id=actor.actor_id,
                    source=RevisionSource.HUMAN,
                    changes=request.changes,
                    evidence_refs=(f"owner-governance:{uuid4().hex}",),
                    expected_revision=request.expected_revision,
                    idempotency_key=f"owner-revision:{fluctlight_id}:{uuid4().hex}",
                    reason=request.reason,
                )
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (ValueError, FluctlightLifecycleError, RevisionConflictError) as exc:
            raise HTTPException(status_code=422, detail="foundation_revision_failed") from exc
        return {
            "id": revision.id,
            "revision": revision.revision,
            "status": revision.status.value,
            "changes": dict(revision.changes),
        }

    @app.post("/internal/fluctlights/{fluctlight_id}/foundation-revisions/{revision_id}/accept")
    async def accept_foundation_revision(
        fluctlight_id: str,
        revision_id: str,
        request: FoundationRevisionAcceptRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            service = require_fluctlight_service(current)
            if not await service.can_actor_access(fluctlight_id, actor.actor_id):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            if not any(
                revision.id == revision_id
                for revision in await service.revision_history(fluctlight_id)
            ):
                raise HTTPException(status_code=404, detail="foundation_revision_not_found")
            snapshot = await service.accept_revision(
                revision_id=revision_id,
                actor_id=actor.actor_id,
                expected_revision=request.expected_revision,
                reason=request.reason,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (ValueError, FluctlightLifecycleError, RevisionConflictError) as exc:
            raise HTTPException(
                status_code=422, detail="foundation_revision_accept_failed"
            ) from exc
        return {"id": snapshot.id, "current_revision": snapshot.current_revision}

    @app.post("/internal/fluctlights/{fluctlight_id}/foundation-revisions/{revision_id}/reject")
    async def reject_foundation_revision(
        fluctlight_id: str,
        revision_id: str,
        request: FoundationRevisionAcceptRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            service = require_fluctlight_service(current)
            if not await service.can_actor_access(fluctlight_id, actor.actor_id):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            revision = await service.reject_revision(
                revision_id=revision_id,
                actor_id=actor.actor_id,
                expected_revision=request.expected_revision,
                reason=request.reason,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (ValueError, FluctlightLifecycleError, RevisionConflictError) as exc:
            raise HTTPException(
                status_code=422, detail="foundation_revision_reject_failed"
            ) from exc
        return {"id": revision.id, "status": revision.status.value}

    @app.post("/internal/fluctlights/{fluctlight_id}/foundation-revisions/rollback")
    async def rollback_foundation_revision(
        fluctlight_id: str,
        request: FoundationRevisionRollbackRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            service = require_fluctlight_service(current)
            if not await service.can_actor_access(fluctlight_id, actor.actor_id):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            snapshot = await service.rollback_revision(
                fluctlight_id=fluctlight_id,
                target_revision=request.target_revision,
                actor_id=actor.actor_id,
                expected_revision=request.expected_revision,
                reason=request.reason,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (ValueError, FluctlightLifecycleError, RevisionConflictError) as exc:
            raise HTTPException(
                status_code=422, detail="foundation_revision_rollback_failed"
            ) from exc
        return {"id": snapshot.id, "current_revision": snapshot.current_revision}

    @app.put("/internal/memories/{memory_id}")
    async def revise_memory(
        memory_id: str,
        request: MemoryRevisionRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            memory_service = require_memory_service(current)
            memory = await memory_service.get(memory_id)
            if not await require_fluctlight_service(current).can_actor_access(
                memory.owner_fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="memory_not_found")
            revision = await memory_service.revise(
                memory_id,
                expected_revision=request.expected_revision,
                content=request.content,
                actor_id=actor.actor_id,
                evidence_refs=tuple(request.evidence_refs),
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (KeyError, ValueError) as exc:
            raise HTTPException(status_code=422, detail="memory_revision_failed") from exc
        return {"memory_id": revision.memory_id, "revision": revision.revision}

    @app.post("/internal/memories/{memory_id}/forget")
    async def forget_memory(
        memory_id: str,
        request: MemoryForgetRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            memory_service = require_memory_service(current)
            memory = await memory_service.get(memory_id)
            if not await require_fluctlight_service(current).can_actor_access(
                memory.owner_fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="memory_not_found")
            revision = await memory_service.forget(
                memory_id,
                expected_revision=request.expected_revision,
                actor_id=actor.actor_id,
                evidence_refs=tuple(request.evidence_refs),
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (KeyError, ValueError) as exc:
            raise HTTPException(status_code=422, detail="memory_forget_failed") from exc
        return {"memory_id": revision.memory_id, "revision": revision.revision}

    @app.post("/internal/fluctlights/{fluctlight_id}/relationships/rollback")
    async def rollback_relationship(
        fluctlight_id: str,
        request: RelationshipRollbackRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_fluctlight_service(current).can_actor_access(
                fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            relationship = await require_relationship_service(current).rollback(
                fluctlight_id,
                request.target_actor_id,
                target_revision=request.target_revision,
                expected_revision=request.expected_revision,
                actor_id=actor.actor_id,
                evidence_refs=tuple(request.evidence_refs),
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (KeyError, ValueError) as exc:
            raise HTTPException(status_code=422, detail="relationship_rollback_failed") from exc
        return {"id": relationship.id, "revision": relationship.revision}

    @app.get("/internal/fluctlights/{fluctlight_id}/autonomy-actions")
    async def list_autonomy_actions(
        fluctlight_id: str,
        limit: int = 100,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[dict[str, object]]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_fluctlight_service(current).can_actor_access(
                fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            actions = await require_autonomy_service(current).list_for_fluctlight(
                fluctlight_id, limit=limit
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        return [
            {
                "id": action.id,
                "action_type": action.action_type,
                "status": action.status.value,
                "workflow_id": action.workflow_id,
                "created_at": action.created_at.isoformat(),
            }
            for action in actions
        ]

    @app.post("/internal/autonomy-actions/{action_id}/govern")
    async def govern_autonomy_action(
        action_id: str,
        request: AutonomousActionGovernanceRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            autonomy = require_autonomy_service(current)
            action = await autonomy.get_action(action_id)
            if not await require_fluctlight_service(current).can_actor_access(
                action.fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="autonomy_action_not_found")
            result = await autonomy.govern(
                action_id,
                to_status=request.status,
                actor_id=actor.actor_id,
                reason=request.reason,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (KeyError, ValueError) as exc:
            raise HTTPException(status_code=422, detail="autonomy_governance_failed") from exc
        return {"id": result.id, "status": result.status.value}

    @app.post("/internal/fluctlights/{fluctlight_id}/events")
    async def create_life_event(
        fluctlight_id: str,
        request: LifeEventRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_fluctlight_service(current).can_actor_access(
                fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            event = await require_life_world_service(current).create_event(
                WorldEvent(
                    id=f"life_event_{uuid4().hex}",
                    fluctlight_id=fluctlight_id,
                    kind=request.kind,
                    start_at=request.start_at,
                    end_at=request.end_at,
                    scene=request.scene,
                    activity=request.activity,
                    location=request.location,
                    evidence_refs=tuple(request.evidence_refs),
                )
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except ValueError as exc:
            raise HTTPException(status_code=422, detail="life_event_failed") from exc
        return {"id": event.id, "status": event.status.value}

    @app.post(
        "/internal/fluctlights/{fluctlight_id}/events/{event_id}/cancel",
        status_code=204,
        response_model=None,
    )
    async def cancel_life_event(
        fluctlight_id: str,
        event_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_fluctlight_service(current).can_actor_access(
                fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            await require_life_world_service(current).cancel_event(
                event_id, fluctlight_id=fluctlight_id
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except KeyError as exc:
            raise HTTPException(status_code=422, detail="life_event_cancel_failed") from exc

    @app.put("/internal/fluctlights/{fluctlight_id}/presence")
    async def set_life_presence(
        fluctlight_id: str,
        request: PresenceRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_fluctlight_service(current).can_actor_access(
                fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            presence = await require_life_world_service(current).set_presence(
                fluctlight_id,
                PresenceOverlay(
                    actor_id=actor.actor_id,
                    current_task=request.current_task,
                    user_presence=request.user_presence,
                ),
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        return {
            "current_task": presence.current_task,
            "user_presence": presence.user_presence,
        }

    @app.post("/internal/fluctlights/{fluctlight_id}/schedules")
    async def accept_life_schedule(
        fluctlight_id: str,
        request: ScheduleRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_fluctlight_service(current).can_actor_access(
                fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            proposal = ScheduleVersion(
                id=f"schedule_{uuid4().hex}",
                fluctlight_id=fluctlight_id,
                local_date=request.local_date,
                timezone=request.timezone,
                items=tuple(
                    ScheduleItem(
                        id=f"schedule_item_{uuid4().hex}",
                        start_at=item.start_at,
                        end_at=item.end_at,
                        activity=item.activity,
                        scene=item.scene,
                        item_type=item.item_type,
                        status=item.status,
                        priority=item.priority,
                        flexibility=item.flexibility,
                        interruption_cost=item.interruption_cost,
                    )
                    for item in request.items
                ),
                generated_from="owner",
                evidence_refs=tuple(request.evidence_refs),
                revision=(request.expected_revision or 0) + 1,
            )
            life_world = require_life_world_service(current)
            if request.completed_before is None:
                accepted = await life_world.accept_schedule(
                    proposal, expected_revision=request.expected_revision
                )
            else:
                accepted = await life_world.replan(
                    proposal,
                    completed_before=request.completed_before,
                    expected_revision=request.expected_revision,
                )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (ValueError, ScheduleValidationError) as exc:
            raise HTTPException(status_code=422, detail="schedule_accept_failed") from exc
        return {
            "id": accepted.id,
            "local_date": accepted.local_date.isoformat(),
            "revision": accepted.revision,
            "status": accepted.status.value,
        }

    @app.post(
        "/internal/fluctlights/{fluctlight_id}/schedules/{schedule_id}/cancel",
        status_code=204,
        response_model=None,
    )
    async def cancel_life_schedule(
        fluctlight_id: str,
        schedule_id: str,
        request: ScheduleCancelRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_fluctlight_service(current).can_actor_access(
                fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            await require_life_world_service(current).cancel_schedule(
                schedule_id,
                fluctlight_id=fluctlight_id,
                expected_revision=request.expected_revision,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except ScheduleValidationError as exc:
            raise HTTPException(status_code=422, detail="schedule_cancel_failed") from exc

    @app.get("/internal/fluctlights/{fluctlight_id}/moments")
    async def fluctlight_moments(
        fluctlight_id: str,
        limit: int = 50,
        include_hidden: bool = False,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[dict[str, object]]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_fluctlight_service(current).can_actor_access(
                fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            rows = await require_moments_service(current).feed(
                owner_fluctlight_id=fluctlight_id,
                actor_id=actor.actor_id,
                limit=limit,
                include_hidden=include_hidden,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        moments = require_moments_service(current)
        response: list[dict[str, object]] = []
        for item in rows:
            reaction_count, viewer_reaction = await moments.reaction_summary(
                item.id, actor.actor_id
            )
            response.append(
                {
                    "id": item.id,
                    "owner_fluctlight_id": item.owner_fluctlight_id,
                    "author_actor_id": item.author_actor_id,
                    "text": item.text,
                    "visibility": item.visibility.value,
                    "status": item.status.value,
                    "media_asset_ids": list(item.media_asset_ids),
                    "media": await require_media_service(current).summaries(
                        item.media_asset_ids, actor_id=actor.actor_id
                    ),
                    "created_at": item.created_at.isoformat(),
                    "comments": [
                        {
                            "id": comment.id,
                            "author_actor_id": comment.author_actor_id,
                            "text": comment.text,
                            "created_at": comment.created_at.isoformat(),
                        }
                        for comment in await moments.comments(item.id)
                    ],
                    "reaction_count": reaction_count,
                    "viewer_reaction": viewer_reaction.value if viewer_reaction else None,
                }
            )
        return response

    @app.get("/internal/moments")
    async def global_moments(
        limit: int = 50,
        include_hidden: bool = False,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[dict[str, object]]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            fluctlight_ids = tuple(
                snapshot.id
                for snapshot in await require_fluctlight_service(current).list_for_actor(
                    actor.actor_id
                )
            )
            moments = require_moments_service(current)
            rows = await moments.global_feed(
                owner_fluctlight_ids=fluctlight_ids,
                limit=limit,
                include_hidden=include_hidden,
            )
            unread_counts = await moments.unread_counts(
                owner_fluctlight_ids=fluctlight_ids,
                actor_id=actor.actor_id,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        response: list[dict[str, object]] = []
        for item in rows:
            reaction_count, viewer_reaction = await moments.reaction_summary(
                item.id, actor.actor_id
            )
            response.append(
                {
                    "id": item.id,
                    "owner_fluctlight_id": item.owner_fluctlight_id,
                    "author_actor_id": item.author_actor_id,
                    "text": item.text,
                    "visibility": item.visibility.value,
                    "status": item.status.value,
                    "media_asset_ids": list(item.media_asset_ids),
                    "media": await require_media_service(current).summaries(
                        item.media_asset_ids, actor_id=actor.actor_id
                    ),
                    "created_at": item.created_at.isoformat(),
                    "comments": [
                        {
                            "id": comment.id,
                            "author_actor_id": comment.author_actor_id,
                            "text": comment.text,
                            "created_at": comment.created_at.isoformat(),
                        }
                        for comment in await moments.comments(item.id)
                    ],
                    "reaction_count": reaction_count,
                    "viewer_reaction": viewer_reaction.value if viewer_reaction else None,
                    "unread_count": unread_counts.get(item.owner_fluctlight_id, 0),
                }
            )
        return response

    @app.post(
        "/internal/fluctlights/{fluctlight_id}/moments/read",
        status_code=204,
        response_model=None,
    )
    async def mark_fluctlight_moments_read(
        fluctlight_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            if not await require_fluctlight_service(current).can_actor_access(
                fluctlight_id, actor.actor_id
            ):
                raise HTTPException(status_code=404, detail="fluctlight_not_found")
            await require_moments_service(current).mark_read(fluctlight_id, actor.actor_id)
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc

    @app.post("/internal/moments/{moment_id}/comments")
    async def comment_on_moment(
        moment_id: str,
        request: MomentCommentRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            moments = require_moments_service(current)
            owner_fluctlight_id = await moments.owner_fluctlight_id(moment_id)
            if not await require_fluctlight_service(current).can_actor_access(
                owner_fluctlight_id, actor.actor_id
            ):
                raise PermissionError("moment is not accessible")
            comment = await moments.comment(
                MomentComment(
                    id=f"moment_comment_{uuid4().hex}",
                    moment_id=moment_id,
                    author_actor_id=actor.actor_id,
                    text=request.text,
                ),
                actor_id=actor.actor_id,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (KeyError, PermissionError, ValueError) as exc:
            raise HTTPException(status_code=422, detail="moment_comment_failed") from exc
        return {"id": comment.id, "moment_id": comment.moment_id, "text": comment.text}

    @app.post("/internal/moments/{moment_id}/reactions")
    async def react_to_moment(
        moment_id: str,
        request: MomentReactionRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            moments = require_moments_service(current)
            owner_fluctlight_id = await moments.owner_fluctlight_id(moment_id)
            if not await require_fluctlight_service(current).can_actor_access(
                owner_fluctlight_id, actor.actor_id
            ):
                raise PermissionError("moment is not accessible")
            reaction = await moments.react(
                MomentReaction(moment_id=moment_id, actor_id=actor.actor_id, kind=request.kind),
                actor_id=actor.actor_id,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (KeyError, PermissionError, ValueError) as exc:
            raise HTTPException(status_code=422, detail="moment_reaction_failed") from exc
        return {"moment_id": reaction.moment_id, "kind": reaction.kind.value}

    async def set_moment_status(
        moment_id: str,
        status: MomentStatus,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            moments = require_moments_service(current)
            owner_fluctlight_id = await moments.owner_fluctlight_id(moment_id)
            if not await require_fluctlight_service(current).can_actor_access(
                owner_fluctlight_id, actor.actor_id
            ):
                raise PermissionError("moment is not accessible")
            await moments.set_status(moment_id, status)
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except (KeyError, PermissionError) as exc:
            raise HTTPException(status_code=422, detail="moment_status_failed") from exc

    @app.post("/internal/moments/{moment_id}/hide", status_code=204, response_model=None)
    async def hide_moment(
        moment_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        await set_moment_status(
            moment_id,
            MomentStatus.HIDDEN,
            x_fluctlight_service_key,
            x_fluctlight_human_session,
        )

    @app.post("/internal/moments/{moment_id}/restore", status_code=204, response_model=None)
    async def restore_moment(
        moment_id: str,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        await set_moment_status(
            moment_id,
            MomentStatus.VISIBLE,
            x_fluctlight_service_key,
            x_fluctlight_human_session,
        )

    @app.get("/internal/conversations/{conversation_id}/history")
    async def conversation_history(
        conversation_id: str,
        before_sequence: int | None = None,
        limit: int = 50,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> dict[str, object]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            page = await require_conversation_service(current).history(
                conversation_id,
                actor_id=actor.actor_id,
                before_sequence=before_sequence,
                limit=limit,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except ConversationNotFoundError as exc:
            raise HTTPException(status_code=404, detail="conversation_not_found") from exc
        except ConversationAuthorizationError as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc
        return page_response(page)

    @app.post(
        "/internal/conversations/{conversation_id}/read", status_code=204, response_model=None
    )
    async def mark_conversation_read(
        conversation_id: str,
        request: ReadPositionRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> None:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            await require_conversation_service(current).mark_read(
                conversation_id,
                actor_id=actor.actor_id,
                read_sequence=request.read_sequence,
                delivered_sequence=request.delivered_sequence,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except ConversationNotFoundError as exc:
            raise HTTPException(status_code=404, detail="conversation_not_found") from exc
        except ConversationAuthorizationError as exc:
            raise HTTPException(status_code=403, detail="forbidden") from exc

    @app.post("/internal/conversations/{conversation_id}/turn")
    async def accept_conversation_turn(
        conversation_id: str,
        request: ConversationTurnRequest,
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ):
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        turn = ConversationTurn(
            conversation_id=conversation_id,
            actor_id=actor.actor_id,
            text=request.text,
            fluctlight_id=request.fluctlight_id,
            attachment_refs=tuple(request.attachment_refs),
            idempotency_key=request.idempotency_key,
            turn_id=request.turn_id or f"turn_{uuid4().hex}",
        )
        return turn_stream_response(require_conversation_service(current), turn)

    @app.get("/internal/media/{asset_id}")
    async def read_media(
        asset_id: str,
        range_header: str | None = Header(default=None, alias="Range"),
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> Response:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            authorized = await require_media_service(current).authorize_read(
                asset_id,
                actor_id=actor.actor_id,
                allowed_range=range_header or "full",
            )
            body, etag = await require_media_service(current).read_object(authorized)
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except RangeNotSatisfiable as exc:
            return Response(
                content=b"",
                status_code=416,
                headers={"content-range": f"bytes */{exc.byte_size}"},
            )
        except (KeyError, PermissionError, ValueError) as exc:
            raise HTTPException(status_code=404, detail="media_unavailable") from exc
        headers = {
            "accept-ranges": "bytes",
            "content-length": str(len(body)),
        }
        if range_header:
            headers["content-range"] = (
                f"{authorized.grant.allowed_range}/{authorized.asset.byte_size}"
            )
        if etag:
            headers["etag"] = etag
        return Response(
            content=body,
            media_type=authorized.asset.mime_type,
            headers=headers,
            status_code=206 if range_header else 200,
        )

    return app
