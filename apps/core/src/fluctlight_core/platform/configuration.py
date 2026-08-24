"""Validated startup configuration for the clean-start deployment."""

from __future__ import annotations

import os
from dataclasses import dataclass
from enum import StrEnum


class RuntimeRole(StrEnum):
    API = "api"
    WORKER = "worker"


class ConfigurationError(ValueError):
    """A required startup value is missing or invalid."""


@dataclass(frozen=True, slots=True)
class PlatformSettings:
    environment: str
    role: RuntimeRole
    database_url: str
    redis_url: str
    s3_endpoint: str
    s3_region: str
    s3_bucket: str
    s3_access_key: str
    s3_secret_key: str
    s3_use_ssl: bool
    core_service_key: str
    settings_key: str
    temporal_address: str
    temporal_namespace: str
    api_host: str = "0.0.0.0"
    api_port: int = 8080

    @classmethod
    def from_environ(cls, environ: dict[str, str] | None = None) -> PlatformSettings:
        values = dict(os.environ) if environ is None else environ

        def required(name: str) -> str:
            value = values.get(name, "").strip()
            if not value:
                raise ConfigurationError(f"missing required configuration: {name}")
            return value

        try:
            role = RuntimeRole(required("FLUCTLIGHT_ROLE"))
        except ValueError as exc:
            raise ConfigurationError("FLUCTLIGHT_ROLE must be api or worker") from exc
        try:
            port = int(values.get("FLUCTLIGHT_API_PORT", "8080"))
        except ValueError as exc:
            raise ConfigurationError("FLUCTLIGHT_API_PORT must be an integer") from exc
        if not 1 <= port <= 65535:
            raise ConfigurationError("FLUCTLIGHT_API_PORT must be between 1 and 65535")
        use_ssl = values.get("S3_USE_SSL", "false").strip().lower() in {"1", "true", "yes"}
        return cls(
            environment=required("FLUCTLIGHT_ENV"),
            role=role,
            database_url=required("DATABASE_URL"),
            redis_url=required("REDIS_URL"),
            s3_endpoint=required("S3_ENDPOINT"),
            s3_region=required("S3_REGION"),
            s3_bucket=required("S3_BUCKET"),
            s3_access_key=required("S3_ACCESS_KEY"),
            s3_secret_key=required("S3_SECRET_KEY"),
            s3_use_ssl=use_ssl,
            core_service_key=required("FLUCTLIGHT_CORE_SERVICE_KEY"),
            settings_key=required("FLUCTLIGHT_SETTINGS_KEY"),
            temporal_address=required("TEMPORAL_ADDRESS"),
            temporal_namespace=required("TEMPORAL_NAMESPACE"),
            api_host=values.get("FLUCTLIGHT_API_HOST", "0.0.0.0"),
            api_port=port,
        )

    def require_role(self, role: RuntimeRole) -> None:
        if self.role != role:
            raise ConfigurationError(f"process role must be {role.value}")
