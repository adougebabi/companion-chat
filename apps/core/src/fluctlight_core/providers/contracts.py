"""Transport-neutral Provider role declarations and safe provenance."""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


class ModelRole(StrEnum):
    INITIALIZATION = "initialization"
    COGNITIVE_ASSESSMENT = "cognitive_assessment"
    ACTION_REALIZATION = "action_realization"
    REFLECTION = "reflection"
    EMBEDDING = "embedding"
    MEDIA_PROMPT = "media_prompt"


@dataclass(frozen=True, slots=True)
class CapabilityReport:
    role: ModelRole
    available: bool
    capability_version: str | None = None
    detail: str | None = None


@dataclass(frozen=True, slots=True)
class ProviderProvenance:
    role: ModelRole
    endpoint_id: str
    model_id: str
    prompt_version: str
    schema_version: str
    correlation_id: str
    token_budget: int
