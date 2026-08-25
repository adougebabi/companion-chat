import asyncio

import pytest
from fluctlight_core.providers.contracts import CapabilityReport, ModelRole
from fluctlight_core.providers.service import (
    ProviderConfigurationError,
    ProviderRoleService,
    RoleAssignment,
)


async def available(assignment: RoleAssignment) -> CapabilityReport:
    return CapabilityReport(role=assignment.role, available=True, capability_version="fake-v1")


async def unavailable(assignment: RoleAssignment) -> CapabilityReport:
    return CapabilityReport(role=assignment.role, available=False)


def test_role_is_not_available_until_its_own_preflight_succeeds() -> None:
    assignment = RoleAssignment(ModelRole.REFLECTION, "endpoint", "model", 100, 10)
    service = ProviderRoleService(available)
    with pytest.raises(ProviderConfigurationError):
        service.require(ModelRole.REFLECTION)
    asyncio.run(service.assign_and_preflight(assignment))
    assert service.require(ModelRole.REFLECTION) == assignment


def test_failed_role_never_falls_back_to_another_role() -> None:
    assignment = RoleAssignment(ModelRole.COGNITIVE_ASSESSMENT, "endpoint", "model", 100, 10)
    service = ProviderRoleService(unavailable)
    with pytest.raises(ProviderConfigurationError):
        asyncio.run(service.assign_and_preflight(assignment))
    with pytest.raises(ProviderConfigurationError):
        service.require(ModelRole.ACTION_REALIZATION)
