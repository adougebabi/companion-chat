from __future__ import annotations

import json

from fluctlight_core.platform.configuration import PlatformSettings, RuntimeRole
from fluctlight_core.platform.outbox import OutboxEvent, event_envelope


def environment(role: str = "api") -> dict[str, str]:
    return {
        "FLUCTLIGHT_ENV": "test",
        "FLUCTLIGHT_ROLE": role,
        "DATABASE_URL": "postgresql://fluctlight:secret@postgres/fluctlight",
        "REDIS_URL": "redis://redis:6379/0",
        "S3_ENDPOINT": "http://minio:9000",
        "S3_REGION": "us-east-1",
        "S3_BUCKET": "fluctlight-media",
        "S3_ACCESS_KEY": "access",
        "S3_SECRET_KEY": "secret",
        "FLUCTLIGHT_CORE_SERVICE_KEY": "service-key",
        "FLUCTLIGHT_SETTINGS_KEY": "settings-key",
        "TEMPORAL_ADDRESS": "temporal:7233",
        "TEMPORAL_NAMESPACE": "default",
    }


def test_platform_settings_require_the_configured_role() -> None:
    settings = PlatformSettings.from_environ(environment())
    settings.require_role(RuntimeRole.API)
    assert settings.api_port == 8080


def test_outbox_envelope_uses_stable_json_payload() -> None:
    envelope = event_envelope(
        OutboxEvent(
            id="event-1",
            kind="platform.created",
            aggregate_type="platform",
            aggregate_id="platform-1",
            causation_id="command-1",
            correlation_id="correlation-1",
            idempotency_key="command-1",
            payload={"z": 1, "a": "value"},
            attempt_policy={"max_attempts": 3},
        )
    )
    assert json.loads(envelope["payload"]) == {"a": "value", "z": 1}
