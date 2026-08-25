"""FastAPI composition root for internal Core platform routes."""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from contextlib import asynccontextmanager
from dataclasses import dataclass
from uuid import uuid4

import boto3  # type: ignore[import-untyped]
from fastapi import FastAPI, Header, HTTPException
from fastapi.responses import Response
from pydantic import BaseModel, Field
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncEngine

from fluctlight_core.actors.service import AuthError, AuthService
from fluctlight_core.cognition.service import CognitionService
from fluctlight_core.cognition.turn_responder import CognitionTurnResponder
from fluctlight_core.conversations.contracts import (
    ConversationAuthorizationError,
    ConversationConflictError,
    ConversationNotFoundError,
    ConversationTurn,
)
from fluctlight_core.conversations.service import ConversationService
from fluctlight_core.diagnostics.service import DiagnosticsAuthorizationError, DiagnosticsService
from fluctlight_core.fluctlights.contracts import CreateFluctlight, Identity
from fluctlight_core.fluctlights.service import FluctlightService
from fluctlight_core.media.service import MediaService
from fluctlight_core.platform.configuration import ConfigurationError, PlatformSettings, RuntimeRole
from fluctlight_core.platform.object_storage import S3ObjectStorage
from fluctlight_core.platform.persistence import (
    MigrationRevisionError,
    UnitOfWorkFactory,
    create_engine,
    verify_revision,
)
from fluctlight_core.providers.adapters import OpenAICompatibleAdapter
from fluctlight_core.providers.contracts import ModelRole
from fluctlight_core.providers.runtime import ConfiguredProviderRuntime
from fluctlight_core.providers.service import (
    ProviderConfigurationError,
    ProviderConfigurationService,
    RoleAssignment,
)
from fluctlight_core.settings.crypto import SecretCodec, SecretConfigurationError
from fluctlight_core.settings.service import SafeSettingsView, SettingsError, SettingsService
from fluctlight_core.transport.conversations import (
    ConversationCreateRequest,
    ConversationTurnRequest,
    ReadPositionRequest,
    page_response,
    turn_stream_response,
)

EXPECTED_REVISION = "0012_t12_consumer_effects"


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


class PlatformPingResponse(BaseModel):
    status: str
    role: str


class SetupRequest(BaseModel):
    setup_token: str = Field(min_length=16)
    password: str = Field(min_length=12)


class LoginRequest(BaseModel):
    password: str = Field(min_length=12)


class PasswordResetRequest(BaseModel):
    password: str = Field(min_length=12)


class FluctlightCreateRequest(BaseModel):
    id: str | None = Field(default=None, min_length=1, max_length=128)
    name: str | None = Field(default=None, max_length=256)


class SessionResponse(BaseModel):
    authenticated: bool
    actor_id: str | None = None
    session_token: str | None = None


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


class DiagnosticEventResponse(BaseModel):
    id: str
    event_type: str
    severity: str
    fluctlight_id: str | None = None
    causation_id: str | None = None
    correlation_id: str
    payload: dict[str, object]
    created_at: str | None = None


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
            )
            provider_runtime = ConfiguredProviderRuntime(
                unit_of_work,
                settings_service,
                adapter=provider_adapter,
                provenance_recorder=provider_service.record_provenance,
            )
            cognition = CognitionService(
                unit_of_work,
                provider_runtime,
                provider_runtime,
                reflection_provider=provider_runtime,
            )
            fluctlights = FluctlightService(unit_of_work)
            object_client = boto3.client(
                "s3",
                endpoint_url=settings.s3_endpoint,
                region_name=settings.s3_region,
                aws_access_key_id=settings.s3_access_key,
                aws_secret_access_key=settings.s3_secret_key,
                use_ssl=settings.s3_use_ssl,
            )

            resolved = ApiDependencies(
                settings=settings,
                engine=engine,
                verify_database=verify_database,
                auth=auth,
                settings_service=settings_service,
                providers=provider_service,
                diagnostics=DiagnosticsService(unit_of_work),
                conversations=ConversationService(unit_of_work, CognitionTurnResponder(cognition)),
                media=MediaService(
                    unit_of_work,
                    S3ObjectStorage(object_client, settings.s3_bucket),
                    fluctlights.can_actor_access,
                ),
                fluctlights=fluctlights,
            )
        yield
        if dependencies is None and resolved is not None:
            await resolved.engine.dispose()

    app = FastAPI(title="Fluctlight Core Platform", version="0.1.0", lifespan=lifespan)

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
        except (AuthError, ProviderConfigurationError) as exc:
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
            page = await require_conversation_service(current).create(
                actor_id=actor.actor_id,
                participant_actor_ids=request.participant_actor_ids,
                title=request.title,
            )
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        except ConversationConflictError as exc:
            raise HTTPException(status_code=409, detail="conversation_conflict") from exc
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

    @app.get("/internal/fluctlights")
    async def list_fluctlights(
        x_fluctlight_service_key: str | None = Header(default=None),
        x_fluctlight_human_session: str | None = Header(default=None),
    ) -> list[dict[str, object]]:
        current = require_service_key(x_fluctlight_service_key)
        try:
            actor = await require_auth_service(current).resolve(x_fluctlight_human_session)
            snapshots = await require_fluctlight_service(current).list_for_actor(actor.actor_id)
        except AuthError as exc:
            raise HTTPException(status_code=401, detail="unauthenticated") from exc
        return [
            {
                "id": snapshot.id,
                "identity": snapshot.identity.as_payload(),
                "personality": snapshot.personality.as_payload(),
                "behavioral_policy": snapshot.behavioral_policy.as_payload(),
                "status": snapshot.status.value,
                "current_revision": snapshot.current_revision,
            }
            for snapshot in snapshots
        ]

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
            "status": snapshot.status.value,
            "current_revision": snapshot.current_revision,
        }

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
        except (KeyError, PermissionError, ValueError) as exc:
            raise HTTPException(status_code=404, detail="media_unavailable") from exc
        headers = {
            "accept-ranges": "bytes",
            "content-length": str(len(body)),
        }
        if range_header:
            headers["content-range"] = (
                f"{range_header.removeprefix('bytes=')}/{authorized.asset.byte_size}"
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
