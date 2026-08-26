from typing import cast

from fastapi.testclient import TestClient
from fluctlight_core.actors.service import AuthService, ResolvedHumanActor, SessionResult
from fluctlight_core.platform.configuration import PlatformSettings, RuntimeRole
from fluctlight_core.settings.service import SafeSettingsView, SettingsService
from fluctlight_core.transport.api import ApiDependencies, create_app
from sqlalchemy.ext.asyncio import AsyncEngine


class FakeAuth:
    async def setup(self, *, setup_token: str, password: str) -> SessionResult:
        return SessionResult("human-owner", "opaque-token", None)  # type: ignore[arg-type]

    async def login(self, *, password: str) -> SessionResult:
        return SessionResult("human-owner", "opaque-token", None)  # type: ignore[arg-type]

    async def resolve(self, token: str | None) -> ResolvedHumanActor:
        if token != "opaque-token":
            from fluctlight_core.actors.service import AuthError

            raise AuthError("unauthenticated")
        return ResolvedHumanActor("human-owner", "session-1")

    async def revoke_all(self, actor: ResolvedHumanActor) -> None:
        return None

    async def reset_password(self, actor: ResolvedHumanActor, *, password: str) -> None:
        return None


class FakeSettings:
    async def read(self, actor: ResolvedHumanActor) -> SafeSettingsView:
        return SafeSettingsView({"providerUrl": "http://provider"}, frozenset({"provider:key"}))

    async def update(self, actor: ResolvedHumanActor, **_: object) -> SafeSettingsView:
        return await self.read(actor)


class FakeProviders:
    def __init__(self) -> None:
        self.requested: tuple[ResolvedHumanActor, str] | None = None

    async def list_models(self, actor: ResolvedHumanActor, *, endpoint_id: str) -> tuple[str, ...]:
        self.requested = (actor, endpoint_id)
        return ("embedding-model", "general-model")


def dependencies() -> ApiDependencies:
    settings = PlatformSettings(
        environment="test",
        role=RuntimeRole.API,
        database_url="postgresql://unused",
        redis_url="redis://unused",
        s3_endpoint="http://unused",
        s3_region="test",
        s3_bucket="test",
        s3_access_key="test",
        s3_secret_key="test",
        s3_use_ssl=False,
        core_service_key="service-key",
        settings_key="unused",
        temporal_address="unused",
        temporal_namespace="test",
    )

    async def verify_database() -> None:
        return None

    return ApiDependencies(
        settings,
        cast(AsyncEngine, None),
        verify_database,
        cast(AuthService, FakeAuth()),
        cast(SettingsService, FakeSettings()),
    )


def test_internal_routes_require_service_identity_and_keep_settings_safe() -> None:
    with TestClient(create_app(dependencies())) as client:
        assert client.get("/internal/auth/session").status_code == 401
        session = client.get(
            "/internal/auth/session",
            headers={
                "x-fluctlight-service-key": "service-key",
                "x-fluctlight-human-session": "opaque-token",
            },
        )
        assert session.json() == {
            "authenticated": True,
            "actor_id": "human-owner",
            "session_token": None,
        }
        settings = client.get(
            "/internal/settings",
            headers={
                "x-fluctlight-service-key": "service-key",
                "x-fluctlight-human-session": "opaque-token",
            },
        )
        assert settings.status_code == 200
        assert settings.json() == {
            "values": {"providerUrl": "http://provider"},
            "configured_secrets": ["provider:key"],
        }
        assert "opaque-token" not in settings.text


def test_provider_model_listing_requires_internal_and_human_identity() -> None:
    dependencies_for_models = dependencies()
    providers = FakeProviders()
    dependencies_for_models.providers = cast(object, providers)  # type: ignore[assignment]
    with TestClient(create_app(dependencies_for_models)) as client:
        assert client.get("/internal/providers/endpoints/local/models").status_code == 401
        response = client.get(
            "/internal/providers/endpoints/local/models",
            headers={
                "x-fluctlight-service-key": "service-key",
                "x-fluctlight-human-session": "opaque-token",
            },
        )
    assert response.status_code == 200
    assert response.json() == {
        "endpoint_id": "local",
        "models": ["embedding-model", "general-model"],
    }
    assert providers.requested == (ResolvedHumanActor("human-owner", "session-1"), "local")
