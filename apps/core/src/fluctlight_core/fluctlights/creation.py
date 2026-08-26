"""Clean-start Fluctlight creation lifecycle application service."""

from __future__ import annotations

from dataclasses import dataclass
from hashlib import sha256
from typing import Any, Protocol

from .contracts import (
    BehavioralPolicy,
    CreateFluctlight,
    FluctlightSnapshot,
    FoundationValidationError,
    Identity,
    InitializationMode,
    Personality,
    PersonalityUpdatePolicy,
)
from .service import FluctlightLifecycleError, FluctlightNotFoundError, FluctlightService


class CreationError(RuntimeError):
    """Raised when an analysis or activation request is not valid."""

    def __init__(self, message: str, *, code: str | None = None) -> None:
        super().__init__(message)
        self.code = code or "creation_invalid"


class InitializationAnalyzer(Protocol):
    async def analyze_initialization(self, description: str) -> dict[str, Any]: ...


class InitialScheduleInitializer(Protocol):
    async def ensure_for(self, fluctlight: FluctlightSnapshot) -> object: ...


@dataclass(frozen=True, slots=True)
class CreationPreview:
    identity: dict[str, Any]
    personality: dict[str, Any]
    behavioral_policy: dict[str, Any]
    provenance: dict[str, Any]

    def as_payload(self) -> dict[str, Any]:
        return {
            "initialization_mode": InitializationMode.LLM_DEFINED.value,
            "foundation": {
                "identity": self.identity,
                "personality": self.personality,
                "behavioral_policy": self.behavioral_policy,
            },
            "provenance": self.provenance,
        }


def _without_id(payload: dict[str, Any]) -> dict[str, Any]:
    result = dict(payload)
    result.pop("id", None)
    return result


def _personality_from_payload(payload: dict[str, Any]) -> Personality:
    values = dict(payload)
    update_policy = values.pop("update_policy", None)
    if update_policy is not None:
        if not isinstance(update_policy, dict):
            raise FoundationValidationError("personality.update_policy must be an object")
        values["update_policy"] = PersonalityUpdatePolicy(**update_policy)
    return Personality(**values)


class CreationLifecycleService:
    """Analyze without persistence; activate one validated foundation atomically."""

    def __init__(
        self,
        fluctlights: FluctlightService,
        analyzer: InitializationAnalyzer,
        schedule_initializer: InitialScheduleInitializer | None = None,
    ) -> None:
        self._fluctlights = fluctlights
        self._analyzer = analyzer
        self._schedule_initializer = schedule_initializer

    async def analyze_description(self, description: str) -> CreationPreview:
        if not isinstance(description, str) or not description.strip() or len(description) > 12_000:
            raise CreationError("description must contain 1 to 12000 characters")
        result = await self._analyzer.analyze_initialization(description.strip())
        foundation = result.get("foundation", result)
        if not isinstance(foundation, dict):
            raise CreationError("initialization response has no foundation")
        try:
            identity = Identity(id="preview", **_without_id(dict(foundation["identity"])))
            personality = _personality_from_payload(dict(foundation["personality"]))
            policy = BehavioralPolicy(**dict(foundation["behavioral_policy"]))
        except (KeyError, TypeError, FoundationValidationError) as exc:
            raise CreationError(
                "initialization response is not a valid Fluctlight foundation",
                code="initialization_foundation_invalid",
            ) from exc
        provenance = result.get("provenance", {})
        if not isinstance(provenance, dict):
            provenance = {}
        return CreationPreview(
            _without_id(identity.as_payload()),
            personality.as_payload(),
            policy.as_payload(),
            dict(provenance),
        )

    async def activate(
        self,
        *,
        actor_id: str,
        request_id: str,
        initialization_mode: InitializationMode,
        identity: dict[str, Any],
        personality: dict[str, Any] | None = None,
        behavioral_policy: dict[str, Any] | None = None,
    ) -> FluctlightSnapshot:
        if not isinstance(request_id, str) or not request_id.strip() or len(request_id) > 256:
            raise CreationError("request_id is required", code="activation_request_invalid")
        fluctlight_id = f"fluctlight_{sha256(f'{actor_id}:{request_id}'.encode()).hexdigest()[:32]}"
        mode = InitializationMode(initialization_mode)
        if mode is InitializationMode.BLANK_SLATE and (
            personality is not None or behavioral_policy is not None
        ):
            raise CreationError(
                "blank_slate does not accept personality or behavioral policy input",
                code="activation_mode_invalid",
            )
        try:
            resolved_identity = Identity(id=fluctlight_id, **_without_id(identity))
            resolved_personality = (
                _personality_from_payload(personality)
                if mode is InitializationMode.LLM_DEFINED and personality is not None
                else Personality.neutral()
            )
            resolved_policy = (
                BehavioralPolicy(**behavioral_policy)
                if mode is InitializationMode.LLM_DEFINED and behavioral_policy is not None
                else BehavioralPolicy()
            )
        except (TypeError, FoundationValidationError) as exc:
            raise CreationError(
                "activation foundation is invalid",
                code="activation_foundation_invalid",
            ) from exc
        if mode is InitializationMode.LLM_DEFINED and (
            personality is None or behavioral_policy is None
        ):
            raise CreationError(
                "llm_defined activation requires the reviewed complete foundation",
                code="activation_foundation_incomplete",
            )
        try:
            existing = await self._fluctlights.get(fluctlight_id)
        except FluctlightNotFoundError:
            existing = None
        if existing is not None:
            if (
                existing.initialization_mode is not mode
                or existing.identity != resolved_identity
                or existing.personality != resolved_personality
                or existing.behavioral_policy != resolved_policy
            ):
                raise CreationError(
                    "request_id was reused with different foundation data",
                    code="activation_request_conflict",
                )
            if self._schedule_initializer is not None:
                await self._schedule_initializer.ensure_for(existing)
            return existing
        try:
            created = await self._fluctlights.create(
                CreateFluctlight(
                    actor_id=actor_id,
                    id=fluctlight_id,
                    initialization_mode=mode,
                    identity=resolved_identity,
                    personality=resolved_personality,
                    behavioral_policy=resolved_policy,
                )
            )
            if self._schedule_initializer is not None:
                await self._schedule_initializer.ensure_for(created)
            return created
        except FluctlightLifecycleError as exc:
            raise CreationError(
                "activation could not be completed",
                code="activation_persistence_failed",
            ) from exc
