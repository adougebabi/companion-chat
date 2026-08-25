"""Explicit role preflight policy; adapters normalize transport only."""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from uuid import uuid4

from sqlalchemy import delete, insert, select

from fluctlight_core.actors.service import AuthService, ResolvedHumanActor
from fluctlight_core.platform.persistence import UnitOfWorkFactory
from fluctlight_core.settings.crypto import SecretValue
from fluctlight_core.settings.service import SettingsService

from . import schema
from .contracts import CapabilityReport, ModelRole, ProviderProvenance


class ProviderConfigurationError(RuntimeError):
    pass


@dataclass(frozen=True, slots=True)
class RoleAssignment:
    role: ModelRole
    endpoint_id: str
    model_id: str
    token_budget: int
    timeout_seconds: int


Preflight = Callable[[RoleAssignment], Awaitable[CapabilityReport]]


@dataclass(frozen=True, slots=True)
class ProviderEndpoint:
    endpoint_id: str
    kind: str
    base_url: str
    secret_purpose: str


EndpointPreflight = Callable[
    [RoleAssignment, ProviderEndpoint, SecretValue], Awaitable[CapabilityReport]
]


class ProviderRoleService:
    def __init__(self, preflight: Preflight) -> None:
        self._preflight = preflight
        self._assignments: dict[ModelRole, RoleAssignment] = {}
        self._reports: dict[ModelRole, CapabilityReport] = {}

    async def assign_and_preflight(self, assignment: RoleAssignment) -> CapabilityReport:
        if assignment.token_budget <= 0 or assignment.timeout_seconds <= 0:
            raise ProviderConfigurationError("role budget and timeout must be positive")
        report = await self._preflight(assignment)
        if report.role != assignment.role or not report.available:
            raise ProviderConfigurationError("required role capability preflight failed")
        self._assignments[assignment.role] = assignment
        self._reports[assignment.role] = report
        return report

    def require(self, role: ModelRole) -> RoleAssignment:
        assignment = self._assignments.get(role)
        if (
            assignment is None
            or not self._reports.get(role, CapabilityReport(role, False)).available
        ):
            raise ProviderConfigurationError(f"role {role.value} is unavailable")
        return assignment


class ProviderConfigurationService:
    """Owner-authorized durable endpoint/role metadata without secret exposure."""

    def __init__(
        self,
        unit_of_work: UnitOfWorkFactory,
        auth: AuthService,
        settings: SettingsService,
        preflight: EndpointPreflight,
    ) -> None:
        self._unit_of_work = unit_of_work
        self._auth = auth
        self._settings = settings
        self._preflight = preflight

    async def configure_endpoint(
        self,
        actor: ResolvedHumanActor,
        *,
        endpoint_id: str,
        kind: str,
        base_url: str,
        secret_purpose: str,
    ) -> None:
        await self._require_owner(actor)
        if not endpoint_id or not kind or not base_url.startswith(("http://", "https://")):
            raise ProviderConfigurationError("endpoint configuration is invalid")
        async with self._unit_of_work.begin(command_id=f"provider-endpoint:{uuid4()}") as tx:
            await tx.session.execute(
                delete(schema.provider_endpoints).where(
                    schema.provider_endpoints.c.id == endpoint_id
                )
            )
            await tx.session.execute(
                insert(schema.provider_endpoints).values(
                    id=endpoint_id,
                    kind=kind,
                    base_url=base_url,
                    secret_purpose=secret_purpose,
                    capability_status="unknown",
                )
            )
            await tx.commit()

    async def configure_role(
        self, actor: ResolvedHumanActor, assignment: RoleAssignment
    ) -> CapabilityReport:
        await self._require_owner(actor)
        async with self._unit_of_work.begin(command_id=f"provider-endpoint-read:{uuid4()}") as tx:
            endpoint = (
                (
                    await tx.session.execute(
                        select(schema.provider_endpoints).where(
                            schema.provider_endpoints.c.id == assignment.endpoint_id
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if endpoint is None:
                raise ProviderConfigurationError("provider endpoint is not configured")
        endpoint_record = ProviderEndpoint(
            endpoint_id=endpoint["id"],
            kind=endpoint["kind"],
            base_url=endpoint["base_url"],
            secret_purpose=endpoint["secret_purpose"],
        )
        secret = await self._settings.resolve_provider_secret(endpoint_record.secret_purpose)
        report = await self._preflight(assignment, endpoint_record, secret)
        if report.role != assignment.role or not report.available:
            raise ProviderConfigurationError("required role capability preflight failed")
        async with self._unit_of_work.begin(command_id=f"provider-role:{uuid4()}") as tx:
            await tx.session.execute(
                delete(schema.model_roles).where(schema.model_roles.c.role == assignment.role.value)
            )
            await tx.session.execute(
                insert(schema.model_roles).values(
                    role=assignment.role.value,
                    provider_endpoint_id=assignment.endpoint_id,
                    model_id=assignment.model_id,
                    required_capabilities="[]",
                    token_budget=assignment.token_budget,
                    timeout_seconds=assignment.timeout_seconds,
                    retry_policy="{}",
                )
            )
            await tx.session.execute(
                insert(schema.provider_preflights).values(
                    id=f"preflight_{uuid4().hex}",
                    role=assignment.role.value,
                    result="available",
                    capability_version=report.capability_version,
                )
            )
            await tx.commit()
        return report

    async def record_provenance(self, provenance: ProviderProvenance) -> None:
        """Persist only safe execution metadata after a Provider call settles."""
        async with self._unit_of_work.begin(command_id=f"provider-provenance:{uuid4()}") as tx:
            await tx.session.execute(
                insert(schema.provider_provenance).values(
                    id=f"provenance_{uuid4().hex}",
                    role=provenance.role.value,
                    endpoint_id=provenance.endpoint_id,
                    model_id=provenance.model_id,
                    prompt_version=provenance.prompt_version,
                    schema_version=provenance.schema_version,
                    correlation_id=provenance.correlation_id,
                    token_budget=provenance.token_budget,
                )
            )
            await tx.commit()

    async def _require_owner(self, actor: ResolvedHumanActor) -> None:
        if not await self._auth.is_owner(actor):
            raise ProviderConfigurationError("forbidden")
