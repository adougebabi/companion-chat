"""Owner-authorized runtime settings with encrypted write-only secrets."""

from __future__ import annotations

import json
from dataclasses import dataclass
from uuid import uuid4

from sqlalchemy import delete, insert, select

from fluctlight_core.actors.service import AuthService, ResolvedHumanActor
from fluctlight_core.platform.persistence import UnitOfWorkFactory

from . import schema
from .crypto import EncryptedSecret, SecretCodec, SecretConfigurationError, SecretValue


class SettingsError(RuntimeError):
    pass


def _validate_runtime_setting(key: str, value: object) -> None:
    """Keep runtime settings explicit rather than accepting a silent KV bag."""

    if key == "media.comfyui":
        if not isinstance(value, dict):
            raise SettingsError("media.comfyui must be an object")
        base_url, workflow = value.get("baseUrl"), value.get("workflow")
        if not isinstance(base_url, str) or not base_url.startswith(("http://", "https://")):
            raise SettingsError("media.comfyui.baseUrl must be an HTTP URL")
        if not isinstance(workflow, dict) or not workflow:
            raise SettingsError("media.comfyui.workflow must be a non-empty object")
        return
    if key in {"product.autonomy", "diagnostics.retention", "media.h3"}:
        if not isinstance(value, dict):
            raise SettingsError(f"{key} must be an object")
        return
    raise SettingsError(f"{key} is not a registered runtime setting")


@dataclass(frozen=True, slots=True)
class SafeSettingsView:
    values: dict[str, object]
    configured_secrets: frozenset[str]


class SettingsService:
    def __init__(
        self, unit_of_work: UnitOfWorkFactory, codec: SecretCodec, auth: AuthService
    ) -> None:
        self._unit_of_work = unit_of_work
        self._codec = codec
        self._auth = auth

    async def read(self, actor: ResolvedHumanActor) -> SafeSettingsView:
        await self._require_owner(actor)
        async with self._unit_of_work.begin(command_id=f"settings-read:{uuid4()}") as tx:
            values = {
                row["key"]: json.loads(row["value_json"])
                for row in (await tx.session.execute(select(schema.runtime_settings))).mappings()
            }
            configured = frozenset(
                row["purpose"]
                for row in (
                    await tx.session.execute(select(schema.setting_secrets.c.purpose))
                ).mappings()
            )
        return SafeSettingsView(values, configured)

    async def update(
        self,
        actor: ResolvedHumanActor,
        *,
        values: dict[str, object],
        secrets: dict[str, str | None],
        clear_secrets: frozenset[str] = frozenset(),
    ) -> SafeSettingsView:
        await self._require_owner(actor)
        async with self._unit_of_work.begin(command_id=f"settings-update:{uuid4()}") as tx:
            for key, value in values.items():
                _validate_runtime_setting(key, value)
                await tx.session.execute(
                    delete(schema.runtime_settings).where(schema.runtime_settings.c.key == key)
                )
                await tx.session.execute(
                    insert(schema.runtime_settings).values(key=key, value_json=json.dumps(value))
                )
                await self._audit(tx, actor.actor_id, key, "updated")
            for purpose, plaintext in secrets.items():
                if purpose in clear_secrets:
                    raise SettingsError("a secret cannot be set and cleared in one patch")
                if plaintext is None or plaintext in {"", "configured"}:
                    continue
                encrypted = self._codec.encrypt(purpose=purpose, plaintext=plaintext)
                await tx.session.execute(
                    delete(schema.setting_secrets).where(
                        schema.setting_secrets.c.purpose == purpose
                    )
                )
                await tx.session.execute(
                    insert(schema.setting_secrets).values(
                        purpose=purpose, ciphertext=encrypted.ciphertext, nonce=encrypted.nonce
                    )
                )
                await self._audit(tx, actor.actor_id, purpose, "updated")
            for purpose in clear_secrets:
                await tx.session.execute(
                    delete(schema.setting_secrets).where(
                        schema.setting_secrets.c.purpose == purpose
                    )
                )
                await self._audit(tx, actor.actor_id, purpose, "cleared")
            await tx.commit()
        return await self.read(actor)

    async def resolve_provider_secret(self, purpose: str) -> SecretValue:
        secret = await self.resolve_optional_provider_secret(purpose)
        if secret is None:
            raise SecretConfigurationError("provider secret is not configured")
        return secret

    async def resolve_optional_provider_secret(self, purpose: str) -> SecretValue | None:
        """Resolve a configured Provider key, or explicitly allow no authentication."""

        async with self._unit_of_work.begin(command_id=f"settings-secret:{uuid4()}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.setting_secrets).where(
                            schema.setting_secrets.c.purpose == purpose
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
        if row is None:
            return None
        return self._codec.decrypt(EncryptedSecret(row["ciphertext"], row["nonce"], purpose))

    async def runtime_value(self, key: str) -> object | None:
        """Return one non-secret runtime setting for a trusted in-process adapter."""

        async with self._unit_of_work.begin(command_id=f"settings-runtime:{key}") as tx:
            value = await tx.session.scalar(
                select(schema.runtime_settings.c.value_json).where(
                    schema.runtime_settings.c.key == key
                )
            )
        return json.loads(value) if value is not None else None

    async def _require_owner(self, actor: ResolvedHumanActor) -> None:
        if not await self._auth.is_owner(actor):
            raise SettingsError("forbidden")

    async def _audit(self, tx, actor_id: str, field: str, result: str) -> None:
        await tx.session.execute(
            insert(schema.settings_audit).values(
                id=f"settings_audit_{uuid4().hex}", actor_id=actor_id, field=field, result=result
            )
        )
