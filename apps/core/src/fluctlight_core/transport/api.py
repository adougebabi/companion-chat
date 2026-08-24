"""FastAPI composition root for internal Core platform routes."""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from contextlib import asynccontextmanager
from dataclasses import dataclass

from fastapi import FastAPI, Header, HTTPException
from pydantic import BaseModel
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncEngine

from fluctlight_core.platform.configuration import ConfigurationError, PlatformSettings, RuntimeRole
from fluctlight_core.platform.persistence import (
    MigrationRevisionError,
    create_engine,
    verify_revision,
)

EXPECTED_REVISION = "0001_platform"


@dataclass(slots=True)
class ApiDependencies:
    settings: PlatformSettings
    engine: AsyncEngine
    verify_database: Callable[[], Awaitable[None]]


class PlatformPingResponse(BaseModel):
    status: str
    role: str


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

            resolved = ApiDependencies(
                settings=settings, engine=engine, verify_database=verify_database
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

    return app
